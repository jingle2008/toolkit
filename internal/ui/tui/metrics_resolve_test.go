package tui

import (
	"testing"

	"github.com/charmbracelet/x/ansi"
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
			{Name: "gen", DisplayName: "openai.gen", Capabilities: []string{"TEXT_GENERATION"}},
			{Name: "img", DisplayName: "openai.img", Capabilities: []string{"TEXT_TO_IMAGE"}},
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
	assert.Equal(t, telemetry.FilterDacID, filter.Key)
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
	assert.Equal(t, telemetry.FilterDacID, filter.Key)
	assert.Equal(t, "ocid1.generativeaidedicatedaicluster.oc1.me-abudhabi-1.amaaaaaadac1", filter.Value)
	assert.Equal(t, telemetry.CapabilityChat, capability)
}

func TestResolveMetricsPlan_WorkloadOnDemand(t *testing.T) {
	t.Parallel()
	m := newResolveModel(t)
	w := &models.GPUWorkload{Name: "p", Namespace: "team-x", Model: "gpt"}
	filter, capability, ok, reason := m.resolveMetricsPlan(w)
	require.True(t, ok, reason)
	assert.Equal(t, telemetry.FilterResourceID, filter.Key)
	assert.Equal(t, "openai.gpt-5.5", filter.Value)
	assert.Equal(t, telemetry.CapabilityChat, capability)
}

func TestResolveMetricsPlan_OnDemandClassification(t *testing.T) {
	t.Parallel()
	m := newResolveModel(t)
	w := &models.GPUWorkload{Name: "p", Namespace: "team-x", Model: "mod"}
	filter, capability, ok, reason := m.resolveMetricsPlan(w)
	require.True(t, ok, reason)
	assert.Equal(t, telemetry.FilterResourceID, filter.Key)
	assert.Equal(t, "openai.mod", filter.Value)
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
		assert.Contains(t, keyHelpDescs(cat), "Open metrics", "category %v", cat)
	}
}

func TestResolveMetricsPlan_UnsupportedOnDemand(t *testing.T) {
	t.Parallel()
	m := newResolveModel(t)
	w := &models.GPUWorkload{Name: "p", Namespace: "team-x", Model: "gen"}
	_, _, ok, reason := m.resolveMetricsPlan(w)
	assert.False(t, ok)
	assert.Equal(t, "metrics not supported for this model", reason)
}

func TestResolveMetricsPlan_UnsupportedDedicated(t *testing.T) {
	t.Parallel()
	m := newResolveModel(t)
	// Dedicated workload whose model resolves to an unsupported capability.
	w := &models.GPUWorkload{Name: "p", Namespace: "amaaaaaadac1", Model: "gen"}
	_, _, ok, reason := m.resolveMetricsPlan(w)
	assert.False(t, ok)
	assert.Equal(t, "metrics not supported for this model", reason)
}

func TestResolveMetricsPlan_NewModalityOnDemand(t *testing.T) {
	t.Parallel()
	m := newResolveModel(t)
	w := &models.GPUWorkload{Name: "p", Namespace: "team-x", Model: "img"}
	filter, capability, ok, reason := m.resolveMetricsPlan(w)
	require.True(t, ok, reason)
	assert.Equal(t, telemetry.FilterResourceID, filter.Key)
	assert.Equal(t, "openai.img", filter.Value)
	assert.Equal(t, telemetry.CapabilityTextToImage, capability)
}

func TestResolveMetricsPlan_DedicatedModerationStillErrors(t *testing.T) {
	t.Parallel()
	m := newResolveModel(t)
	// "mod" has TEXT_CLASSIFICATION (unfilterable) → dedicated mode rejects.
	w := &models.GPUWorkload{Name: "p", Namespace: "amaaaaaadac1", Model: "mod"}
	_, _, ok, reason := m.resolveMetricsPlan(w)
	assert.False(t, ok)
	assert.Equal(t, "metrics not available for this model in dedicated mode", reason)
}

func TestFinishMetrics_UnresolvableShowsToast(t *testing.T) {
	t.Parallel()
	m := newResolveModel(t)
	// GPUWorkload with empty Model → resolveMetricsPlan returns ok=false, reason="workload has no model"
	m.finishMetrics(&models.GPUWorkload{Name: "p", Namespace: "amaaaaaadac1"})
	require.NotNil(t, m.toasts.Active(), "toast must be set after finishMetrics on unresolvable item")
	assert.Equal(t, "workload has no model", m.toasts.Active().Msg)
	assert.Equal(t, toastWarn, m.toasts.Active().Sev, "no dashboard for the row is not a failure")
}

