package k8s

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
	cgotesting "k8s.io/client-go/testing"
)

var (
	isvcGR       = schema.GroupResource{Group: "ome.io", Resource: "inferenceservices"}
	errNotFound  = apierrors.NewNotFound(isvcGR, "svc")
	errForbidden = apierrors.NewForbidden(isvcGR, "svc", errors.New("no role"))
)

// onInferenceService tries ome.io, then the legacy API when the object
// isn't on ome.io; a legacy API that can't be read leaves ome.io's
// not-found standing, as listing treats it.
func TestOnInferenceService(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	cases := []struct {
		name         string
		ome, legacy  error
		wantGVR      schema.GroupVersionResource
		wantNotFound bool
		wantErr      error
	}{
		{"on ome.io", nil, boom, inferenceServiceGVR, false, nil},
		{"ome.io fails", boom, nil, inferenceServiceGVR, false, boom},
		{"on legacy", errNotFound, nil, inferenceServiceV1GVR, false, nil},
		{"nowhere", errNotFound, errNotFound, inferenceServiceGVR, true, nil},
		{"legacy forbidden", errNotFound, errForbidden, inferenceServiceGVR, true, nil},
		{"legacy fails", errNotFound, boom, inferenceServiceV1GVR, false, boom},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gvr, err := onInferenceService(func(gvr schema.GroupVersionResource) error {
				if gvr == inferenceServiceGVR {
					return tc.ome
				}
				return tc.legacy
			})
			assert.Equal(t, tc.wantGVR, gvr)
			switch {
			case tc.wantNotFound:
				assert.True(t, apierrors.IsNotFound(err), "%v", err)
			case tc.wantErr != nil:
				require.ErrorIs(t, err, tc.wantErr)
			default:
				require.NoError(t, err)
			}
		})
	}
}

// A role without the legacy DAC API still lists the ome.io DACs.
func TestListDedicatedAIClusters_LegacyForbidden(t *testing.T) {
	t.Parallel()
	client := fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), dacListKinds(),
		makeUnstructuredDACV2("dac-2", "profileA", 3, "tid2"))
	client.PrependReactor("list", "dedicatedaiclusters", func(a cgotesting.Action) (bool, runtime.Object, error) {
		if a.GetResource() == dacV1GVR {
			return true, nil, apierrors.NewForbidden(dacV1GVR.GroupResource(), "", errors.New("no role"))
		}
		return false, nil, nil
	})
	clusters, err := listDedicatedAIClusters(context.Background(), client)
	require.NoError(t, err)
	require.Len(t, clusters, 1)
	assert.Equal(t, "dac-2", clusters[0].Name)
}
