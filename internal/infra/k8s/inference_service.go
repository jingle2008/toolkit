package k8s

import (
	"context"
	"fmt"
	"sync"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"

	"github.com/jingle2008/toolkit/internal/logging"
	"github.com/jingle2008/toolkit/pkg/models"
)

const (
	// dacAnnotation names the dedicated AI cluster an InferenceService
	// runs on. Its value equals the namespace.
	dacAnnotation = "ome.io/dedicated-ai-cluster"
	// namespacedBaseModelKind is the model ref kind for a tenant-imported
	// (namespaced) model; the shared catalog uses ClusterBaseModel.
	namespacedBaseModelKind = "BaseModel"
)

// LoadInferenceServices lists InferenceService CRs across all namespaces
// and groups them by namespace — the DAC ID for a dedicated endpoint
// (== DedicatedAICluster.Name), the model name for an on-demand one.
// Both the ome.io/v1beta1 API and the legacy ome.oracle.com/v1alpha1 API
// (endpoints on legacy DACs) are listed; a cluster without the legacy
// CRD, or a user not allowed to read it, simply contributes no legacy
// rows.
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
	logging.FromContext(ctx).Debug("loaded inference services",
		"services", len(list.Items), "legacy", len(legacy), "namespaces", len(result))
	return result, nil
}

// listLegacyInferenceServices lists ome.oracle.com/v1alpha1
// InferenceServices, returning none when the API is unavailable.
func listLegacyInferenceServices(ctx context.Context, client dynamic.Interface) ([]unstructured.Unstructured, error) {
	list, err := client.Resource(inferenceServiceV1GVR).List(ctx, v1.ListOptions{})
	if legacyAPIUnavailable(err) {
		// Warn once: every reload hits the same, expected Forbidden.
		if apierrors.IsForbidden(err) {
			log := logging.FromContext(ctx).Debug
			legacyForbiddenOnce.Do(func() { log = logging.FromContext(ctx).Warn })
			log("not allowed to list legacy InferenceServices; skipping them", "error", err)
		}
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list legacy InferenceService: %w", err)
	}
	return list.Items, nil
}

// legacyForbiddenOnce makes the Forbidden warning on the legacy list a
// once-per-process message.
var legacyForbiddenOnce sync.Once

// optionalCount is nestedCount as a pointer: nil when the field is unset,
// so an endpoint without one shows no replica count rather than 0.
func optionalCount(root map[string]any, fields ...string) *int {
	if n, ok := nestedCount(root, fields...); ok {
		return &n
	}
	return nil
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
	minReplicas := optionalCount(spec, "engine", "minReplicas")
	maxReplicas := optionalCount(spec, "engine", "maxReplicas")

	ns := obj.GetNamespace()
	svcType := models.InferenceServiceOnDemand
	if _, ok := obj.GetAnnotations()[dacAnnotation]; ok || models.IsDACName(ns) {
		svcType = models.InferenceServiceDedicated
	}

	return models.InferenceService{
		Name:      obj.GetName(),
		Namespace: ns,
		Type:      svcType,
		TenantID:  obj.GetLabels()[labelTenancyID],
		Model:     modelName,
		// Imported whether referenced as a namespaced BaseModel or as a
		// tenant-labelled ClusterBaseModel: the name (an OCID) says so.
		ImportedModel: modelKind == namespacedBaseModelKind || models.IsImportedModelName(modelName),
		Runtime:       runtime,
		MinReplicas:   minReplicas,
		MaxReplicas:   maxReplicas,
		Ready:         readyStatus(obj),
		Age:           models.AgeOf(obj.GetCreationTimestamp().Time),
		CreatedAt:     obj.GetCreationTimestamp().Time,
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
	minReplicas := optionalCount(predictor, "minReplicas")
	maxReplicas := optionalCount(predictor, "maxReplicas")

	ns := obj.GetNamespace()
	svcType := models.InferenceServiceOnDemand
	if dacRef != "" || models.IsDACName(ns) {
		svcType = models.InferenceServiceDedicated
	}

	return models.InferenceService{
		Name:        obj.GetName(),
		Namespace:   ns,
		Type:        svcType,
		TenantID:    obj.GetLabels()[labelTenancyID],
		Model:       modelName,
		Runtime:     runtime,
		MinReplicas: minReplicas,
		MaxReplicas: maxReplicas,
		Ready:       readyStatus(obj),
		Age:         models.AgeOf(obj.GetCreationTimestamp().Time),
		CreatedAt:   obj.GetCreationTimestamp().Time,
		Legacy:      true,
	}
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
