package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDedicatedAICluster_Getters(t *testing.T) {
	t.Parallel()
	cluster := DedicatedAICluster{
		Name:      "cluster1",
		Type:      "A100",
		UnitShape: "shapeA",
		Status:    "Ready",
		TenantID:  "tenant1",
	}
	assert.Equal(t, "cluster1", cluster.GetName())
	assert.ElementsMatch(t, []string{"cluster1", "A100", "shapeA", "Ready", "tenant1", "", "", "", "", ""}, cluster.GetFilterableFields())
}

func TestDedicatedAICluster_GetIDAndTenantID(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		realm     string
		region    string
		dacName   string
		tenantID  string
		expID     string
		expTenant string
	}{
		{
			name:      "IAD short code",
			realm:     "oc1",
			region:    string(RegionIAD),
			dacName:   "mydac",
			tenantID:  "t123",
			expID:     "ocid1.generativeaidedicatedaicluster.oc1.iad.mydac",
			expTenant: "ocid1.tenancy.oc1..t123",
		},
		{
			name:      "PHX short code",
			realm:     "oc1",
			region:    string(RegionPHX),
			dacName:   "mydac",
			tenantID:  "t123",
			expID:     "ocid1.generativeaidedicatedaicluster.oc1.phx.mydac",
			expTenant: "ocid1.tenancy.oc1..t123",
		},
		{
			name:      "Other region passthrough",
			realm:     "oc1",
			region:    "eu-frankfurt-1",
			dacName:   "mydac",
			tenantID:  "t123",
			expID:     "ocid1.generativeaidedicatedaicluster.oc1.eu-frankfurt-1.mydac",
			expTenant: "ocid1.tenancy.oc1..t123",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			dac := DedicatedAICluster{Name: c.dacName, TenantID: c.tenantID}
			gotID := dac.GetID(c.realm, c.region)
			if gotID != c.expID {
				t.Errorf("GetID() = %q, want %q", gotID, c.expID)
			}
			gotTenant := dac.GetTenantID(c.realm)
			if gotTenant != c.expTenant {
				t.Errorf("GetTenantID() = %q, want %q", gotTenant, c.expTenant)
			}
		})
	}
}
