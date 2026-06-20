package tui

import (
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
		{"classification", &models.BaseModel{Capabilities: []string{"TEXT_CLASSIFICATION"}}, telemetry.CapabilityTextClassification},
		{"imagemod", &models.BaseModel{Capabilities: []string{"IMAGE_CONTENT_MODERATION"}}, telemetry.CapabilityImageContentModeration},
		{"chat-over-classification", &models.BaseModel{Capabilities: []string{"TEXT_CLASSIFICATION", "CHAT"}}, telemetry.CapabilityChat},
		{"embed-over-classification", &models.BaseModel{Capabilities: []string{"TEXT_CLASSIFICATION", "TEXT_EMBEDDINGS"}}, telemetry.CapabilityTextEmbeddings},
		{"classification-over-imagemod", &models.BaseModel{Capabilities: []string{"IMAGE_CONTENT_MODERATION", "TEXT_CLASSIFICATION"}}, telemetry.CapabilityTextClassification},
		{"text_to_text", &models.BaseModel{Capabilities: []string{"TEXT_TO_TEXT"}}, telemetry.CapabilityChat},
		{"image_text_to_text", &models.BaseModel{Capabilities: []string{"IMAGE_TEXT_TO_TEXT"}}, telemetry.CapabilityChat},
		{"chat-over-t2t", &models.BaseModel{Capabilities: []string{"TEXT_TO_TEXT", "CHAT"}}, telemetry.CapabilityChat},
		{"embedding-synonym", &models.BaseModel{Capabilities: []string{"EMBEDDING"}}, telemetry.CapabilityTextEmbeddings},
		{"content-moderation-synonym", &models.BaseModel{Capabilities: []string{"CONTENT_MODERATION"}}, telemetry.CapabilityTextClassification},
		{"text_to_image", &models.BaseModel{Capabilities: []string{"TEXT_TO_IMAGE"}}, telemetry.CapabilityTextToImage},
		{"image_text_to_image", &models.BaseModel{Capabilities: []string{"IMAGE_TEXT_TO_IMAGE"}}, telemetry.CapabilityImageTextToImage},
		{"text_to_audio", &models.BaseModel{Capabilities: []string{"TEXT_TO_AUDIO"}}, telemetry.CapabilityTextToAudio},
		{"audio_to_text", &models.BaseModel{Capabilities: []string{"AUDIO_TO_TEXT"}}, telemetry.CapabilityAudioToText},
		{"unsupported-only", &models.BaseModel{Capabilities: []string{"REALTIME"}}, telemetry.CapabilityUnsupported},
		{"supported-wins", &models.BaseModel{Capabilities: []string{"TEXT_GENERATION", "CHAT"}}, telemetry.CapabilityChat},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, capabilityForModel(tc.model))
		})
	}
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
	cmd := m.openMetrics(&models.DedicatedAICluster{Name: "d"}) // no ModelName
	require.NotNil(t, cmd)
	assert.Equal(t, g0, m.gen, "no catalog load, so no generation bump")
}

func TestOpenDacMetrics_CatalogLoadedResolvesNow(t *testing.T) {
	t.Parallel()
	m := makeTestModel()
	m.dataset = &models.Dataset{BaseModels: []models.BaseModel{{Name: "r", Capabilities: []string{"TEXT_RERANK"}}}}
	g0 := m.gen
	cmd := m.openMetrics(&models.DedicatedAICluster{Name: "d", ModelName: "r"})
	require.NotNil(t, cmd, "launches immediately from the cached base catalog")
	assert.Equal(t, g0, m.gen, "no load dispatched when the catalog is present")
}

func TestOpenDacMetrics_NotLoadedDispatchesLoad(t *testing.T) {
	t.Parallel()
	m := makeTestModel() // dataset nil → base catalog not loaded
	g0 := m.gen
	cmd := m.openMetrics(&models.DedicatedAICluster{Name: "d", ModelName: "some-base-model"})
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
	cmd := m.openMetrics(&models.DedicatedAICluster{Name: "d", ModelName: "amaaaaaaxyz"})
	require.NotNil(t, cmd)
	assert.Equal(t, g0+1, m.gen, "imported catalog not loaded → load dispatched despite base being present")
}

func TestHandleOpenMetricsTrigger_StaleGenNoOpen(t *testing.T) {
	t.Parallel()
	m := makeTestModel()
	m.dataset = &models.Dataset{BaseModels: []models.BaseModel{{Name: "r", Capabilities: []string{"TEXT_RERANK"}}}}
	got := m.handleOpenMetricsTrigger(openMetricsTriggerMsg{
		item: &models.DedicatedAICluster{Name: "d", ModelName: "r"}, cat: domain.BaseModel, gen: m.gen + 1,
	})
	assert.Nil(t, got, "stale generation does not open")
}

func TestHandleOpenMetricsTrigger_CatalogNotLoadedNoOpen(t *testing.T) {
	t.Parallel()
	m := makeTestModel() // dataset nil → catalog load must have failed/dropped
	got := m.handleOpenMetricsTrigger(openMetricsTriggerMsg{
		item: &models.DedicatedAICluster{Name: "d", ModelName: "r"}, cat: domain.BaseModel, gen: m.gen,
	})
	assert.Nil(t, got, "no open when the catalog isn't loaded")
}

func TestHandleOpenMetricsTrigger_OpensWhenLoaded(t *testing.T) {
	t.Parallel()
	m := makeTestModel()
	m.dataset = &models.Dataset{BaseModels: []models.BaseModel{{Name: "r", Capabilities: []string{"TEXT_RERANK"}}}}
	got := m.handleOpenMetricsTrigger(openMetricsTriggerMsg{
		item: &models.DedicatedAICluster{Name: "d", ModelName: "r"}, cat: domain.BaseModel, gen: m.gen,
	})
	require.NotNil(t, got, "opens the dashboard once the catalog is applied")
}
