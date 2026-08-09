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
	assert.ElementsMatch(t, []string{"cluster1", "A100", "shapeA", "Ready", "tenant1", "", "", "", "", ""}, cluster.FilterableFields())
}

func TestDedicatedAICluster_OwnerState(t *testing.T) {
	t.Parallel()
	var noOwner DedicatedAICluster
	assert.Equal(t, "", noOwner.OwnerState())

	internal := DedicatedAICluster{Owner: &Tenant{IsInternal: true}}
	assert.Equal(t, "true", internal.OwnerState())

	external := DedicatedAICluster{Owner: &Tenant{IsInternal: false}}
	assert.Equal(t, "false", external.OwnerState())
}

func TestDedicatedAICluster_Usage(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		total int
		idle  int
		want  string
	}{
		{name: "no replicas", total: 0, idle: 0, want: ""},
		{name: "negative total", total: -1, idle: 0, want: ""},
		{name: "fully idle", total: 4, idle: 4, want: "0%"},
		{name: "fully used", total: 4, idle: 0, want: "100%"},
		{name: "half used", total: 4, idle: 2, want: "50%"},
		{name: "rounded", total: 3, idle: 1, want: "67%"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			dac := DedicatedAICluster{TotalReplicas: c.total, IdleReplicas: c.idle}
			assert.Equal(t, c.want, dac.Usage())
		})
	}
}

func TestDedicatedAICluster_IsFaulty(t *testing.T) {
	t.Parallel()
	cases := []struct {
		status string
		want   bool
	}{
		{status: "", want: false},
		{status: "Ready", want: false},
		{status: "active", want: false},
		{status: "fail", want: true},
		{status: "failed", want: true},
		{status: "FAILED", want: true},
		{status: "Fail", want: true},
		{status: "failing", want: false},
	}

	for _, c := range cases {
		t.Run(c.status, func(t *testing.T) {
			t.Parallel()
			dac := DedicatedAICluster{Status: c.status}
			assert.Equal(t, c.want, dac.IsFaulty())
		})
	}
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
			gotID := dac.OCID(c.realm, c.region)
			if gotID != c.expID {
				t.Errorf("GetID() = %q, want %q", gotID, c.expID)
			}
			gotTenant := dac.TenancyOCID(c.realm)
			if gotTenant != c.expTenant {
				t.Errorf("GetTenantID() = %q, want %q", gotTenant, c.expTenant)
			}
		})
	}
}
