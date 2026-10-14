package k8s

import (
	"context"
	"fmt"
	"strings"
	"time"

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
func LoadInferenceServices(ctx context.Context, client dynamic.Interface) (map[string][]models.InferenceService, error) {
	list, err := client.Resource(inferenceServiceGVR).List(ctx, v1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list InferenceService: %w", err)
	}
	result := make(map[string][]models.InferenceService)
	for i := range list.Items {
		s := parseInferenceService(&list.Items[i])
		result[s.Namespace] = append(result[s.Namespace], s)
	}
	logging.FromContext(ctx).Debugw("loaded inference services",
		"services", len(list.Items), "namespaces", len(result))
	return result, nil
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

	age := ""
	if ts := obj.GetCreationTimestamp(); !ts.IsZero() {
		age = FormatAge(time.Since(ts.Time))
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
		Age:           age,
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
