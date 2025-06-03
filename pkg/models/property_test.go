package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPropertyDefinition_Getters(t *testing.T) {
	t.Parallel()
	p := PropertyDefinition{
		Name:         "foo",
		Description:  "desc",
		Type:         "string",
		Options:      []string{"a", "b"},
		DefaultValue: "bar",
	}
	assert.Equal(t, "foo", p.GetName())
	assert.Equal(t, "desc", p.GetDescription())
	assert.Equal(t, "bar", p.GetValue())
	assert.ElementsMatch(t, []string{"foo", "desc"}, p.GetFilterableFields())
}

func TestPropertyRegionalOverride_Getters(t *testing.T) {
	t.Parallel()
	pro := PropertyRegionalOverride{
		Realms:  []string{"r1"},
		Name:    "p1",
		Regions: []string{"us-phoenix-1"},
		Group:   "g1",
		Values: []struct {
			Value string "json:\"value\""
		}{{Value: "v1"}},
	}
	assert.Equal(t, "p1", pro.GetName())
	assert.Equal(t, []string{"us-phoenix-1"}, pro.GetRegions())
	assert.Equal(t, "v1", pro.GetValue())
	assert.Contains(t, pro.GetFilterableFields(), "p1")
	assert.Contains(t, pro.GetFilterableFields(), "us-phoenix-1")
}

func TestPropertyTenancyOverride_GetTenantID(t *testing.T) {
	t.Parallel()
	pro := PropertyRegionalOverride{
		Realms:  []string{"r1"},
		Name:    "p1",
		Regions: []string{"us-phoenix-1"},
		Group:   "g1",
		Values: []struct {
			Value string "json:\"value\""
		}{{Value: "v1"}},
	}
	pto := PropertyTenancyOverride{
		Tag:                      "tenantX",
		PropertyRegionalOverride: pro,
	}
	assert.Equal(t, "tenantX", pto.GetTenantID())
}
