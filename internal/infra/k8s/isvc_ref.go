// InferenceService references: the serving pods on a node, the
// InferenceServices that own them, and which to restart before eviction.

package k8s

import (
	"context"
	"fmt"
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// servingAPI ties a serving pod label to the InferenceService API that owns
// the pod. The two OME generations keep minReplicas at different spec paths.
type servingAPI struct {
	label           string
	gvr             schema.GroupVersionResource
	minReplicasPath []string
}

var servingAPIs = []servingAPI{
	{label: servingLabelV2, gvr: inferenceServiceGVR, minReplicasPath: []string{"engine", "minReplicas"}},
	{label: servingLabelV1, gvr: inferenceServiceV1GVR, minReplicasPath: []string{"predictor", "minReplicas"}},
}

// isvcRef identifies one InferenceService with pods on the node.
type isvcRef struct {
	api       *servingAPI
	namespace string
	name      string
}

func (r isvcRef) String() string {
	return r.api.gvr.Group + ":" + r.namespace + "/" + r.name
}

// parseISVCRef inverts isvcRef.String.
func parseISVCRef(s string) (isvcRef, bool) {
	group, rest, ok := strings.Cut(s, ":")
	if !ok {
		return isvcRef{}, false
	}
	namespace, name, ok := strings.Cut(rest, "/")
	if !ok {
		return isvcRef{}, false
	}
	for i := range servingAPIs {
		if servingAPIs[i].gvr.Group == group {
			return isvcRef{api: &servingAPIs[i], namespace: namespace, name: name}, true
		}
	}
	return isvcRef{}, false
}

// servingRefs returns the distinct InferenceServices with live pods in
// pods, sorted for deterministic processing.
func servingRefs(pods []corev1.Pod) []isvcRef {
	seen := map[string]isvcRef{}
	for i := range pods {
		pod := &pods[i]
		if !podActive(pod) || pod.DeletionTimestamp != nil {
			continue
		}
		for j := range servingAPIs {
			api := &servingAPIs[j]
			if name := pod.Labels[api.label]; name != "" {
				ref := isvcRef{api: api, namespace: pod.Namespace, name: name}
				seen[ref.String()] = ref
				break
			}
		}
	}
	refs := make([]isvcRef, 0, len(seen))
	for _, ref := range seen {
		refs = append(refs, ref)
	}
	slices.SortFunc(refs, func(a, b isvcRef) int { return strings.Compare(a.String(), b.String()) })
	return refs
}

/*
planRestarts picks the InferenceServices on the node to restart: those
whose minReplicas is at most 1 (unset counts as 1). Multi-replica services
are left to eviction, their other replicas serving meanwhile; so is a
service whose CR is gone.
*/
func planRestarts(ctx context.Context, client dynamic.Interface, pods []corev1.Pod) ([]isvcRef, error) {
	var plan []isvcRef
	for _, ref := range servingRefs(pods) {
		obj, err := client.Resource(ref.api.gvr).Namespace(ref.namespace).Get(ctx, ref.name, v1.GetOptions{})
		if apierrors.IsNotFound(err) {
			note(ctx, true, "drain: InferenceService not found, leaving its pods to eviction", "service", ref.String())
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("get InferenceService %s: %w", ref, err)
		}
		spec, _ := obj.Object["spec"].(map[string]any)
		minReplicas, ok := nestedCount(spec, ref.api.minReplicasPath...)
		if !ok {
			minReplicas = 1
		}
		if minReplicas > 1 {
			note(ctx, false, "drain: leaving multi-replica InferenceService to eviction", "service", ref.String(), "minReplicas", minReplicas)
			continue
		}
		plan = append(plan, ref)
	}
	return plan, nil
}

// remainingServices lists the restarted InferenceServices that still have
// live pods on the node.
func remainingServices(st DrainState, pods []corev1.Pod) []string {
	var remaining []string
	for _, s := range st.Restarted {
		ref, ok := parseISVCRef(s)
		if !ok {
			continue
		}
		for i := range pods {
			pod := &pods[i]
			if podActive(pod) && pod.Namespace == ref.namespace && pod.Labels[ref.api.label] == ref.name {
				remaining = append(remaining, s)
				break
			}
		}
	}
	return remaining
}
