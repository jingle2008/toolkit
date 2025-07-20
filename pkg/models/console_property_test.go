package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConsolePropertyDefinition_Getters(t *testing.T) {
	t.Parallel()
	cpd := ConsolePropertyDefinition{
		Name:        "cpd1",
		Description: "desc1",
		Value:       "val1",
	}
	assert.Equal(t, "cpd1", cpd.GetName())
	assert.Equal(t, "desc1", cpd.GetDescription())
	assert.Equal(t, "val1", cpd.GetValue())
	assert.ElementsMatch(t, []string{"cpd1", "desc1"}, cpd.GetFilterableFields())
}

func TestConsolePropertyRegionalOverride_Getters(t *testing.T) {
	t.Parallel()
	cpro := ConsolePropertyRegionalOverride{
		Name:    "cpro1",
		Regions: []string{"us-phoenix-1", "us-ashburn-1"},
		Values: []struct {
			Value string `json:"value"`
		}{
			{Value: "v1"},
		},
	}
	assert.Equal(t, "cpro1", cpro.GetName())
	assert.ElementsMatch(t, []string{"us-phoenix-1", "us-ashburn-1"}, cpro.GetRegions())
	assert.Equal(t, "v1", cpro.GetValue())
	assert.Contains(t, cpro.GetFilterableFields(), "us-phoenix-1")
	assert.Contains(t, cpro.GetFilterableFields(), "us-ashburn-1")
}

func TestConsolePropertyTenancyOverride_GetTenantID(t *testing.T) {
	t.Parallel()
	cpto := ConsolePropertyTenancyOverride{
		TenantID: "tenantX",
	}
	assert.Equal(t, "tenantX", cpto.GetTenantID())
}

func TestConsoleProperty_Overrides_FilterableFields_And_IsFaulty(t *testing.T) {
	t.Parallel()
	cpro := ConsolePropertyRegionalOverride{
		Name:    "cpro2",
		Regions: []string{"us-ashburn-1"},
		Values: []struct {
			Value string `json:"value"`
		}{
			{Value: "v2"},
		},
	}
	fields := cpro.GetFilterableFields()
	assert.Contains(t, fields, "us-ashburn-1")
	assert.Contains(t, fields, "cpro2")
	assert.Equal(t, "v2", cpro.GetValue())
	assert.False(t, cpro.IsFaulty())

	cpto := ConsolePropertyTenancyOverride{
		TenantID:                        "tenantY",
		ConsolePropertyRegionalOverride: cpro,
	}
	fields2 := cpto.GetFilterableFields()
	assert.Contains(t, fields2, "tenantY")
	assert.False(t, cpto.IsFaulty())
}