func TestResolveMetricsPlan_InferenceService(t *testing.T) {
	t.Parallel()
	m := newResolveModel(t)

	dedicated := &models.InferenceService{Name: "e", Namespace: "amaaaaaadac1", Type: models.InferenceServiceDedicated, Model: "gpt"}
	filter, capability, ok, reason := m.resolveMetricsPlan(dedicated)
	require.True(t, ok, reason)
	assert.Equal(t, telemetry.FilterDacID, filter.Key)
	assert.Equal(t, "ocid1.generativeaidedicatedaicluster.oc1.me-abudhabi-1.amaaaaaadac1", filter.Value)
	assert.Equal(t, telemetry.CapabilityChat, capability)

	onDemand := &models.InferenceService{Name: "gpt", Namespace: "gpt", Type: models.InferenceServiceOnDemand, Model: "gpt"}
	filter, _, ok, reason = m.resolveMetricsPlan(onDemand)
	require.True(t, ok, reason)
	assert.Equal(t, telemetry.FilterResourceID, filter.Key)
	assert.Equal(t, "openai.gpt-5.5", filter.Value)

	_, _, ok, reason = m.resolveMetricsPlan(&models.InferenceService{Name: "e", Type: models.InferenceServiceDedicated})
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestMetricsCatalog_InferenceService(t *testing.T) {
	t.Parallel()
	m := newResolveModel(t)
	cat, need := m.metricsCatalog(&models.InferenceService{Type: models.InferenceServiceOnDemand, Model: "gpt"})
	assert.True(t, need)
	assert.Equal(t, domain.BaseModel, cat)

	cat, need = m.metricsCatalog(&models.InferenceService{Type: models.InferenceServiceDedicated, Model: "amaaaaaaimp"})
	assert.True(t, need)
	assert.Equal(t, domain.ImportedModel, cat)

	_, need = m.metricsCatalog(&models.InferenceService{Type: models.InferenceServiceDedicated})
	assert.False(t, need, "no model, no catalog")
}

// A merged imported model whose primary copy sits in `default` resolves
// to the DAC holding its copy.
func TestResolveMetricsPlan_ImportedModelCopyInDAC(t *testing.T) {
	t.Parallel()
	m := newResolveModel(t)
	im := &models.ImportedModel{
		BaseModel: models.BaseModel{Name: "amaaaaaaim"},
		Namespace: "default",
		Copies:    []string{"amaaaaaadac1"},
	}
	filter, _, ok, reason := m.resolveMetricsPlan(im)
	require.True(t, ok, reason)
	assert.Equal(t, "ocid1.generativeaidedicatedaicluster.oc1.me-abudhabi-1.amaaaaaadac1", filter.Value)
}

// An imported model served by several DACs opens a picker over them;
// choosing one opens that DAC's dashboard.
func TestOpenMetrics_ImportedModelOnSeveralDACsPicks(t *testing.T) {
	t.Parallel()
	m := newResolveModel(t)
	im := &models.ImportedModel{
		BaseModel: models.BaseModel{Name: "amaaaaaaim", Capabilities: []string{"TEXT_RERANK"}},
		Namespace: "default",
		Copies:    []string{"amaaaaaadac1", "amaaaaaadac2"},
	}
	require.Nil(t, m.openMetrics(im))
	require.Equal(t, common.RowPickerView, m.viewMode)
	assert.Equal(t, []domain.Scope{
		{Category: domain.DedicatedAICluster, Name: "amaaaaaadac1"},
		{Category: domain.DedicatedAICluster, Name: "amaaaaaadac2"},
	}, m.rowPicker.Targets())
	view := ansi.Strip(m.rowPickerView())
	assert.Contains(t, view, "Open metrics for amaaaaaaim on")
	assert.Contains(t, view, "enter open metrics")

	// The real choose: it returns the dashboard launch (built, not run),
	// with no error toast.
	_, _ = m.updateRowPickerView(keyPress("down"))
	_, cmd := m.updateRowPickerView(keyPress("enter"))
	require.NotNil(t, cmd, "choosing a DAC launches its dashboard")
	if m.toasts.Active() != nil {
		assert.NotEqual(t, toastError, m.toasts.Active().Sev, m.toasts.Active().Msg)
	}
	assert.Equal(t, common.ListView, m.viewMode)

	// The chosen copy resolves to that DAC's plan.
	filter, _, ok, reason := m.resolveMetricsPlan(&models.ImportedModel{
		BaseModel: im.BaseModel, Namespace: "amaaaaaadac2",
	})
	require.True(t, ok, reason)
	assert.Equal(t, "ocid1.generativeaidedicatedaicluster.oc1.me-abudhabi-1.amaaaaaadac2", filter.Value)
}
