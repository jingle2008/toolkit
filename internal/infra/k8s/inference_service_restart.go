package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"

	"github.com/jingle2008/toolkit/internal/logging"
)

// restartedAtAnnotation is the InferenceService annotation the OME
// controller propagates to the serving pods' template; bumping its value
// triggers a rolling restart (the kubectl-annotate runbook equivalent).
// The legacy ome.oracle.com/v1alpha1 controller propagates it too: on
// dp-dev-ord / dp-ppe-ord, every bump of restartedAt alone produced a new
// ReplicaSet.
const restartedAtAnnotation = "restartedAt"

/*
ResolveInferenceService returns the namespace of the InferenceService
called name, verifying it exists. With namespace set, that namespace is
checked directly; without it, every namespace is searched and the name
must be unique — an ambiguous name errors with the candidate namespaces.
*/
func ResolveInferenceService(ctx context.Context, kubeconfig, contextName, namespace, name string) (string, error) {
	client, err := NewDynamicClientFromKubeConfig(kubeconfig, contextName)
	if err != nil {
		return "", err
	}
	return resolveInferenceService(ctx, client, namespace, name)
}

/*
RestartInferenceService triggers a rolling restart of an InferenceService
by stamping its restartedAt annotation with the current UTC time. The
merge patch is atomic and needs no resourceVersion, so it never conflicts
with concurrent controller status updates.
*/
func RestartInferenceService(ctx context.Context, kubeconfig, contextName, namespace, name string) error {
	client, err := NewDynamicClientFromKubeConfig(kubeconfig, contextName)
	if err != nil {
		return err
	}
	return restartInferenceService(ctx, client, namespace, name, time.Now())
}

func resolveInferenceService(ctx context.Context, client dynamic.Interface, namespace, name string) (string, error) {
	if namespace != "" {
		_, err := onInferenceService(func(gvr schema.GroupVersionResource) error {
			_, err := client.Resource(gvr).Namespace(namespace).Get(ctx, name, v1.GetOptions{})
			return err
		})
		if err != nil {
			return "", fmt.Errorf("get InferenceService %s/%s: %w", namespace, name, err)
		}
		return namespace, nil
	}

	namespaces, err := inferenceServiceNamespaces(ctx, client, name)
	if err != nil {
		return "", err
	}
	switch len(namespaces) {
	case 0:
		return "", fmt.Errorf("InferenceService %q not found in any namespace", name)
	case 1:
		return namespaces[0], nil
	default:
		return "", fmt.Errorf("InferenceService %q exists in multiple namespaces (%s); specify a namespace",
			name, strings.Join(namespaces, ", "))
	}
}

// inferenceServiceNamespaces returns the sorted, distinct namespaces
// holding an InferenceService called name in either API. A cluster
// whose legacy API can't be read contributes no legacy matches.
func inferenceServiceNamespaces(ctx context.Context, client dynamic.Interface, name string) ([]string, error) {
	// The field selector narrows server-side; the name check below keeps
	// the result correct even where the selector is ignored.
	opts := v1.ListOptions{FieldSelector: "metadata.name=" + name}
	list, err := client.Resource(inferenceServiceGVR).List(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("list InferenceService: %w", err)
	}
	items := list.Items
	legacy, err := client.Resource(inferenceServiceV1GVR).List(ctx, opts)
	switch {
	case legacyAPIUnavailable(err): // no legacy CRD, or no access to it
	case err != nil:
		return nil, fmt.Errorf("list legacy InferenceService: %w", err)
	default:
		items = append(items, legacy.Items...)
	}
	var namespaces []string
	for i := range items {
		if items[i].GetName() == name {
			namespaces = append(namespaces, items[i].GetNamespace())
		}
	}
	slices.Sort(namespaces)
	return slices.Compact(namespaces), nil
}

func restartInferenceService(ctx context.Context, client dynamic.Interface, namespace, name string, now time.Time) error {
	// The caller only has namespace/name, so patch whichever API serves it.
	_, err := onInferenceService(func(gvr schema.GroupVersionResource) error {
		return patchRestartedAt(ctx, client, gvr, namespace, name, now)
	})
	return err
}

// patchRestartedAt stamps restartedAt on the InferenceService served by
// gvr; both OME API generations honor the annotation.
func patchRestartedAt(ctx context.Context, client dynamic.Interface, gvr schema.GroupVersionResource, namespace, name string, now time.Time) error {
	restartedAt := now.UTC().Format(time.RFC3339)
	patch, err := json.Marshal(map[string]any{
		"metadata": map[string]any{
			"annotations": map[string]string{restartedAtAnnotation: restartedAt},
		},
	})
	if err != nil {
		return fmt.Errorf("marshal restart patch: %w", err)
	}
	if _, err := client.Resource(gvr).Namespace(namespace).
		Patch(ctx, name, types.MergePatchType, patch, v1.PatchOptions{}); err != nil {
		return fmt.Errorf("restart InferenceService %s/%s: %w", namespace, name, err)
	}
	logging.FromContext(ctx).Info("restarted inference service",
		"namespace", namespace, "name", name, "apiGroup", gvr.Group, restartedAtAnnotation, restartedAt)
	return nil
}
