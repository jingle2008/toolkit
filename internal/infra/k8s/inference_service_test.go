package k8s

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"

	"github.com/jingle2008/toolkit/pkg/models"
)

// isvcDedicated is the shape of a real dedicated InferenceService,
// trimmed to the fields the parser reads plus a few it must ignore
// (predictor component, IngressReady condition).
const isvcDedicated = `
apiVersion: ome.io/v1beta1
kind: InferenceService
metadata:
  name: amaaaaaask7dceyatjkqbeheqhw5ry5dkayifvgqbaikjeg2ys676oc6j3ea
  namespace: amaaaaaa22gykbqa6lppvrrw7lrrclyuk5s4kw27tie7bmwr3j4m5p7hmg7a
  creationTimestamp: "2025-08-20T00:00:00Z"
  labels:
    tenancy-id: aaaaaaaaysjwrhptwmm6ynuvobvsmb3yjv6v6bz7ev2veq22fgzbugsrm53q
  annotations:
    ome.io/dedicated-ai-cluster: amaaaaaa22gykbqa6lppvrrw7lrrclyuk5s4kw27tie7bmwr3j4m5p7hmg7a
    ome.io/deploymentMode: RawDeployment
spec:
  engine:
    maxReplicas: 2
    minReplicas: 1
  model:
    apiGroup: ome.io
    kind: ClusterBaseModel
    name: rerank-v3-5
  runtime:
    apiGroup: ome.io
    kind: ClusterServingRuntime
    name: cohere-rerank-v3-5
status:
  components:
    predictor:
      latestCreatedRevision: "1"
  conditions:
  - status: "True"
    type: EngineReady
  - status: "False"
    type: IngressReady
  - status: "True"
    type: Ready
`

// isvcOnDemand is a shared endpoint: namespace named after the model, no
// tenant, no DAC annotation, no runtime ref, and not ready.
const isvcOnDemand = `
apiVersion: ome.io/v1beta1
kind: InferenceService
metadata:
  name: embed-v4
  namespace: embed-v4
spec:
  engine:
    maxReplicas: 4
    minReplicas: 2
  model:
    kind: ClusterBaseModel
    name: embed-v4
status:
  conditions:
  - status: "False"
    type: Ready
`

func TestParseInferenceService_Dedicated(t *testing.T) {
	t.Parallel()
	got := parseInferenceService(parseYAML(t, isvcDedicated))
	got.Age = "" // wall-clock dependent

	assert.Equal(t, models.InferenceService{
		Name:        "amaaaaaask7dceyatjkqbeheqhw5ry5dkayifvgqbaikjeg2ys676oc6j3ea",
		Namespace:   "amaaaaaa22gykbqa6lppvrrw7lrrclyuk5s4kw27tie7bmwr3j4m5p7hmg7a",
		Type:        models.InferenceServiceDedicated,
		TenantID:    "aaaaaaaaysjwrhptwmm6ynuvobvsmb3yjv6v6bz7ev2veq22fgzbugsrm53q",
		Model:       "rerank-v3-5",
		Runtime:     "cohere-rerank-v3-5",
		MinReplicas: 1,
		MaxReplicas: 2,
		Ready:       "True",
	}, got)
}

func TestParseInferenceService_OnDemand(t *testing.T) {
	t.Parallel()
	got := parseInferenceService(parseYAML(t, isvcOnDemand))

	assert.Equal(t, models.InferenceServiceOnDemand, got.Type)
	assert.Empty(t, got.TenantID)
	assert.Empty(t, got.Runtime)
	assert.Equal(t, "False", got.Ready)
	assert.True(t, got.IsFaulty())
	assert.Equal(t, "2/4", got.Replicas())
	assert.Empty(t, got.Age, "no creationTimestamp means no age")
}

func TestParseInferenceService_Variants(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		src   string
		check func(t *testing.T, s models.InferenceService)
	}{
		{
			name: "DAC namespace without annotation is dedicated",
			src: `
metadata: {name: x, namespace: amaaaaaa-dac}
spec: {model: {kind: ClusterBaseModel, name: m}}
`,
			check: func(t *testing.T, s models.InferenceService) {
				t.Helper()
				assert.Equal(t, models.InferenceServiceDedicated, s.Type)
			},
		},
		{
			name: "namespaced BaseModel ref is an imported model",
			src: `
metadata: {name: x, namespace: amaaaaaa-dac}
spec: {model: {kind: BaseModel, name: amaaaaaa-model}}
`,
			check: func(t *testing.T, s models.InferenceService) {
				t.Helper()
				assert.True(t, s.ImportedModel)
				assert.Equal(t, "amaaaaaa-model", s.Model)
			},
		},
		{
			name: "missing Ready condition reads Unknown",
			src: `
metadata: {name: x, namespace: y}
status: {conditions: [{type: EngineReady, status: "True"}]}
`,
			check: func(t *testing.T, s models.InferenceService) {
				t.Helper()
				assert.Equal(t, "Unknown", s.Ready)
				assert.True(t, s.IsFaulty())
				assert.Equal(t, "0/0", s.Replicas())
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tc.check(t, parseInferenceService(parseYAML(t, tc.src)))
		})
	}
}

func TestLoadInferenceServices_GroupsByNamespace(t *testing.T) {
	t.Parallel()
	listKinds := map[schema.GroupVersionResource]string{
		inferenceServiceGVR: "InferenceServiceList",
	}
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), listKinds,
		parseYAML(t, isvcDedicated), parseYAML(t, isvcOnDemand))

	got, err := LoadInferenceServices(context.Background(), client)
	require.NoError(t, err)

	require.Len(t, got, 2)
	require.Len(t, got["embed-v4"], 1)
	assert.Equal(t, "embed-v4", got["embed-v4"][0].Name)
	dac := got["amaaaaaa22gykbqa6lppvrrw7lrrclyuk5s4kw27tie7bmwr3j4m5p7hmg7a"]
	require.Len(t, dac, 1)
	assert.Equal(t, "rerank-v3-5", dac[0].Model)
	assert.NotEmpty(t, dac[0].Age)
}
