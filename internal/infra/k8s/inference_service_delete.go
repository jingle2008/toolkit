package k8s

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"

	"github.com/jingle2008/toolkit/internal/logging"
	"github.com/jingle2008/toolkit/pkg/models"
)

// inferenceServiceFinalizer is the finalizer both OME controllers put on
// every InferenceService (all 173 on dp-prod-ord carry it, and only it).
const inferenceServiceFinalizer = "inferenceservice.finalizers"

// orphanFinalizeAfter is how long a deletion must have been pending before
// DeleteOrphanInferenceService removes the finalizer: the controller gets
// its chance to clean up first.
const orphanFinalizeAfter = 10 * time.Minute

// dacGVRs are every API a DAC can be served by, including one the toolkit
// doesn't list: the orphan guard refuses when the DAC exists in any.
var dacGVRs = []schema.GroupVersionResource{
	dacV2GVR,
	dacV1GVR,
	{Group: "gpu-alfred.generative-ai.oracle.com", Version: "v1alpha1", Resource: "dedicatedaiclusters"},
}

// ErrNotOrphan is returned when DeleteOrphanInferenceService refuses an
// endpoint that isn't an orphan.
var ErrNotOrphan = errors.New("not an orphaned InferenceService")

// ErrDeleteDeferred matches the refusals of an orphan whose deletion is
// already pending but not yet safe to finalize: too recent, held by an
// unknown finalizer, or held by none. Like ErrNotOrphan, nothing failed;
// the delete declined.
var ErrDeleteDeferred = errors.New("orphan delete deferred")

// deferredError is a refusal that matches ErrDeleteDeferred while
// keeping its own message, so callers that print it see it unchanged.
type deferredError struct{ msg string }

func (e deferredError) Error() string      { return e.msg }
func (deferredError) Is(target error) bool { return target == ErrDeleteDeferred }

func deferred(format string, args ...any) error {
	return deferredError{msg: fmt.Sprintf(format, args...)}
}

// OrphanDeleteOutcome says what DeleteOrphanInferenceService did.
type OrphanDeleteOutcome string

const (
	// OrphanDeleted means a delete was issued; the controller finalizes it.
	OrphanDeleted OrphanDeleteOutcome = "deleted"
	// OrphanFinalized means a deletion stuck on its finalizer was
	// completed by removing the finalizer.
	OrphanFinalized OrphanDeleteOutcome = "finalized"
)

// Action names the outcome for prompts and the audit log.
func (o OrphanDeleteOutcome) Action() string {
	if o == OrphanFinalized {
		return "force-delete"
	}
	return "delete"
}

// Summary is the one-line result for target, as every surface reports it.
func (o OrphanDeleteOutcome) Summary(target string) string {
	if o == OrphanFinalized {
		return "force-deleted " + target + ": finalizer removed"
	}
	return "deleted " + target + "; if it stays deleting, run again after 10m to force-delete"
}

/*
PlanOrphanInferenceServiceDelete reports, without changing anything, what
DeleteOrphanInferenceService would do to namespace/name: OrphanDeleted or
OrphanFinalized, or an error saying why it would refuse. Callers plan
first so a dry run or a confirmation prompt names the real action.
*/
func PlanOrphanInferenceServiceDelete(ctx context.Context, kubeconfig, contextName, namespace, name string) (OrphanDeleteOutcome, error) {
	client, err := NewDynamicClientFromKubeConfig(kubeconfig, contextName)
	if err != nil {
		return "", err
	}
	p, err := planOrphanDelete(ctx, client, namespace, name, time.Now())
	return p.outcome, err
}

/*
DeleteOrphanInferenceService removes a dedicated InferenceService whose
dedicated AI cluster no longer exists; anything else is refused with
ErrNotOrphan, since an endpoint's lifecycle otherwise belongs to the
control plane (delete its DAC instead). An endpoint not yet being
deleted is deleted. One whose deletion has been pending for over
orphanFinalizeAfter — the OME controller can't finalize an endpoint
whose DAC or model is gone, so its namespace stays Terminating — is
completed by removing the OME finalizer. The two steps are separate
calls on purpose: the controller gets its chance to clean up first.
*/
func DeleteOrphanInferenceService(ctx context.Context, kubeconfig, contextName, namespace, name string) (OrphanDeleteOutcome, error) {
	client, err := NewDynamicClientFromKubeConfig(kubeconfig, contextName)
	if err != nil {
		return "", err
	}
	return deleteOrphanInferenceService(ctx, client, namespace, name, time.Now())
}

// orphanDeletePlan is what deleting an orphan would do, and to which
// object.
type orphanDeletePlan struct {
	obj     *unstructured.Unstructured
	gvr     schema.GroupVersionResource
	outcome OrphanDeleteOutcome
}

func planOrphanDelete(ctx context.Context, client dynamic.Interface, namespace, name string, now time.Time) (orphanDeletePlan, error) {
	obj, gvr, err := getInferenceService(ctx, client, namespace, name)
	if err != nil {
		return orphanDeletePlan{}, err
	}
	if err := checkOrphan(ctx, client, obj, gvr); err != nil {
		return orphanDeletePlan{}, err
	}
	p := orphanDeletePlan{obj: obj, gvr: gvr, outcome: OrphanDeleted}
	deleting := obj.GetDeletionTimestamp()
	if deleting == nil {
		return p, nil
	}
	target := namespace + "/" + name
	if pending := now.Sub(deleting.Time); pending < orphanFinalizeAfter {
		return orphanDeletePlan{}, deferred("InferenceService %s: deletion requested %s ago; the controller may still be finalizing it — retry after %s",
			target, pending.Round(time.Second), orphanFinalizeAfter)
	}
	finalizers := obj.GetFinalizers()
	if unknown := slices.DeleteFunc(slices.Clone(finalizers), func(f string) bool { return f == inferenceServiceFinalizer }); len(unknown) > 0 {
		return orphanDeletePlan{}, deferred("InferenceService %s: unknown finalizers %s; not removing them", target, strings.Join(unknown, ", "))
	}
	if len(finalizers) == 0 {
		return orphanDeletePlan{}, deferred("InferenceService %s: deletion is pending but no finalizer holds it", target)
	}
	p.outcome = OrphanFinalized
	return p, nil
}

