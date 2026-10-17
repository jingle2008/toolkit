package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTenant_Getters(t *testing.T) {
	t.Parallel()
	tenant := Tenant{Name: "tenantA", ID: "id1", Note: "n"}
	assert.Equal(t, "tenantA", tenant.GetName())
	assert.Equal(t, "id1", tenant.GetTenantID())
	assert.Subset(t, tenant.FilterableFields(), []string{"tenantA", "id1", "n"})
	assert.False(t, tenant.IsFaulty())
}

// A tenant without a metadata name is named by its OCID.
func TestTenant_Named(t *testing.T) {
	t.Parallel()
	assert.True(t, Tenant{Name: "Acme", ID: "ocid1.tenancy.oc1..a"}.Named())
	assert.False(t, Tenant{Name: "ocid1.tenancy.oc1..a", ID: "ocid1.tenancy.oc1..a"}.Named())
}

// A tenancy being deleted or terminated is faulty; any other or an
// unknown status isn't.
func TestTenant_IsFaultyByStatus(t *testing.T) {
	t.Parallel()
	for status, faulty := range map[string]bool{
		"": false, "ACTIVE": false, "ALWAYS_FREE_ONLY": false,
		TenantStatusDeleting: true, TenantStatusTerminated: true,
	} {
		assert.Equalf(t, faulty, Tenant{Status: status}.IsFaulty(), "status %q", status)
	}
	assert.Contains(t, Tenant{Status: "DELETING"}.FilterableFields(), "DELETING")
}
