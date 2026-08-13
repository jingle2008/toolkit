package k8s

import (
	"context"
	"fmt"
	"strconv"

	"k8s.io/apimachinery/pkg/api/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"

	"github.com/jingle2008/toolkit/pkg/infra/logging"
	"github.com/jingle2008/toolkit/pkg/models"
)

// instanceTypeKey is the well-known node label OME's serving runtimes
// constrain on to pin a runtime to specific bare-metal shapes.
const instanceTypeKey = "node.kubernetes.io/instance-type"

// LoadServingRuntimes returns all ClusterServingRuntime CRs as a slice.
// Namespaced ServingRuntime CRs are not included: like ClusterBaseModel,
// the toolkit's surfaces are for the shared fleet-level catalog.
func LoadServingRuntimes(ctx context.Context, client dynamic.Interface) ([]models.ServingRuntime, error) {
	list, err := client.Resource(clusterServingRuntimeGVR).List(ctx, v1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list ClusterServingRuntime: %w", err)
	}
	result := make([]models.ServingRuntime, 0, len(list.Items))
	for _, item := range list.Items {
		result = append(result, parseServingRuntime(ctx, &item))
	}
	logging.FromContext(ctx).Debugw("loaded serving runtimes", "count", len(result))
	return result, nil
}

/*
parseServingRuntime extracts the operator-facing fields from one CR.

Several values appear twice in the CR: once under spec.engineConfig and
once at the top level of spec. engineConfig wins, with the top level as
fallback, and a divergence between the two is logged at debug rather
than surfaced — a "sources disagree" column would be noise until we
know it happens.
*/
func parseServingRuntime(ctx context.Context, obj *unstructured.Unstructured) models.ServingRuntime {
	spec, _, _ := unstructured.NestedMap(obj.Object, "spec")
	engine, _, _ := unstructured.NestedMap(spec, "engineConfig")

	disabled, _, _ := unstructured.NestedBool(spec, "disabled")
	accelerators, _, _ := unstructured.NestedStringSlice(spec, "acceleratorRequirements", "acceleratorClasses")
	sizeMin, _, _ := unstructured.NestedString(spec, "modelSizeRange", "min")
	sizeMax, _, _ := unstructured.NestedString(spec, "modelSizeRange", "max")

	res := runtimeResources(spec, engine)

	rt := models.ServingRuntime{
		Name:               obj.GetName(),
		AcceleratorClasses: accelerators,
		InstanceTypes:      instanceTypesFrom(ctx, spec, engine),
		Image:              res.image,
		ModelSizeMin:       sizeMin,
		ModelSizeMax:       sizeMax,
		CPULimit:           res.cpuLimit,
		MemoryLimit:        res.memLimit,
		GPULimit:           res.gpuLimit,
		CPURequest:         res.cpuRequest,
		MemoryRequest:      res.memRequest,
		GPURequest:         res.gpuRequest,
		NodeCount:          res.nodes,
		Disabled:           disabled,
		AutoSelect:         anyAutoSelect(spec),
	}
	logRunnerDivergence(ctx, obj.GetName(), spec, engine, res.image)
	return rt
}

// resourceSet is the per-runtime resource picture: the image that
// serves it, the totals across every pod it occupies, and how many
// pods that is.
type resourceSet struct {
	image                              string
	cpuLimit, memLimit, gpuLimit       string
	cpuRequest, memRequest, gpuRequest string
	nodes                              int
}

/*
runtimeResources resolves the image and resource totals for a runtime.

Two shapes exist. The ordinary one has a single container —
engineConfig.runner, or the ome-container in spec.containers — and its
resources are the runtime's resources.

A multi-node runtime instead has engineConfig.leader and
engineConfig.worker, each with their own container, and worker.size
says how many workers there are. Its resources are the *sum* across
leader + worker×size, because the operator question is what it costs
to serve the model, not what one pod of it costs. Summing goes through
resource.Quantity so units are handled and the result is canonical
(512Gi + 512Gi renders as 1Ti, not 1024Gi).
*/
func runtimeResources(spec, engine map[string]any) resourceSet {
	if leader, worker, ok := leaderWorker(engine); ok {
		return sumLeaderWorker(leader, worker)
	}
	runner := runnerContainer(spec, engine)
	image, _, _ := unstructured.NestedString(runner, "image")
	return resourceSet{
		image:      image,
		cpuLimit:   nestedQuantity(runner, "resources", "limits", "cpu"),
		memLimit:   nestedQuantity(runner, "resources", "limits", "memory"),
		gpuLimit:   nestedQuantity(runner, "resources", "limits", "nvidia.com/gpu"),
		cpuRequest: nestedQuantity(runner, "resources", "requests", "cpu"),
		memRequest: nestedQuantity(runner, "resources", "requests", "memory"),
		gpuRequest: nestedQuantity(runner, "resources", "requests", "nvidia.com/gpu"),
		nodes:      1,
	}
}

