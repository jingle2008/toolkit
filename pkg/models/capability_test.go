package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBaseModel_HasCapability(t *testing.T) {
	t.Parallel()
	m := BaseModel{Capabilities: []string{"CHAT", "TEXT_RERANK"}}
	assert.True(t, m.HasCapability("CHAT"))
	assert.True(t, m.HasCapability("TEXT_RERANK"))
	assert.False(t, m.HasCapability("TEXT_EMBEDDINGS"))
	assert.False(t, m.HasCapability("chat"), "match is exact / case-sensitive")
	assert.False(t, BaseModel{}.HasCapability("CHAT"), "no capabilities")
}

func TestDataset_FindModelByName(t *testing.T) {
	t.Parallel()
	d := &Dataset{
		BaseModels: []BaseModel{{Name: "base-1", Capabilities: []string{"CHAT"}}},
		ImportedModelMap: map[string][]ImportedModel{
			"tenantA": {{BaseModel: BaseModel{Name: "imp-1", Capabilities: []string{"TEXT_RERANK"}}}},
		},
	}
	assert.Equal(t, "base-1", d.FindModelByName("base-1").Name, "found in base catalog")

	imp := d.FindModelByName("imp-1")
	assert.Equal(t, "imp-1", imp.Name, "found in imported models")
	assert.True(t, imp.HasCapability("TEXT_RERANK"))

	assert.Nil(t, d.FindModelByName("nope"), "unknown name")
	assert.Nil(t, d.FindModelByName(""), "empty name")
}

func TestCapabilityConstants(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "TEXT_CLASSIFICATION", CapabilityTextClassification)
	assert.Equal(t, "IMAGE_CONTENT_MODERATION", CapabilityImageContentModeration)
}

func TestExpandedCapabilityConstants(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "TEXT_TO_TEXT", CapabilityTextToText)
	assert.Equal(t, "IMAGE_TEXT_TO_TEXT", CapabilityImageTextToText)
	assert.Equal(t, "EMBEDDING", CapabilityEmbedding)
	assert.Equal(t, "TEXT_TO_IMAGE", CapabilityTextToImage)
	assert.Equal(t, "IMAGE_TEXT_TO_IMAGE", CapabilityImageTextToImage)
	assert.Equal(t, "TEXT_TO_AUDIO", CapabilityTextToAudio)
	assert.Equal(t, "AUDIO_TO_TEXT", CapabilityAudioToText)
	assert.Equal(t, "CONTENT_MODERATION", CapabilityContentModeration)
	assert.Equal(t, "TEXT_GENERATION", CapabilityTextGeneration)
	assert.Equal(t, "AUDIO_TO_AUDIO", CapabilityAudioToAudio)
	assert.Equal(t, "REALTIME", CapabilityRealtime)
	assert.Equal(t, "PROMPT_INJECTION_PROTECTION", CapabilityPromptInjectionProtection)
}
