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
	rows := Tenants(tenants, "")
	if len(rows) != 2 {
		t.Errorf("expected 2 rows, got %d", len(rows))
	}

	// Filter by name
	rows = Tenants(tenants, "tenant2")
	if len(rows) != 1 || rows[0].Name != "tenant2" {
		t.Errorf("expected 1 row for tenant2, got %v", rows)
	}

	// Filter by tenant ID
	rows = Tenants(tenants, "id1")
	if len(rows) != 1 || len(rows[0].IDs) == 0 || rows[0].IDs[0] != "id1" {
		t.Errorf("expected 1 row for id1, got %v", rows)
	}

	// Filter with no match
	rows = Tenants(tenants, "doesnotexist")
	if len(rows) != 0 {
		t.Errorf("expected 0 rows for unmatched filter, got %v", rows)
	}
}