// leaderWorker returns the multi-node role containers, if this is a
// multi-node runtime. A runtime with engineConfig.runner is not, even
// if it also carries leader/worker.
func leaderWorker(engine map[string]any) (leader, worker map[string]any, ok bool) {
	if _, found, _ := unstructured.NestedMap(engine, "runner"); found {
		return nil, nil, false
	}
	leader, hasLeader, _ := unstructured.NestedMap(engine, "leader")
	worker, hasWorker, _ := unstructured.NestedMap(engine, "worker")
	return leader, worker, hasLeader || hasWorker
}

func sumLeaderWorker(leader, worker map[string]any) resourceSet {
	leaderRunner, _, _ := unstructured.NestedMap(leader, "runner")
	workerRunner, _, _ := unstructured.NestedMap(worker, "runner")

	// A worker stanza with no explicit size still describes one worker;
	// treating a missing size as zero would silently halve the total.
	// nestedCount rather than NestedInt64 because the number's Go type
	// depends on the decoder — the API server's yields int64, a
	// YAML-parsed fixture yields float64, and NestedInt64 rejects the
	// latter by returning not-found.
	workers := 1
	if n, found := nestedCount(worker, "size"); found {
		workers = n
	}
	if workerRunner == nil {
		workers = 0
	}

	leaders := 0
	if leaderRunner != nil {
		leaders = 1
	}

	// Leader and worker run the same image on every multi-node runtime
	// across both indexed clusters, so the leader's is taken as the
	// runtime's. The worker is only consulted when the leader has no
	// image at all. There is deliberately no divergence check here: the
	// equivalent one for engineConfig vs top-level spec earns its place
	// because four runtimes actually disagree, whereas leader/worker
	// never have, so a check would be untested speculation.
	out := resourceSet{nodes: leaders + workers}
	out.image, _, _ = unstructured.NestedString(leaderRunner, "image")
	if out.image == "" {
		out.image, _, _ = unstructured.NestedString(workerRunner, "image")
	}

	for _, field := range []struct {
		kind string
		key  string
		dst  *string
	}{
		{"limits", "cpu", &out.cpuLimit},
		{"limits", "memory", &out.memLimit},
		{"limits", "nvidia.com/gpu", &out.gpuLimit},
		{"requests", "cpu", &out.cpuRequest},
		{"requests", "memory", &out.memRequest},
		{"requests", "nvidia.com/gpu", &out.gpuRequest},
	} {
		*field.dst = sumQuantities(
			nestedQuantity(leaderRunner, "resources", field.kind, field.key), leaders,
			nestedQuantity(workerRunner, "resources", field.kind, field.key), workers,
		)
	}
	return out
}

/*
sumQuantities returns leader*leaderN + worker*workerN as a canonical
quantity string, or "" when neither side declares the resource.

An unparseable quantity is skipped rather than zeroed: reporting a
smaller total than reality would understate what the runtime costs,
which is the more damaging direction to be wrong in.
*/
func sumQuantities(leaderVal string, leaderN int, workerVal string, workerN int) string {
	total := resource.Quantity{}
	any := false
	for _, side := range []struct {
		val string
		n   int
	}{{leaderVal, leaderN}, {workerVal, workerN}} {
		if side.val == "" || side.n <= 0 {
			continue
		}
		q, err := resource.ParseQuantity(side.val)
		if err != nil {
			continue
		}
		for i := 0; i < side.n; i++ {
			total.Add(q)
		}
		any = true
	}
	if !any {
		return ""
	}
	return total.String()
}

/*
runnerContainer returns the container spec to read image and limits from.

spec.engineConfig.runner is a single container map and wins when
present. The fallback walks spec.containers for the one named
"ome-container" — the model server itself, as opposed to any sidecar.
*/
func runnerContainer(spec, engine map[string]any) map[string]any {
	if runner, found, _ := unstructured.NestedMap(engine, "runner"); found {
		return runner
	}
	containers, _, _ := unstructured.NestedSlice(spec, "containers")
	for _, c := range containers {
		container, ok := c.(map[string]any)
		if !ok {
			continue
		}
		if name, _, _ := unstructured.NestedString(container, "name"); name == omeContainerName {
			return container
		}
	}
	return nil
}

// omeContainerName is the model-server container in a serving runtime's
// pod spec; anything else in spec.containers is a sidecar.
const omeContainerName = "ome-container"

