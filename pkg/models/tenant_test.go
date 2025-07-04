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
	fields := tenant.GetFilterableFields()
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
