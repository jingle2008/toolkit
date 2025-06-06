package rows

import (
	"testing"

	"github.com/jingle2008/toolkit/pkg/models"
)

func TestGetTenants(t *testing.T) {
	tenants := []models.Tenant{
		{
			Name:                     "tenant1",
			IDs:                      []string{"id1"},
			LimitOverrides:           1,
			ConsolePropertyOverrides: 2,
			PropertyOverrides:        3,
		},
		{
			Name:                     "tenant2",
			IDs:                      []string{"id2"},
			LimitOverrides:           4,
			ConsolePropertyOverrides: 5,
			PropertyOverrides:        6,
		},
	}

	// No filter: all tenants returned
	rows := GetTenants(tenants, "")
	if len(rows) != 2 {
		t.Errorf("expected 2 rows, got %d", len(rows))
	}

	// Filter by name
	rows = GetTenants(tenants, "tenant2")
	if len(rows) != 1 || rows[0][0] != "tenant2" {
		t.Errorf("expected 1 row for tenant2, got %v", rows)
	}

	// Filter by tenant ID
	rows = GetTenants(tenants, "id1")
	if len(rows) != 1 || rows[0][1] != "id1" {
		t.Errorf("expected 1 row for id1, got %v", rows)
	}

	// Filter with no match
	rows = GetTenants(tenants, "doesnotexist")
	if len(rows) != 0 {
		t.Errorf("expected 0 rows for unmatched filter, got %v", rows)
	}
}