/*
instanceTypesFrom pulls the instance-type values out of the required
node affinity, preferring engineConfig's copy.

It reads only requiredDuringScheduling terms with the
node.kubernetes.io/instance-type key and the In operator: those are the
hard constraints that decide where a runtime can actually land.
Preferred affinity and NotIn terms are deliberately ignored — showing a
soft preference as though it were a restriction would mislead.
*/
func instanceTypesFrom(ctx context.Context, spec, engine map[string]any) []string {
	types := instanceTypesFromAffinity(engine)
	if len(types) == 0 {
		types = instanceTypesFromAffinity(spec)
	} else if fallback := instanceTypesFromAffinity(spec); len(fallback) > 0 && !equalStrings(types, fallback) {
		logging.FromContext(ctx).Debugw("serving runtime affinity differs between engineConfig and spec",
			"engineConfig", types, "spec", fallback)
	}
	return types
}

func instanceTypesFromAffinity(root map[string]any) []string {
	terms, _, _ := unstructured.NestedSlice(root,
		"affinity", "nodeAffinity", "requiredDuringSchedulingIgnoredDuringExecution", "nodeSelectorTerms")
	var out []string
	for _, t := range terms {
		term, ok := t.(map[string]any)
		if !ok {
			continue
		}
		exprs, _, _ := unstructured.NestedSlice(term, "matchExpressions")
		for _, e := range exprs {
			expr, ok := e.(map[string]any)
			if !ok {
				continue
			}
			key, _, _ := unstructured.NestedString(expr, "key")
			op, _, _ := unstructured.NestedString(expr, "operator")
			if key != instanceTypeKey || op != "In" {
				continue
			}
			values, _, _ := unstructured.NestedStringSlice(expr, "values")
			out = append(out, values...)
		}
	}
	return out
}

// anyAutoSelect reports whether any supported model format opts into
// automatic selection. The flag is per-format in the CR, but the
// question it answers is per-runtime, so one is enough.
func anyAutoSelect(spec map[string]any) bool {
	formats, _, _ := unstructured.NestedSlice(spec, "supportedModelFormats")
	for _, f := range formats {
		format, ok := f.(map[string]any)
		if !ok {
			continue
		}
		if auto, _, _ := unstructured.NestedBool(format, "autoSelect"); auto {
			return true
		}
	}
	return false
}

// logRunnerDivergence reports an engineConfig/top-level image mismatch.
// Silent when either side is absent — that's the normal single-source
// shape, not a conflict.
func logRunnerDivergence(ctx context.Context, name string, spec, engine map[string]any, chosen string) {
	engineImage, _, _ := unstructured.NestedString(engine, "runner", "image")
	if engineImage == "" {
		return
	}
	containers, _, _ := unstructured.NestedSlice(spec, "containers")
	for _, c := range containers {
		container, ok := c.(map[string]any)
		if !ok {
			continue
		}
		if n, _, _ := unstructured.NestedString(container, "name"); n != omeContainerName {
			continue
		}
		if img, _, _ := unstructured.NestedString(container, "image"); img != "" && img != engineImage {
			logging.FromContext(ctx).Debugw("serving runtime image differs between engineConfig and spec",
				"runtime", name, "engineConfig", engineImage, "spec", img, "using", chosen)
		}
	}
}

/*
nestedQuantity reads a Kubernetes resource quantity as a string.

Quantities are not consistently typed on the wire: the same field is a
JSON string when it carries a unit ("80Gi", "500m") and a bare number
when it doesn't (cpu: 10, nvidia.com/gpu: 1). Across this fleet every
`cpu` is a number and `nvidia.com/gpu` is a number about seven times
out of eight, so reading these with NestedString silently yields ""
for most rows — a blank cell that reads as "unset" rather than as
"we failed to parse it".

Integers are formatted without a decimal point so "10" doesn't become
"10.000000"; the unstructured converter yields int64 for whole numbers
but float64 is handled too, since a fractional cpu (0.5) is legal.
*/
func nestedQuantity(root map[string]any, fields ...string) string {
	v, found, err := unstructured.NestedFieldNoCopy(root, fields...)
	if !found || err != nil {
		return ""
	}
	switch q := v.(type) {
	case string:
		return q
	case int64:
		return strconv.FormatInt(q, 10)
	case float64:
		return strconv.FormatFloat(q, 'g', -1, 64)
	default:
		return ""
	}
}

// nestedCount reads an integer field that may have decoded as either
// int64 or float64 depending on the decoder in play.
func nestedCount(root map[string]any, fields ...string) (int, bool) {
	v, found, err := unstructured.NestedFieldNoCopy(root, fields...)
	if !found || err != nil {
		return 0, false
	}
	switch n := v.(type) {
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	default:
		return 0, false
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
