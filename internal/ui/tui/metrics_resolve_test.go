package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/infra/telemetry"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"
	"github.com/jingle2008/toolkit/pkg/models"
)

func newResolveModel(t *testing.T) *Model {
	t.Helper()
	m := makeTestModel()
	m.environment = models.Environment{Realm: "oc1", Region: "me-abudhabi-1", Type: "prod"}
	m.dataset = &models.Dataset{
		BaseModels: []models.BaseModel{
			{Name: "gpt", DisplayName: "openai.gpt-5.5", Capabilities: []string{"CHAT"}},
			{Name: "mod", DisplayName: "openai.mod", Capabilities: []string{"TEXT_CLASSIFICATION"}},
		},
	}
	return m
}

func keyHelpDescs(cat domain.Category) []string {
	ctx := keys.ResolveKeys(cat, common.ListView).Context
	out := make([]string, 0, len(ctx))
	for _, b := range ctx {
		out = append(out, b.Help().Desc)
	}
	return out
}

func TestResolveMetricsPlan_ImportedModelDedicated(t *testing.T) {
	t.Parallel()
	m := newResolveModel(t)
	im := &models.ImportedModel{
		BaseModel: models.BaseModel{Name: "amaaaaaaim", Capabilities: []string{"TEXT_RERANK"}},
		Namespace: "amaaaaaadac1",
	}
	filter, capability, ok, reason := m.resolveMetricsPlan(im)
	require.True(t, ok, reason)
	assert.Equal(t, telemetry.FilterDacId, filter.Key)
	assert.Equal(t, "ocid1.generativeaidedicatedaicluster.oc1.me-abudhabi-1.amaaaaaadac1", filter.Value)
	assert.Equal(t, telemetry.CapabilityTextRerank, capability)
}

func TestResolveMetricsPlan_ImportedModelNotADAC(t *testing.T) {
	t.Parallel()
	m := newResolveModel(t)
	im := &models.ImportedModel{BaseModel: models.BaseModel{Name: "x"}, Namespace: "team-x"}
	_, _, ok, reason := m.resolveMetricsPlan(im)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestResolveMetricsPlan_WorkloadEmptyModel(t *testing.T) {
	t.Parallel()
	m := newResolveModel(t)
	_, _, ok, reason := m.resolveMetricsPlan(&models.GPUWorkload{Name: "p", Namespace: "amaaaaaadac1"})
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestResolveMetricsPlan_WorkloadDedicated(t *testing.T) {
	t.Parallel()
	m := newResolveModel(t)
	w := &models.GPUWorkload{Name: "p", Namespace: "amaaaaaadac1", Model: "gpt"}
	filter, capability, ok, reason := m.resolveMetricsPlan(w)
	require.True(t, ok, reason)
	assert.Equal(t, telemetry.FilterDacId, filter.Key)
	assert.Equal(t, "ocid1.generativeaidedicatedaicluster.oc1.me-abudhabi-1.amaaaaaadac1", filter.Value)
	assert.Equal(t, telemetry.CapabilityChat, capability)
}

func TestResolveMetricsPlan_WorkloadOnDemand(t *testing.T) {
	t.Parallel()
	m := newResolveModel(t)
	w := &models.GPUWorkload{Name: "p", Namespace: "team-x", Model: "gpt"}
	filter, capability, ok, reason := m.resolveMetricsPlan(w)
	require.True(t, ok, reason)
	assert.Equal(t, telemetry.FilterResourceId, filter.Key)
	assert.Equal(t, "openai.gpt-5.5", filter.Value)
	assert.Equal(t, telemetry.CapabilityChat, capability)
}

func TestResolveMetricsPlan_OnDemandClassification(t *testing.T) {
	t.Parallel()
	m := newResolveModel(t)
	w := &models.GPUWorkload{Name: "p", Namespace: "team-x", Model: "mod"}
	filter, capability, ok, reason := m.resolveMetricsPlan(w)
	require.True(t, ok, reason)
	assert.Equal(t, telemetry.FilterResourceId, filter.Key)
	assert.Equal(t, telemetry.CapabilityTextClassification, capability)
}

func TestResolveMetricsPlan_OnDemandModelNotFound(t *testing.T) {
	t.Parallel()
	m := newResolveModel(t)
	w := &models.GPUWorkload{Name: "p", Namespace: "team-x", Model: "missing"}
	_, _, ok, reason := m.resolveMetricsPlan(w)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestResolveMetricsPlan_DedicatedClassificationNoOp(t *testing.T) {
	t.Parallel()
	m := newResolveModel(t)
	// Dedicated workload whose model resolves to a content-moderation
	// capability is unreachable in dedicated mode → toast, no open.
	w := &models.GPUWorkload{Name: "p", Namespace: "amaaaaaadac1", Model: "mod"}
	_, _, ok, reason := m.resolveMetricsPlan(w)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestMetricsCatalog_NewCategories(t *testing.T) {
	t.Parallel()
	m := newResolveModel(t)
	_, need := m.metricsCatalog(&models.ImportedModel{Namespace: "amaaaaaadac1"})
	assert.False(t, need, "imported model carries its own capabilities")

	cat, need := m.metricsCatalog(&models.GPUWorkload{Namespace: "team-x", Model: "gpt"})
	assert.True(t, need)
	assert.Equal(t, domain.BaseModel, cat, "on-demand matches the base catalog")

	cat, need = m.metricsCatalog(&models.GPUWorkload{Namespace: "amaaaaaadac1", Model: "amaaaaaaimp"})
	assert.True(t, need)
	assert.Equal(t, domain.ImportedModel, cat, "dedicated workload with an imported model name")

	_, need = m.metricsCatalog(&models.GPUWorkload{Namespace: "team-x"}) // empty model
	assert.False(t, need)
}

func TestKeys_OpenMetricsOnNewCategories(t *testing.T) {
	t.Parallel()
	for _, cat := range []domain.Category{domain.GPUWorkload, domain.ImportedModel, domain.DedicatedAICluster} {
		assert.Contains(t, keyHelpDescs(cat), "Open Metrics", "category %v", cat)
	}
}

func TestFinishMetrics_UnresolvableShowsToast(t *testing.T) {
	t.Parallel()
	m := newResolveModel(t)
	// GPUWorkload with empty Model → resolveMetricsPlan returns ok=false, reason="workload has no model"
	cmd := m.finishMetrics(&models.GPUWorkload{Name: "p", Namespace: "amaaaaaadac1"})
	assert.NotNil(t, cmd, "a toast cmd must be dispatched for unresolvable items with a reason")
}
