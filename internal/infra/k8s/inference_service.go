package k8s

import (
	"context"
	"fmt"
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"

	"github.com/jingle2008/toolkit/pkg/infra/logging"
	"github.com/jingle2008/toolkit/pkg/models"
)

const (
	// dacAnnotation names the dedicated AI cluster an InferenceService
	// runs on. Its value equals the namespace.
	dacAnnotation = "ome.io/dedicated-ai-cluster"
	// dacNamespacePrefix is the leading run of a DAC OCID suffix, which
	// is what a dedicated endpoint's namespace is named after.
	dacNamespacePrefix = "amaaaaaa"
	// namespacedBaseModelKind is the model ref kind for a tenant-imported
	// (namespaced) model; the shared catalog uses ClusterBaseModel.
	namespacedBaseModelKind = "BaseModel"
)

// LoadInferenceServices lists InferenceService CRs across all namespaces
// and groups them by namespace — the DAC ID for a dedicated endpoint
// (== DedicatedAICluster.Name), the model name for an on-demand one.
// Both the ome.io/v1beta1 API and the legacy ome.oracle.com/v1alpha1 API
// (endpoints on legacy DACs) are listed; a cluster without the legacy
// CRD simply contributes no legacy rows.
func LoadInferenceServices(ctx context.Context, client dynamic.Interface) (map[string][]models.InferenceService, error) {
	list, err := client.Resource(inferenceServiceGVR).List(ctx, v1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list InferenceService: %w", err)
	}
	legacy, err := listLegacyInferenceServices(ctx, client)
	if err != nil {
		return nil, err
	}
	result := make(map[string][]models.InferenceService)
	for i := range list.Items {
		s := parseInferenceService(&list.Items[i])
		result[s.Namespace] = append(result[s.Namespace], s)
	}
	for i := range legacy {
		s := parseLegacyInferenceService(&legacy[i])
		result[s.Namespace] = append(result[s.Namespace], s)
	}
	logging.FromContext(ctx).Debugw("loaded inference services",
		"services", len(list.Items), "legacy", len(legacy), "namespaces", len(result))
	return result, nil
}

// listLegacyInferenceServices lists ome.oracle.com/v1alpha1
// InferenceServices, returning none when the CRD isn't installed.
func listLegacyInferenceServices(ctx context.Context, client dynamic.Interface) ([]unstructured.Unstructured, error) {
	list, err := client.Resource(legacyInferenceServiceGVR).List(ctx, v1.ListOptions{})
	if apierrors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list legacy InferenceService: %w", err)
	}
	return list.Items, nil
}

/*
parseInferenceService extracts the operator-facing fields from one CR.

An endpoint is dedicated when it carries the DAC annotation or, failing
that, lives in a namespace named after a DAC OCID; anything else is a
shared on-demand endpoint. A missing tenancy-id label leaves TenantID
blank rather than bucketing under UNKNOWN_TENANCY: the map is keyed by
namespace, and on-demand endpoints legitimately have no tenant.
*/
func parseInferenceService(obj *unstructured.Unstructured) models.InferenceService {
	spec, _, _ := unstructured.NestedMap(obj.Object, "spec")
	modelName, _, _ := unstructured.NestedString(spec, "model", "name")
	modelKind, _, _ := unstructured.NestedString(spec, "model", "kind")
	runtime, _, _ := unstructured.NestedString(spec, "runtime", "name")
	minReplicas, _ := nestedCount(spec, "engine", "minReplicas")
	maxReplicas, _ := nestedCount(spec, "engine", "maxReplicas")

	ns := obj.GetNamespace()
	svcType := models.InferenceServiceOnDemand
	if _, ok := obj.GetAnnotations()[dacAnnotation]; ok || strings.HasPrefix(ns, dacNamespacePrefix) {
		svcType = models.InferenceServiceDedicated
	}

	return models.InferenceService{
		Name:          obj.GetName(),
		Namespace:     ns,
		Type:          svcType,
		TenantID:      obj.GetLabels()["tenancy-id"],
		Model:         modelName,
		ImportedModel: modelKind == namespacedBaseModelKind,
		Runtime:       runtime,
		MinReplicas:   minReplicas,
		MaxReplicas:   maxReplicas,
		Ready:         readyStatus(obj),
		Age:           ageOf(obj),
	}
}

/*
parseLegacyInferenceService extracts the same fields from a legacy
ome.oracle.com/v1alpha1 CR, whose serving config sits under
spec.predictor. The model is the first spec.predictor.model.modelRefs
entry (a legacy Model CR, not a BaseModel), falling back to
spec.predictor.model.name; an endpoint is dedicated when it references a
DAC (dedicatedAIClusterRef) or lives in a DAC-named namespace.
*/
func parseLegacyInferenceService(obj *unstructured.Unstructured) models.InferenceService {
	predictor, _, _ := unstructured.NestedMap(obj.Object, "spec", "predictor")
	modelName, _, _ := unstructured.NestedString(predictor, "model", "name")
	if refs, _, _ := unstructured.NestedSlice(predictor, "model", "modelRefs"); len(refs) > 0 {
		if ref, ok := refs[0].(map[string]any); ok {
			if name, _, _ := unstructured.NestedString(ref, "name"); name != "" {
				modelName = name
			}
		}
	}
	runtime, _, _ := unstructured.NestedString(predictor, "model", "runtime")
	dacRef, _, _ := unstructured.NestedString(predictor, "model", "dedicatedAIClusterRef", "name")
	minReplicas, _ := nestedCount(predictor, "minReplicas")
	maxReplicas, _ := nestedCount(predictor, "maxReplicas")

	ns := obj.GetNamespace()
	svcType := models.InferenceServiceOnDemand
	if dacRef != "" || strings.HasPrefix(ns, dacNamespacePrefix) {
		svcType = models.InferenceServiceDedicated
	}

	return models.InferenceService{
		Name:        obj.GetName(),
		Namespace:   ns,
		Type:        svcType,
		TenantID:    obj.GetLabels()["tenancy-id"],
		Model:       modelName,
		Runtime:     runtime,
		MinReplicas: minReplicas,
		MaxReplicas: maxReplicas,
		Ready:       readyStatus(obj),
		Age:         ageOf(obj),
		Legacy:      true,
	}
}

// ageOf formats the time since obj's creation, or "" when unset.
func ageOf(obj *unstructured.Unstructured) string {
	if ts := obj.GetCreationTimestamp(); !ts.IsZero() {
		return FormatAge(time.Since(ts.Time))
	}
	return ""
}

// readyStatus returns the status of the Ready condition, or "Unknown"
// when the controller hasn't reported one yet.
func readyStatus(obj *unstructured.Unstructured) string {
	conds, _, _ := unstructured.NestedSlice(obj.Object, "status", "conditions")
	for _, c := range conds {
		cond, ok := c.(map[string]any)
		if !ok {
			continue
		}
		if t, _, _ := unstructured.NestedString(cond, "type"); t != "Ready" {
			continue
		}
		if s, _, _ := unstructured.NestedString(cond, "status"); s != "" {
			return s
		}
	}
	return "Unknown"
}
