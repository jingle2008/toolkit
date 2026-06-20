package tui

import (
	"context"
	"errors"
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

// capLoader is a fakeLoader whose model-catalog loads are configurable,
// so resolveCapability's lazy-load paths can be exercised.
type capLoader struct {
	fakeLoader
	base        []models.BaseModel
	baseErr     error
	imported    map[string][]models.ImportedModel
	importedErr error
}

func (f capLoader) LoadBaseModels(context.Context, string, models.Environment) ([]models.BaseModel, error) {
	return f.base, f.baseErr
}

func (f capLoader) LoadImportedModels(context.Context, string, models.Environment) (map[string][]models.ImportedModel, error) {
	return f.imported, f.importedErr
}

func TestResolveCapability(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env := models.Environment{}
	boom := errors.New("boom")
	rerankBase := []models.BaseModel{{Name: "r", Capabilities: []string{"TEXT_RERANK"}}}
	embedImported := map[string][]models.ImportedModel{
		"t": {{BaseModel: models.BaseModel{Name: "e", Capabilities: []string{"TEXT_EMBEDDINGS"}}}},
	}

	t.Run("empty model name → chat, no load", func(t *testing.T) {
		t.Parallel()
		// A loader that would error proves no load happened.
		got, err := resolveCapability(ctx, capLoader{baseErr: boom}, "", env, "", nil, nil)
		require.NoError(t, err)
		assert.Equal(t, telemetry.CapabilityChat, got)
	})

	t.Run("cached base hit → no load", func(t *testing.T) {
		t.Parallel()
		got, err := resolveCapability(ctx, capLoader{baseErr: boom}, "", env, "r", rerankBase, nil)
		require.NoError(t, err)
		assert.Equal(t, telemetry.CapabilityTextRerank, got)
	})

	t.Run("base not cached → loads base", func(t *testing.T) {
		t.Parallel()
		got, err := resolveCapability(ctx, capLoader{base: rerankBase}, "", env, "r", nil, nil)
		require.NoError(t, err)
		assert.Equal(t, telemetry.CapabilityTextRerank, got)
	})

	t.Run("base load fails → error", func(t *testing.T) {
		t.Parallel()
		_, err := resolveCapability(ctx, capLoader{baseErr: boom}, "", env, "r", nil, nil)
		require.Error(t, err)
	})

	t.Run("not in base → loads imported", func(t *testing.T) {
		t.Parallel()
		got, err := resolveCapability(ctx, capLoader{base: []models.BaseModel{}, imported: embedImported}, "", env, "e", nil, nil)
		require.NoError(t, err)
		assert.Equal(t, telemetry.CapabilityTextEmbeddings, got)
	})

	t.Run("imported load fails → error", func(t *testing.T) {
		t.Parallel()
		_, err := resolveCapability(ctx, capLoader{base: []models.BaseModel{}, importedErr: boom}, "", env, "ghost", nil, nil)
		require.Error(t, err)
	})

	t.Run("loaded cleanly but absent → chat", func(t *testing.T) {
		t.Parallel()
		got, err := resolveCapability(ctx, capLoader{base: []models.BaseModel{}, imported: map[string][]models.ImportedModel{}}, "", env, "ghost", nil, nil)
		require.NoError(t, err)
		assert.Equal(t, telemetry.CapabilityChat, got)
	})
}
