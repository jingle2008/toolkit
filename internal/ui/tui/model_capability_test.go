package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/infra/telemetry"
	"github.com/jingle2008/toolkit/pkg/models"
)

func TestCapabilityForModel(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		model *models.BaseModel
		want  telemetry.Capability
	}{
		{"nil", nil, telemetry.CapabilityChat},
		{"finetune", &models.BaseModel{Type: "Fine-tuning", Capabilities: []string{"TEXT_RERANK"}}, telemetry.CapabilityChat},
		{"chat", &models.BaseModel{Capabilities: []string{"CHAT"}}, telemetry.CapabilityChat},
		{"rerank", &models.BaseModel{Capabilities: []string{"TEXT_RERANK"}}, telemetry.CapabilityTextRerank},
		{"embed", &models.BaseModel{Capabilities: []string{"TEXT_EMBEDDINGS"}}, telemetry.CapabilityTextEmbeddings},
		// Precedence CHAT > RERANK > EMBED when several are present.
		{"multi", &models.BaseModel{Capabilities: []string{"TEXT_EMBEDDINGS", "CHAT"}}, telemetry.CapabilityChat},
		{"unknown", &models.BaseModel{Capabilities: []string{"SOMETHING_ELSE"}}, telemetry.CapabilityChat},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, capabilityForModel(tc.model))
		})
	}
}

func TestOpenDacMetrics_NoModelLaunchesImmediately(t *testing.T) {
	t.Parallel()
	m := makeTestModel()
	cmd := m.openDacMetrics(&models.DedicatedAICluster{Name: "d"}) // no ModelName
	require.NotNil(t, cmd)
	assert.Nil(t, m.metricsPending, "no catalog wait when the DAC has no model")
}

func TestOpenDacMetrics_ResolvesFromLoadedBase(t *testing.T) {
	t.Parallel()
	m := makeTestModel()
	m.dataset = &models.Dataset{BaseModels: []models.BaseModel{{Name: "r", Capabilities: []string{"TEXT_RERANK"}}}}
	cmd := m.openDacMetrics(&models.DedicatedAICluster{Name: "d", ModelName: "r"})
	require.NotNil(t, cmd, "launches immediately from the cached catalog")
	assert.Nil(t, m.metricsPending, "resolved without waiting on a load")
}

func TestOpenDacMetrics_CatalogsNotLoadedDefers(t *testing.T) {
	t.Parallel()
	m := makeTestModel() // m.dataset is nil → catalogs not loaded
	cmd := m.openDacMetrics(&models.DedicatedAICluster{Name: "d", ModelName: "some-model"})
	require.NotNil(t, cmd, "dispatches a catalog load")
	require.NotNil(t, m.metricsPending, "open is deferred until catalogs load")
	assert.Equal(t, "some-model", m.metricsPending.modelName)
	assert.Equal(t, m.gen, m.metricsPending.gen, "pending keyed to the dispatched load generation")
}

func TestAdvanceMetrics_FallsBackToImported(t *testing.T) {
	t.Parallel()
	m := makeTestModel()
	// Base loaded but the model is absent, imported not loaded yet.
	m.dataset = &models.Dataset{BaseModels: []models.BaseModel{}}
	m.metricsPending = &pendingMetrics{ocid: "o", modelName: "e"}

	cmd := m.advanceMetrics()
	require.NotNil(t, cmd, "dispatches the imported-model load")
	require.NotNil(t, m.metricsPending, "still waiting")
	assert.Equal(t, m.gen, m.metricsPending.gen)

	// Imported now loaded and carries the model → resolves.
	m.dataset.ImportedModelMap = map[string][]models.ImportedModel{
		"t": {{BaseModel: models.BaseModel{Name: "e", Capabilities: []string{"TEXT_EMBEDDINGS"}}}},
	}
	cmd = m.advanceMetrics()
	require.NotNil(t, cmd)
	assert.Nil(t, m.metricsPending, "resolved from the imported catalog")
}

func TestResumeMetrics_GenGuard(t *testing.T) {
	t.Parallel()
	m := makeTestModel()
	m.dataset = &models.Dataset{BaseModels: []models.BaseModel{{Name: "r", Capabilities: []string{"TEXT_RERANK"}}}}
	m.metricsPending = &pendingMetrics{ocid: "o", modelName: "r", gen: 7}

	// A load from a different generation (e.g. navigation) must not resume.
	assert.Nil(t, m.resumeMetrics(6))
	require.NotNil(t, m.metricsPending, "pending untouched by an unrelated load")

	// The matching generation resumes and resolves.
	cmd := m.resumeMetrics(7)
	require.NotNil(t, cmd)
	assert.Nil(t, m.metricsPending)
}

func TestHandleBaseModelsLoaded_ResumesAndCaches(t *testing.T) {
	t.Parallel()
	m := makeTestModel()
	m.metricsPending = &pendingMetrics{ocid: "o", modelName: "r", gen: m.gen}

	cmd := m.handleBaseModelsLoaded([]models.BaseModel{{Name: "r", Capabilities: []string{"TEXT_RERANK"}}}, m.gen)
	require.NotNil(t, cmd, "populates the catalog and resumes the pending open")
	assert.Nil(t, m.metricsPending, "resolved from the just-loaded base catalog")
	require.NotNil(t, m.dataset)
	assert.Len(t, m.dataset.BaseModels, 1, "catalog is cached for later navigation")
}

func TestHandleBaseModelsLoaded_StaleGenKeepsPending(t *testing.T) {
	t.Parallel()
	m := makeTestModel()
	m.metricsPending = &pendingMetrics{ocid: "o", modelName: "r", gen: m.gen}

	got := m.handleBaseModelsLoaded([]models.BaseModel{{Name: "r"}}, m.gen+1) // stale
	assert.Nil(t, got, "stale load does not resume")
	require.NotNil(t, m.metricsPending, "pending survives a stale load")
}
