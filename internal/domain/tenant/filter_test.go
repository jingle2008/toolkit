package tenant

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

	tests := []struct {
		name      string
		filter    string
		wantCount int
		wantName  string
		wantID    string
	}{
		{
			name:      "No filter returns all",
			filter:    "",
			wantCount: 2,
		},
		{
			name:      "Filter by name",
			filter:    "tenant2",
			wantCount: 1,
			wantName:  "tenant2",
		},
		{
			name:      "Filter by tenant ID",
			filter:    "id1",
			wantCount: 1,
			wantID:    "id1",
		},
		{
			name:      "No match",
			filter:    "doesnotexist",
			wantCount: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rows := Filter(tenants, tc.filter)
			if len(rows) != tc.wantCount {
				t.Errorf("filter %q: expected %d rows, got %d", tc.filter, tc.wantCount, len(rows))
			}
			if tc.wantName != "" && len(rows) > 0 {
				if rows[0].Name != tc.wantName {
					t.Errorf("filter %q: expected name %q, got %q", tc.filter, tc.wantName, rows[0].Name)
				}
			}
			if tc.wantID != "" && len(rows) > 0 {
				found := false
				for _, id := range rows[0].IDs {
					if id == tc.wantID {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("filter %q: expected ID %q in %+v", tc.filter, tc.wantID, rows[0].IDs)
				}
			}
		})
	}
}
