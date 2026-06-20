package tui

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
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

func TestModelCapability(t *testing.T) {
	t.Parallel()
	m := makeTestModel()
	assert.Equal(t, telemetry.CapabilityChat, m.modelCapability("anything"), "nil dataset → chat")

	m.dataset = &models.Dataset{BaseModels: []models.BaseModel{
		{Name: "c", Capabilities: []string{"CHAT"}},
		{Name: "r", Capabilities: []string{"TEXT_RERANK"}},
	}}
	assert.Equal(t, telemetry.CapabilityChat, m.modelCapability("c"))
	assert.Equal(t, telemetry.CapabilityTextRerank, m.modelCapability("r"))
	assert.Equal(t, telemetry.CapabilityChat, m.modelCapability("missing"))
}

func TestCatalogLoaded(t *testing.T) {
	t.Parallel()
	m := makeTestModel() // dataset nil
	assert.False(t, m.catalogLoaded(domain.BaseModel))
	assert.False(t, m.catalogLoaded(domain.ImportedModel))

	m.dataset = &models.Dataset{BaseModels: []models.BaseModel{}}
	assert.True(t, m.catalogLoaded(domain.BaseModel), "non-nil slice counts as loaded")
	assert.False(t, m.catalogLoaded(domain.ImportedModel))

	m.dataset.ImportedModelMap = map[string][]models.ImportedModel{}
	assert.True(t, m.catalogLoaded(domain.ImportedModel))
}

func TestOpenDacMetrics_NoModelLaunchesImmediately(t *testing.T) {
	t.Parallel()
	m := makeTestModel()
	g0 := m.gen
	cmd := m.openDacMetrics(&models.DedicatedAICluster{Name: "d"}) // no ModelName
	require.NotNil(t, cmd)
	assert.Equal(t, g0, m.gen, "no catalog load, so no generation bump")
}

func TestOpenDacMetrics_CatalogLoadedResolvesNow(t *testing.T) {
	t.Parallel()
	m := makeTestModel()
	m.dataset = &models.Dataset{BaseModels: []models.BaseModel{{Name: "r", Capabilities: []string{"TEXT_RERANK"}}}}
	g0 := m.gen
	cmd := m.openDacMetrics(&models.DedicatedAICluster{Name: "d", ModelName: "r"})
	require.NotNil(t, cmd, "launches immediately from the cached base catalog")
	assert.Equal(t, g0, m.gen, "no load dispatched when the catalog is present")
}

func TestOpenDacMetrics_NotLoadedDispatchesLoad(t *testing.T) {
	t.Parallel()
	m := makeTestModel() // dataset nil → base catalog not loaded
	g0 := m.gen
	cmd := m.openDacMetrics(&models.DedicatedAICluster{Name: "d", ModelName: "some-base-model"})
	require.NotNil(t, cmd, "dispatches a catalog load")
	assert.Equal(t, g0+1, m.gen, "a load was dispatched under a fresh generation")
}

func TestOpenDacMetrics_PrefixRoutesToImported(t *testing.T) {
	t.Parallel()
	m := makeTestModel()
	// Base catalog is loaded, but an imported/finetune name (amaaaaaa…)
	// must consult the imported catalog, which is not loaded → dispatch.
	m.dataset = &models.Dataset{BaseModels: []models.BaseModel{}}
	g0 := m.gen
	cmd := m.openDacMetrics(&models.DedicatedAICluster{Name: "d", ModelName: "amaaaaaaxyz"})
	require.NotNil(t, cmd)
	assert.Equal(t, g0+1, m.gen, "imported catalog not loaded → load dispatched despite base being present")
}

func TestHandleDacMetricsCatalogLoaded_StaleGenDropped(t *testing.T) {
	t.Parallel()
	m := makeTestModel()
	got := m.handleDacMetricsCatalogLoaded(dacMetricsCatalogLoadedMsg{
		ocid: "o", modelName: "r", cat: domain.BaseModel,
		base: []models.BaseModel{{Name: "r"}}, gen: m.gen + 1, // stale
	})
	assert.Nil(t, got, "stale load neither caches nor opens")
	assert.Nil(t, m.dataset, "dataset untouched by a stale load")
}

func TestHandleDacMetricsCatalogLoaded_ErrorToastsNoOpen(t *testing.T) {
	t.Parallel()
	m := makeTestModel()
	got := m.handleDacMetricsCatalogLoaded(dacMetricsCatalogLoadedMsg{
		ocid: "o", modelName: "r", cat: domain.BaseModel,
		gen: m.gen, err: errors.New("boom"),
	})
	require.NotNil(t, got, "returns a toast command")
	assert.Nil(t, m.dataset, "no dataset mutation on load error")
}

func TestHandleDacMetricsCatalogLoaded_CachesAndOpens(t *testing.T) {
	t.Parallel()
	m := makeTestModel()
	got := m.handleDacMetricsCatalogLoaded(dacMetricsCatalogLoadedMsg{
		ocid: "o", modelName: "r", cat: domain.BaseModel,
		base: []models.BaseModel{{Name: "r", Capabilities: []string{"TEXT_RERANK"}}}, gen: m.gen,
	})
	require.NotNil(t, got, "opens the dashboard")
	require.NotNil(t, m.dataset)
	assert.Len(t, m.dataset.BaseModels, 1, "catalog cached for later navigation")
}
