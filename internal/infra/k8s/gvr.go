package k8s

import (
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// The APIs the toolkit reads. ome.io/v1beta1 is OME's current API;
// ome.oracle.com/v1alpha1 is the legacy one some clusters still serve,
// and none may (legacyAPIUnavailable).
var (
	clusterBaseModelGVR      = schema.GroupVersionResource{Group: "ome.io", Version: "v1beta1", Resource: "clusterbasemodels"}
	baseModelGVR             = schema.GroupVersionResource{Group: "ome.io", Version: "v1beta1", Resource: "basemodels"}
	clusterServingRuntimeGVR = schema.GroupVersionResource{Group: "ome.io", Version: "v1beta1", Resource: "clusterservingruntimes"}
	dacV2GVR                 = schema.GroupVersionResource{Group: "ome.io", Version: "v1beta1", Resource: "dedicatedaiclusters"}
	dacProfileGVR            = schema.GroupVersionResource{Group: "ome.io", Version: "v1beta1", Resource: "dedicatedaiclusterprofiles"}
	inferenceServiceGVR      = schema.GroupVersionResource{Group: "ome.io", Version: "v1beta1", Resource: "inferenceservices"}

	dacV1GVR              = schema.GroupVersionResource{Group: "ome.oracle.com", Version: "v1alpha1", Resource: "dedicatedaiclusters"}
	inferenceServiceV1GVR = schema.GroupVersionResource{Group: "ome.oracle.com", Version: "v1alpha1", Resource: "inferenceservices"}

	podGVR = schema.GroupVersionResource{Version: "v1", Resource: "pods"}
)

// labelTenancyID is the label naming the tenancy a resource belongs to.
const labelTenancyID = "tenancy-id"

// legacyAPIUnavailable reports whether err means the legacy
// ome.oracle.com API can't be read at all — the CRD isn't installed
// (NotFound) or the user's role doesn't cover it (Forbidden) — so it
// should contribute nothing rather than fail the ome.io results.
func legacyAPIUnavailable(err error) bool {
	return apierrors.IsNotFound(err) || apierrors.IsForbidden(err)
}

// onInferenceService runs try against the ome.io InferenceService API
// and, when the object isn't there, the legacy one — an endpoint lives in
// exactly one — returning the API it ran against. A legacy API that can't
// be read counts as not having it, as it does for listing, so the ome.io
// error stands; any other legacy error is returned.
func onInferenceService(try func(gvr schema.GroupVersionResource) error) (schema.GroupVersionResource, error) {
	err := try(inferenceServiceGVR)
	if !apierrors.IsNotFound(err) {
		return inferenceServiceGVR, err
	}
	legacyErr := try(inferenceServiceV1GVR)
	switch {
	case legacyErr == nil:
		return inferenceServiceV1GVR, nil
	case legacyAPIUnavailable(legacyErr):
		return inferenceServiceGVR, err
	default:
		return inferenceServiceV1GVR, legacyErr
	}
}
