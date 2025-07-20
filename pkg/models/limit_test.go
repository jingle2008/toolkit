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
		LimitRegionalOverride: LimitRegionalOverride{
			Realms:  []string{"realmA"},
			Name:    "CPU",
			Regions: []string{"us-phoenix-1", "us-ashburn-1"},
			Group:   "group1",
			Values:  []LimitRange{{Min: 2, Max: 8}},
		},
		TenantID: "tenantX",
	}
	assert.Equal(t, "CPU", lto.GetName())
	assert.Equal(t, "tenantX", lto.GetTenantID())
	assert.Contains(t, lto.GetFilterableFields(), "us-phoenix-1")
	assert.Contains(t, lto.GetFilterableFields(), "us-ashburn-1")
	assert.Contains(t, lto.GetFilterableFields(), "CPU")
}

func TestLimitRegionalOverride_FilterableFields_And_IsFaulty(t *testing.T) {
	t.Parallel()
	lro := LimitRegionalOverride{
		Regions: []string{"us-ashburn-1"},
		Name:    "gpuCount",
	}
	fields := lro.GetFilterableFields()
	assert.Contains(t, fields, "us-ashburn-1")
	assert.Contains(t, fields, "gpuCount")
	assert.False(t, lro.IsFaulty())

	lto := LimitTenancyOverride{
		LimitRegionalOverride: lro,
		TenantID:              "ocid1.tenancy.oc1..aaaa",
	}
	fields2 := lto.GetFilterableFields()
	assert.Contains(t, fields2, "ocid1.tenancy.oc1..aaaa")
	assert.False(t, lto.IsFaulty())

	ld := LimitDefinition{Name: "foo", Description: "bar"}
	assert.ElementsMatch(t, []string{"foo", "bar"}, ld.GetFilterableFields())
	assert.False(t, ld.IsFaulty())
}
