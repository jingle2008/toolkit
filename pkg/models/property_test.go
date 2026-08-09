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
	assert.ElementsMatch(t, []string{"foo", "desc"}, p.FilterableFields())
	assert.False(t, p.IsFaulty())
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
	assert.Contains(t, pro.FilterableFields(), "p1")
	assert.Contains(t, pro.FilterableFields(), "us-phoenix-1")
	assert.False(t, pro.IsFaulty())
}

func TestPropertyRegionalOverride_GetValue_NoValues(t *testing.T) {
	t.Parallel()
	pro := PropertyRegionalOverride{Name: "p1"}
	assert.Equal(t, "", pro.GetValue())
}

func TestPropertyTenancyOverride_SetTenantNameAndFilterableFields(t *testing.T) {
	t.Parallel()
	pto := PropertyTenancyOverride{
		TenantID: "tenantX",
		PropertyRegionalOverride: PropertyRegionalOverride{
			Name:    "p1",
			Regions: []string{"us-phoenix-1", "us-ashburn-1"},
		},
	}

	pto.SetTenantName("acme")
	assert.Equal(t, "acme", pto.TenantName)
	assert.ElementsMatch(t,
		[]string{"us-phoenix-1", "us-ashburn-1", "p1", "acme", "tenantX"},
		pto.FilterableFields())
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
		TenantID:                 "tenantX",
		PropertyRegionalOverride: pro,
	}
	assert.Equal(t, "tenantX", pto.GetTenantID())
}