func deleteOrphanInferenceService(ctx context.Context, client dynamic.Interface, namespace, name string, now time.Time) (OrphanDeleteOutcome, error) {
	p, err := planOrphanDelete(ctx, client, namespace, name, now)
	if err != nil {
		return "", err
	}
	target := namespace + "/" + name
	logger := logging.FromContext(ctx)
	switch p.outcome {
	case OrphanFinalized:
		if err := removeFinalizers(ctx, client, p.gvr, p.obj); err != nil {
			return "", fmt.Errorf("finalize InferenceService %s: %w", target, err)
		}
		logger.Info("finalized orphaned inference service", "namespace", namespace, "name", name,
			"apiGroup", p.gvr.Group, "deletionRequested", p.obj.GetDeletionTimestamp().UTC().Format(time.RFC3339))
	default:
		if err := client.Resource(p.gvr).Namespace(namespace).Delete(ctx, name, v1.DeleteOptions{}); err != nil {
			return "", fmt.Errorf("delete InferenceService %s: %w", target, err)
		}
		logger.Info("deleted orphaned inference service", "namespace", namespace, "name", name, "apiGroup", p.gvr.Group)
	}
	return p.outcome, nil
}

// getInferenceService fetches namespace/name from the ome.io API, then the
// legacy one, returning the API it was found in.
func getInferenceService(ctx context.Context, client dynamic.Interface, namespace, name string) (*unstructured.Unstructured, schema.GroupVersionResource, error) {
	obj, err := client.Resource(inferenceServiceGVR).Namespace(namespace).Get(ctx, name, v1.GetOptions{})
	if err == nil {
		return obj, inferenceServiceGVR, nil
	}
	if apierrors.IsNotFound(err) {
		legacy, legacyErr := client.Resource(inferenceServiceV1GVR).Namespace(namespace).Get(ctx, name, v1.GetOptions{})
		if legacyErr == nil {
			return legacy, inferenceServiceV1GVR, nil
		}
		if !apierrors.IsNotFound(legacyErr) {
			err = legacyErr
		}
	}
	return nil, schema.GroupVersionResource{}, fmt.Errorf("get InferenceService %s/%s: %w", namespace, name, err)
}

// checkOrphan returns nil when obj is a dedicated endpoint none of whose
// DAC names exists in any DAC API. Lookup errors other than NotFound fail
// closed.
func checkOrphan(ctx context.Context, client dynamic.Interface, obj *unstructured.Unstructured, gvr schema.GroupVersionResource) error {
	dacs := endpointDACNames(obj, gvr)
	if len(dacs) == 0 {
		return fmt.Errorf("%w: %s/%s is not a dedicated endpoint (shared endpoints belong to the control plane)",
			ErrNotOrphan, obj.GetNamespace(), obj.GetName())
	}
	for _, dac := range dacs {
		for _, dg := range dacGVRs {
			_, err := client.Resource(dg).Get(ctx, dac, v1.GetOptions{})
			switch {
			case err == nil:
				return fmt.Errorf("%w: its DAC %s exists (%s); delete the DAC instead", ErrNotOrphan, dac, dg.Group)
			case apierrors.IsNotFound(err), meta404(err):
			default:
				return fmt.Errorf("check DAC %s (%s): %w", dac, dg.Group, err)
			}
		}
	}
	return nil
}

// meta404 reports a "resource type not served" error — a cluster without
// one of the DAC CRDs has no DAC there.
func meta404(err error) bool {
	var status apierrors.APIStatus
	return errors.As(err, &status) && status.Status().Code == 404
}

// endpointDACNames returns the DACs an endpoint names: its DAC reference
// (the ome.io annotation, or the legacy spec ref) and its namespace when
// that is named after a DAC. Both are checked, so a disagreement between
// them can't hide a live DAC.
func endpointDACNames(obj *unstructured.Unstructured, gvr schema.GroupVersionResource) []string {
	var ref string
	if gvr == inferenceServiceV1GVR {
		ref, _, _ = unstructured.NestedString(obj.Object, "spec", "predictor", "model", "dedicatedAIClusterRef", "name")
	} else {
		ref = obj.GetAnnotations()[dacAnnotation]
	}
	var out []string
	if ref != "" {
		out = append(out, ref)
	}
	if ns := obj.GetNamespace(); models.IsDACName(ns) && ns != ref {
		out = append(out, ns)
	}
	return out
}

// removeFinalizers drops the OME finalizer with a JSON patch guarded by
// resourceVersion, so a concurrent change (e.g. the controller finishing
// after all) fails the patch instead of being overwritten.
func removeFinalizers(ctx context.Context, client dynamic.Interface, gvr schema.GroupVersionResource, obj *unstructured.Unstructured) error {
	patch, err := json.Marshal([]map[string]any{
		{"op": "test", "path": "/metadata/resourceVersion", "value": obj.GetResourceVersion()},
		{"op": "remove", "path": "/metadata/finalizers"},
	})
	if err != nil {
		return fmt.Errorf("marshal finalizer patch: %w", err)
	}
	_, err = client.Resource(gvr).Namespace(obj.GetNamespace()).Patch(ctx, obj.GetName(), types.JSONPatchType, patch, v1.PatchOptions{})
	return err
}
