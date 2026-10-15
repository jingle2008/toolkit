package k8s

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

func newISVCClient(t *testing.T, srcs ...string) *dynamicfake.FakeDynamicClient {
	t.Helper()
	listKinds := map[schema.GroupVersionResource]string{
		inferenceServiceGVR:       "InferenceServiceList",
		legacyInferenceServiceGVR: "InferenceServiceList",
	}
	objs := make([]runtime.Object, 0, len(srcs))
	for _, src := range srcs {
		objs = append(objs, parseYAML(t, src))
	}
	return dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), listKinds, objs...)
}

// isvcEmbedOtherNS shares embed-v4's name in a second namespace, making a
// namespace-less lookup ambiguous.
const isvcEmbedOtherNS = `
apiVersion: ome.io/v1beta1
kind: InferenceService
metadata:
  name: embed-v4
  namespace: amaaaaaa-dac
`

func TestResolveInferenceService(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		objs      []string
		namespace string
		target    string
		want      string
		wantErr   string
	}{
		{name: "explicit namespace", objs: []string{isvcOnDemand}, namespace: "embed-v4", target: "embed-v4", want: "embed-v4"},
		{name: "explicit namespace missing", objs: []string{isvcOnDemand}, namespace: "other", target: "embed-v4", wantErr: "get InferenceService other/embed-v4"},
		{name: "unique across namespaces", objs: []string{isvcOnDemand, isvcDedicated}, target: "embed-v4", want: "embed-v4"},
		{name: "not found anywhere", objs: []string{isvcDedicated}, target: "embed-v4", wantErr: `"embed-v4" not found in any namespace`},
		{
			name: "ambiguous lists namespaces", objs: []string{isvcOnDemand, isvcEmbedOtherNS}, target: "embed-v4",
			wantErr: "multiple namespaces (amaaaaaa-dac, embed-v4); specify a namespace",
		},
		{
			name: "legacy-only name is refused", objs: []string{isvcLegacyDedicated}, target: "amaaaaaa7mjirbaaxtzngfz2lszq4nadg2xwhn2u4gyri7idzunxg6rl5tjq",
			wantErr: "restart is not supported",
		},
		{
			name: "legacy-only name in namespace is refused", objs: []string{isvcLegacyDedicated},
			namespace: "amaaaaaa7mjirbaaoocrt6sskewo4u3ttueiqw6y6tmtmbnc4eefwqu37isa", target: "amaaaaaa7mjirbaaxtzngfz2lszq4nadg2xwhn2u4gyri7idzunxg6rl5tjq",
			wantErr: "restart is not supported",
		},
		{name: "ambiguity resolved by namespace", objs: []string{isvcOnDemand, isvcEmbedOtherNS}, namespace: "amaaaaaa-dac", target: "embed-v4", want: "amaaaaaa-dac"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := resolveInferenceService(context.Background(), newISVCClient(t, tc.objs...), tc.namespace, tc.target)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				if strings.Contains(tc.wantErr, "not supported") {
					require.ErrorIs(t, err, ErrLegacyInferenceService)
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestRestartInferenceService_StampsRestartedAt(t *testing.T) {
	t.Parallel()
	client := newISVCClient(t, isvcDedicated)
	ns := "amaaaaaa22gykbqa6lppvrrw7lrrclyuk5s4kw27tie7bmwr3j4m5p7hmg7a"
	name := "amaaaaaask7dceyatjkqbeheqhw5ry5dkayifvgqbaikjeg2ys676oc6j3ea"
	now := time.Date(2026, 10, 14, 15, 4, 5, 0, time.FixedZone("PDT", -7*3600))

	require.NoError(t, restartInferenceService(context.Background(), client, ns, name, now))

	got, err := client.Resource(inferenceServiceGVR).Namespace(ns).Get(context.Background(), name, v1.GetOptions{})
	require.NoError(t, err)
	annos := got.GetAnnotations()
	assert.Equal(t, "2026-10-14T22:04:05Z", annos[restartedAtAnnotation], "value is UTC RFC3339")
	assert.Equal(t, "RawDeployment", annos["ome.io/deploymentMode"], "merge patch keeps other annotations")

	// A second restart overwrites the value (kubectl annotate --overwrite).
	require.NoError(t, restartInferenceService(context.Background(), client, ns, name, now.Add(time.Minute)))
	got, err = client.Resource(inferenceServiceGVR).Namespace(ns).Get(context.Background(), name, v1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "2026-10-14T22:05:05Z", got.GetAnnotations()[restartedAtAnnotation])
}

func TestRestartInferenceService_NotFound(t *testing.T) {
	t.Parallel()
	err := restartInferenceService(context.Background(), newISVCClient(t), "ns", "missing", time.Now())
	require.ErrorContains(t, err, "restart InferenceService ns/missing")
}
