package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTenant_Getters(t *testing.T) {
	t.Parallel()
	tenant := Tenant{
		Name: "tenantA",
		IDs:  []string{"id1", "id2"},
	}
	assert.Equal(t, "tenantA", tenant.GetName())
	assert.Equal(t, "id1 (+1)", tenant.GetTenantID())
	fields := tenant.FilterableFields()
	assert.Contains(t, fields, "tenantA")
	assert.Contains(t, fields, "id1 (+1)")

	tenant2 := Tenant{
		Name: "tenantB",
		IDs:  []string{"id3"},
	}
	assert.Equal(t, "id3", tenant2.GetTenantID())

	tenant3 := Tenant{
		Name: "tenantC",
		IDs:  []string{},
	}
	assert.Equal(t, "", tenant3.GetTenantID())
}

func TestTenant_IsFaulty(t *testing.T) {
	t.Parallel()
	// Multiple IDs on one tenant is the fault condition.
	assert.False(t, Tenant{Name: "a"}.IsFaulty())
	assert.False(t, Tenant{Name: "a", IDs: []string{"id1"}}.IsFaulty())
	assert.True(t, Tenant{Name: "a", IDs: []string{"id1", "id2"}}.IsFaulty())
}
