package k8s

import (
	"context"
	"fmt"
	"strconv"

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

	runner := runnerContainer(spec, engine)
	image, _, _ := unstructured.NestedString(runner, "image")
	cpu := nestedQuantity(runner, "resources", "limits", "cpu")
	memory := nestedQuantity(runner, "resources", "limits", "memory")
	gpu := nestedQuantity(runner, "resources", "limits", "nvidia.com/gpu")
	cpuReq := nestedQuantity(runner, "resources", "requests", "cpu")
	memoryReq := nestedQuantity(runner, "resources", "requests", "memory")
	gpuReq := nestedQuantity(runner, "resources", "requests", "nvidia.com/gpu")

	rt := models.ServingRuntime{
		Name:               obj.GetName(),
		AcceleratorClasses: accelerators,
		InstanceTypes:      instanceTypesFrom(ctx, spec, engine),
		Image:              image,
		ModelSizeMin:       sizeMin,
		ModelSizeMax:       sizeMax,
		CPULimit:           cpu,
		MemoryLimit:        memory,
		GPULimit:           gpu,
		CPURequest:         cpuReq,
		MemoryRequest:      memoryReq,
		GPURequest:         gpuReq,
		Disabled:           disabled,
		AutoSelect:         anyAutoSelect(spec),
	}
	logRunnerDivergence(ctx, obj.GetName(), spec, engine, image)
	return rt
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
