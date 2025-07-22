package k8s

import (
	"context"
	"testing"

	"github.com/jingle2008/toolkit/pkg/models"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

func newCBM(name string, spec, status map[string]any, labels, ann map[string]string) *unstructured.Unstructured {
	// DeepCopyJSONValue cannot handle map[string]string, so convert to map[string]any
	labelsAny := map[string]any{}
	for k, v := range labels {
		labelsAny[k] = v
	}
	annAny := map[string]any{}
	for k, v := range ann {
		annAny[k] = v
	}
	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "ome.io/v1beta1",
			"kind":       "ClusterBaseModel",
			"metadata": map[string]any{
				"name":        name,
				"labels":      labelsAny,
				"annotations": annAny,
			},
			"spec":   spec,
			"status": status,
		},
	}
	return obj
}

func TestLoadBaseModels_FakeDynamic(t *testing.T) {
	ctx := context.Background()

	// Full-featured CR
	spec1 := map[string]any{
		"displayName":       "Display",
		"version":           "v1",
		"vendor":            "ACME",
		"maxTokens":         int64(2048),
		"modelCapabilities": []any{"capA", "capB"},
		"additionalMetadata": map[string]any{
			"internalName":       "internal-foo",
			"dacShapeConfigs":    "compatibleDACShapes:\n- name: SHAPE1\n  quotaUnit: 2\n  default: true\n",
			"image-text-to-text": "true",
		},
		"modelParameterSize": "7B",
	}
	status1 := map[string]any{
		"state": "Ready",
	}
	labels1 := map[string]string{
		"genai-model-deprecated-date":        "2025-01-01",
		"genai-model-on-demand-retired-date": "2025-02-01",
		"genai-model-dedicated-retired-date": "2025-03-01",
	}
	ann1 := map[string]string{
		"models.ome.io/runtime":                 "python:3.10",
		"models.ome.io/lifecycle-phase":         "DEPRECATED",
		"models.ome.io/experimental":            "true",
		"models.ome.io/internal":                "true",
		"ome.io/base-model-decryption-key-name": "vault-key",
	}

	// Minimal CR
	spec2 := map[string]any{
		"version": "v2",
	}
	status2 := map[string]any{
		"state": "Creating",
	}
	labels2 := map[string]string{}
	ann2 := map[string]string{}

	obj1 := newCBM("foo", spec1, status1, labels1, ann1)
	obj2 := newCBM("bar", spec2, status2, labels2, ann2)

	scheme := runtime.NewScheme()
	client := dynamicfake.NewSimpleDynamicClient(scheme, obj1, obj2)

	modelsOut, err := LoadBaseModels(ctx, client)
	assert.NoError(t, err)
	assert.Len(t, modelsOut, 2)

	// Sort by Name to ensure deterministic order
	var m, m2 any
	if modelsOut[0].Name == "foo" {
		m = modelsOut[0]
		m2 = modelsOut[1]
	} else {
		m = modelsOut[1]
		m2 = modelsOut[0]
	}

	// Full-featured assertions
	bm := m.(models.BaseModel)
	assert.Equal(t, "internal-foo", bm.InternalName)
	assert.Equal(t, "foo", bm.Name)
	assert.Equal(t, "Display", bm.DisplayName)
	assert.Equal(t, "v1", bm.Version)
	assert.Equal(t, "ACME", bm.Vendor)
	assert.Equal(t, 2048, bm.MaxTokens)
	assert.Equal(t, "vault-key", bm.VaultKey)
	assert.Equal(t, "DEPRECATED", bm.LifeCyclePhase)
	assert.True(t, bm.IsExperimental)
	assert.True(t, bm.IsInternal)
	assert.ElementsMatch(t, []string{"capA", "capB"}, bm.Capabilities)
	assert.Equal(t, "python:3.10", bm.Runtime)
	assert.Equal(t, "Serving", bm.Type)
	assert.Equal(t, "2025-01-01", bm.DeprecatedDate)
	assert.Equal(t, "2025-02-01", bm.OnDemandRetiredDate)
	assert.Equal(t, "2025-03-01", bm.DedicatedRetiredDate)
	assert.True(t, bm.IsImageTextToText)
	assert.Equal(t, "7B", bm.ParameterSize)
	assert.Equal(t, "Ready", bm.Status)
	if assert.NotNil(t, bm.DacShapeConfigs) && assert.NotEmpty(t, bm.DacShapeConfigs.CompatibleDACShapes) {
		assert.Equal(t, "SHAPE1", bm.DacShapeConfigs.CompatibleDACShapes[0].Name)
		assert.Equal(t, 2, bm.DacShapeConfigs.CompatibleDACShapes[0].QuotaUnit)
		assert.True(t, bm.DacShapeConfigs.CompatibleDACShapes[0].Default)
	}

	// Minimal assertions
	bm2 := m2.(models.BaseModel)
	assert.Equal(t, "bar", bm2.InternalName)
	assert.Equal(t, "bar", bm2.Name)
	assert.Equal(t, "v2", bm2.Version)
	assert.Equal(t, "", bm2.DisplayName)
	assert.Equal(t, "", bm2.Vendor)
	assert.Equal(t, 0, bm2.MaxTokens)
	assert.Equal(t, "", bm2.VaultKey)
	assert.Equal(t, "", bm2.LifeCyclePhase)
	assert.False(t, bm2.IsExperimental)
	assert.False(t, bm2.IsInternal)
	assert.Empty(t, bm2.Capabilities)
	assert.Equal(t, "", bm2.Runtime)
	assert.Equal(t, "", bm2.Type)
	assert.Equal(t, "", bm2.DeprecatedDate)
	assert.Equal(t, "", bm2.OnDemandRetiredDate)
	assert.Equal(t, "", bm2.DedicatedRetiredDate)
	assert.False(t, bm2.IsImageTextToText)
	assert.Equal(t, "", bm2.ParameterSize)
	assert.Equal(t, "Creating", bm2.Status)
	assert.Nil(t, bm2.DacShapeConfigs)
}
