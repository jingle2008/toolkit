package rows

import (
	"testing"

	"github.com/jingle2008/toolkit/pkg/models"
)

func TestGetServiceTenancies(t *testing.T) {
	tenancies := []models.ServiceTenancy{
		{
			Name:        "svc1",
			Realm:       "public",
			Environment: "dev",
			HomeRegion:  "us-west",
			Regions:     []string{"us-west", "us-east"},
		},
		{
			Name:        "svc2",
			Realm:       "private",
			Environment: "prod",
			HomeRegion:  "eu-central",
			Regions:     []string{"eu-central"},
		},
	}

	// No filter: all tenancies returned
	rows := ServiceTenancies(tenancies, "")
	if len(rows) != 2 {
		t.Errorf("expected 2 rows, got %d", len(rows))
	}

	// Filter by name
	rows = ServiceTenancies(tenancies, "svc2")
	if len(rows) != 1 || rows[0].Name != "svc2" {
		t.Errorf("expected 1 row for svc2, got %v", rows)
	}

	// Filter by region
	rows = ServiceTenancies(tenancies, "west")
	if len(rows) != 1 || rows[0].Name != "svc1" {
		t.Errorf("expected 1 row for us-west, got %v", rows)
	}

	// Filter by environment
	rows = ServiceTenancies(tenancies, "prod")
	if len(rows) != 1 || rows[0].Environment != "prod" {
		t.Errorf("expected 1 row for prod, got %v", rows)
	}

	// Filter with no match
	rows = ServiceTenancies(tenancies, "doesnotexist")
	if len(rows) != 0 {
		t.Errorf("expected 0 rows for unmatched filter, got %v", rows)
	}
}
