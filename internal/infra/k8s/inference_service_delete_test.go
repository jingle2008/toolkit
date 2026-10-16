package k8s

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8stesting "k8s.io/client-go/testing"
)

const (
	orphanNS   = "amaaaaaa-gone"
	orphanName = "amaaaaaa-isvc"
)

var orphanNow = time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)

// orphanISVC is a dedicated endpoint in a DAC-named namespace; extra is
// appended under metadata.
func orphanISVC(extra string) string {
	return `
apiVersion: ome.io/v1beta1
kind: InferenceService
metadata:
  name: ` + orphanName + `
  namespace: ` + orphanNS + `
  annotations:
    ome.io/dedicated-ai-cluster: ` + orphanNS + `
` + extra
}

// stuck is a deletion pending since an hour before orphanNow.
const stuck = `  deletionTimestamp: "2026-10-16T11:00:00Z"
  finalizers: ["inferenceservice.finalizers"]
  resourceVersion: "42"
`

func dacYAML(apiVersion, name string) string {
	return "apiVersion: " + apiVersion + "\nkind: DedicatedAICluster\nmetadata:\n  name: " + name + "\n"
}

func TestDeleteOrphanInferenceService_Refusals(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		objs    []string
		wantErr string
		notOrph bool
	}{
		{name: "shared endpoint", objs: []string{isvcOnDemand}, wantErr: "not a dedicated endpoint", notOrph: true},
		{name: "DAC exists", objs: []string{orphanISVC(""), dacYAML("ome.io/v1beta1", orphanNS)}, wantErr: "its DAC " + orphanNS + " exists (ome.io)", notOrph: true},
		{
			name: "DAC exists in an API the toolkit doesn't list", notOrph: true,
			objs:    []string{orphanISVC(""), dacYAML("gpu-alfred.generative-ai.oracle.com/v1alpha1", orphanNS)},
			wantErr: "exists (gpu-alfred.generative-ai.oracle.com)",
		},
		{
			name: "namespace DAC exists though the annotation names another", notOrph: true,
			objs: []string{
				"apiVersion: ome.io/v1beta1\nkind: InferenceService\nmetadata:\n  name: " + orphanName + "\n  namespace: " + orphanNS +
					"\n  annotations:\n    ome.io/dedicated-ai-cluster: amaaaaaa-other\n",
				dacYAML("ome.io/v1beta1", orphanNS),
			},
			wantErr: "its DAC " + orphanNS + " exists",
		},
		{
			name:    "deletion too recent",
			objs:    []string{orphanISVC("  deletionTimestamp: \"2026-10-16T11:58:00Z\"\n  finalizers: [\"inferenceservice.finalizers\"]\n")},
			wantErr: "deletion requested 2m0s ago; the controller may still be finalizing it — retry after 10m0s",
		},
		{
			name:    "unknown finalizer",
			objs:    []string{orphanISVC("  deletionTimestamp: \"2026-10-16T11:00:00Z\"\n  finalizers: [\"inferenceservice.finalizers\", \"example.com/keep\"]\n")},
			wantErr: "unknown finalizers example.com/keep",
		},
		{name: "missing", objs: []string{isvcOnDemand}, wantErr: "get InferenceService " + orphanNS + "/" + orphanName},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client := newISVCClient(t, tc.objs...)
			ns, name := orphanNS, orphanName
			if tc.name == "shared endpoint" {
				ns, name = "embed-v4", "embed-v4"
			}
			_, err := deleteOrphanInferenceService(context.Background(), client, ns, name, orphanNow)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
			assert.Equal(t, tc.notOrph, errors.Is(err, ErrNotOrphan))
			for _, a := range client.Actions() {
				assert.Containsf(t, []string{"get", "list"}, a.GetVerb(), "a refusal must not change anything: %s", a.GetVerb())
			}
		})
	}
}

func TestDeleteOrphanInferenceService_Deletes(t *testing.T) {
	t.Parallel()
	client := newISVCClient(t, orphanISVC(""))
	outcome, err := deleteOrphanInferenceService(context.Background(), client, orphanNS, orphanName, orphanNow)
	require.NoError(t, err)
	assert.Equal(t, OrphanDeleted, outcome)
	_, err = client.Resource(inferenceServiceGVR).Namespace(orphanNS).Get(context.Background(), orphanName, v1.GetOptions{})
	assert.True(t, apierrors.IsNotFound(err))
}

// A deletion stuck on the OME finalizer is completed by removing it.
func TestDeleteOrphanInferenceService_Finalizes(t *testing.T) {
	t.Parallel()
	client := newISVCClient(t, orphanISVC(stuck))
	outcome, err := deleteOrphanInferenceService(context.Background(), client, orphanNS, orphanName, orphanNow)
	require.NoError(t, err)
	assert.Equal(t, OrphanFinalized, outcome)
	var patched bool
	for _, a := range client.Actions() {
		if p, ok := a.(k8stesting.PatchAction); ok {
			patched = true
			assert.JSONEq(t, `[{"op":"test","path":"/metadata/resourceVersion","value":"42"},{"op":"remove","path":"/metadata/finalizers"}]`, string(p.GetPatch()))
		}
		assert.NotEqual(t, "delete", a.GetVerb(), "a pending deletion isn't re-issued")
	}
	assert.True(t, patched)
}

func TestDeleteOrphanInferenceService_Legacy(t *testing.T) {
	t.Parallel()
	legacy := `
apiVersion: ome.oracle.com/v1alpha1
kind: InferenceService
metadata:
  name: ` + orphanName + `
  namespace: legacy-ns
spec:
  predictor:
    model:
      dedicatedAIClusterRef:
        name: amaaaaaa-legacy-dac
`
	client := newISVCClient(t, legacy)
	outcome, err := deleteOrphanInferenceService(context.Background(), client, "legacy-ns", orphanName, orphanNow)
	require.NoError(t, err)
	assert.Equal(t, OrphanDeleted, outcome)

	client = newISVCClient(t, legacy, dacYAML("ome.oracle.com/v1alpha1", "amaaaaaa-legacy-dac"))
	_, err = deleteOrphanInferenceService(context.Background(), client, "legacy-ns", orphanName, orphanNow)
	require.ErrorIs(t, err, ErrNotOrphan, "the spec ref names a live legacy DAC")
}

// A DAC lookup that fails for any reason but "not there" refuses: the
// guard fails closed.
func TestDeleteOrphanInferenceService_DACLookupFailsClosed(t *testing.T) {
	t.Parallel()
	client := newISVCClient(t, orphanISVC(""))
	client.PrependReactor("get", "dedicatedaiclusters", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewForbidden(dacV2GVR.GroupResource(), orphanNS, errors.New("rbac"))
	})
	_, err := deleteOrphanInferenceService(context.Background(), client, orphanNS, orphanName, orphanNow)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "check DAC "+orphanNS)
	assert.NotErrorIs(t, err, ErrNotOrphan)
}
