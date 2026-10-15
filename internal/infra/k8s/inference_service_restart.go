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

	"github.com/jingle2008/toolkit/pkg/infra/logging"
)

// restartedAtAnnotation is the InferenceService annotation the OME
// controller propagates to the serving pods' template; bumping its value
// triggers a rolling restart (the kubectl-annotate runbook equivalent).
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
		if _, err := client.Resource(inferenceServiceGVR).Namespace(namespace).Get(ctx, name, v1.GetOptions{}); err != nil {
			return "", fmt.Errorf("get InferenceService %s/%s: %w", namespace, name, err)
		}
		return namespace, nil
	}

	// The field selector narrows server-side; the name check below keeps
	// the result correct even where the selector is ignored.
	list, err := client.Resource(inferenceServiceGVR).List(ctx, v1.ListOptions{
		FieldSelector: "metadata.name=" + name,
	})
	if err != nil {
		return "", fmt.Errorf("list InferenceService: %w", err)
	}
	var namespaces []string
	for i := range list.Items {
		if list.Items[i].GetName() == name {
			namespaces = append(namespaces, list.Items[i].GetNamespace())
		}
	}
	switch len(namespaces) {
	case 0:
		return "", fmt.Errorf("InferenceService %q not found in any namespace", name)
	case 1:
		return namespaces[0], nil
	default:
		slices.Sort(namespaces)
		return "", fmt.Errorf("InferenceService %q exists in multiple namespaces (%s); specify a namespace",
			name, strings.Join(namespaces, ", "))
	}
}

func restartInferenceService(ctx context.Context, client dynamic.Interface, namespace, name string, now time.Time) error {
	return patchRestartedAt(ctx, client, inferenceServiceGVR, namespace, name, now)
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
	logging.FromContext(ctx).Infow("restarted inference service",
		"namespace", namespace, "name", name, restartedAtAnnotation, restartedAt)
	return nil
}
