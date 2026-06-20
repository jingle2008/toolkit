package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/jingle2008/toolkit/internal/infra/telemetry"
	"github.com/jingle2008/toolkit/pkg/models"
)

func TestModelCapability(t *testing.T) {
	t.Parallel()
	m := makeTestModel()
	m.dataset = &models.Dataset{
		BaseModels: []models.BaseModel{
			{Name: "chat-model", Capabilities: []string{"CHAT"}},
			{Name: "rerank-model", Capabilities: []string{"TEXT_RERANK"}},
			{Name: "embed-model", Capabilities: []string{"TEXT_EMBEDDINGS"}},
			// Multiple capabilities → precedence CHAT > RERANK > EMBED.
			{Name: "multi-model", Capabilities: []string{"TEXT_EMBEDDINGS", "CHAT"}},
			// Finetune is chat-only regardless of declared capabilities.
			{Name: "ft-model", Type: "Fine-tuning", Capabilities: []string{"TEXT_RERANK"}},
			// Unrecognized capability falls back to chat.
			{Name: "mystery-model", Capabilities: []string{"SOMETHING_ELSE"}},
		},
	}
	cases := []struct {
		modelName string
		want      telemetry.Capability
	}{
		{"", telemetry.CapabilityChat},               // DAC has no model
		{"not-in-dataset", telemetry.CapabilityChat}, // unresolved
		{"chat-model", telemetry.CapabilityChat},
		{"rerank-model", telemetry.CapabilityTextRerank},
		{"embed-model", telemetry.CapabilityTextEmbeddings},
		{"multi-model", telemetry.CapabilityChat},
		{"ft-model", telemetry.CapabilityChat},
		{"mystery-model", telemetry.CapabilityChat},
	}
	for _, tc := range cases {
		t.Run(tc.modelName, func(t *testing.T) {
			t.Parallel()
			got := m.modelCapability(&models.DedicatedAICluster{ModelName: tc.modelName})
			assert.Equal(t, tc.want, got)
		})
	}
}
