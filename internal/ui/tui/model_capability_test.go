package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/pkg/models"
)

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
	g0 := m.gens.msg
	cmd := m.openMetrics(&models.DedicatedAICluster{Name: "d"}) // no ModelName
	require.NotNil(t, cmd)
	assert.Equal(t, g0, m.gens.msg, "no catalog load, so no generation bump")
}

func TestOpenDacMetrics_CatalogLoadedResolvesNow(t *testing.T) {
	t.Parallel()
	m := makeTestModel()
	m.dataset = &models.Dataset{BaseModels: []models.BaseModel{{Name: "r", Capabilities: []string{"TEXT_RERANK"}}}}
	g0 := m.gens.msg
	cmd := m.openMetrics(&models.DedicatedAICluster{Name: "d", ModelName: "r"})
	require.NotNil(t, cmd, "launches immediately from the cached base catalog")
	assert.Equal(t, g0, m.gens.msg, "no load dispatched when the catalog is present")
}

func TestOpenDacMetrics_NotLoadedDispatchesLoad(t *testing.T) {
	t.Parallel()
	m := makeTestModel() // dataset nil → base catalog not loaded
	g0 := m.gens.msg
	cmd := m.openMetrics(&models.DedicatedAICluster{Name: "d", ModelName: "some-base-model"})
	require.NotNil(t, cmd, "dispatches a catalog load")
	assert.Equal(t, g0+1, m.gens.msg, "a load was dispatched under a fresh generation")
}

func TestOpenDacMetrics_PrefixRoutesToImported(t *testing.T) {
	t.Parallel()
	m := makeTestModel()
	// Base catalog is loaded, but an imported/finetune name (amaaaaaa…)
	// must consult the imported catalog, which is not loaded → dispatch.
	m.dataset = &models.Dataset{BaseModels: []models.BaseModel{}}
	g0 := m.gens.msg
	cmd := m.openMetrics(&models.DedicatedAICluster{Name: "d", ModelName: "amaaaaaaxyz"})
	require.NotNil(t, cmd)
	assert.Equal(t, g0+1, m.gens.msg, "imported catalog not loaded → load dispatched despite base being present")
}

func TestHandleOpenMetricsTrigger_StaleGenNoOpen(t *testing.T) {
	t.Parallel()
	m := makeTestModel()
	m.dataset = &models.Dataset{BaseModels: []models.BaseModel{{Name: "r", Capabilities: []string{"TEXT_RERANK"}}}}
	got := m.handleOpenMetricsTrigger(openMetricsTriggerMsg{
		item: &models.DedicatedAICluster{Name: "d", ModelName: "r"}, cat: domain.BaseModel, gen: m.gens.msg + 1,
	})
	assert.Nil(t, got, "stale generation does not open")
}

func TestHandleOpenMetricsTrigger_CatalogNotLoadedNoOpen(t *testing.T) {
	t.Parallel()
	m := makeTestModel() // dataset nil → catalog load must have failed/dropped
	got := m.handleOpenMetricsTrigger(openMetricsTriggerMsg{
		item: &models.DedicatedAICluster{Name: "d", ModelName: "r"}, cat: domain.BaseModel, gen: m.gens.msg,
	})
	assert.Nil(t, got, "no open when the catalog isn't loaded")
}

func TestHandleOpenMetricsTrigger_OpensWhenLoaded(t *testing.T) {
	t.Parallel()
	m := makeTestModel()
	m.dataset = &models.Dataset{BaseModels: []models.BaseModel{{Name: "r", Capabilities: []string{"TEXT_RERANK"}}}}
	got := m.handleOpenMetricsTrigger(openMetricsTriggerMsg{
		item: &models.DedicatedAICluster{Name: "d", ModelName: "r"}, cat: domain.BaseModel, gen: m.gens.msg,
	})
	require.NotNil(t, got, "opens the dashboard once the catalog is applied")
}
