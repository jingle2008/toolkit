package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLimitDefinition_Getters(t *testing.T) {
	t.Parallel()
	ld := LimitDefinition{
		Name:        "CPU",
		Description: "CPU limit",
		Type:        "resource",
		Scope:       "global",
		DefaultMin:  "1",
		DefaultMax:  "10",
		Service:     "compute",
		PublicName:  "CPU Public",
		IsStaged:    true,
		IsQuota:     false,
		UsageSource: "usage",
	}
	assert.Equal(t, "CPU", ld.GetName())
	assert.Equal(t, "CPU limit", ld.GetDescription())
	assert.ElementsMatch(t, []string{"CPU", "CPU limit"}, ld.GetFilterableFields())
}

func TestLimitTenancyOverride_Getters(t *testing.T) {
	t.Parallel()
	lto := LimitTenancyOverride{
		Realms:   []string{"realmA"},
		Name:     "CPU",
		Regions:  []string{"us-phoenix-1", "us-ashburn-1"},
		Group:    "group1",
		TenantID: "tenantX",
		Values:   []LimitRange{{Min: 2, Max: 8}},
	}
	assert.Equal(t, "CPU", lto.GetName())
	assert.Equal(t, "tenantX", lto.GetTenantID())
	assert.Contains(t, lto.GetFilterableFields(), "us-phoenix-1")
	assert.Contains(t, lto.GetFilterableFields(), "us-ashburn-1")
	assert.Contains(t, lto.GetFilterableFields(), "CPU")
}
