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
