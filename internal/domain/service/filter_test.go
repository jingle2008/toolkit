package service

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

	tests := []struct {
		name        string
		filter      string
		wantCount   int
		wantNames   []string
		wantEnv     string
		wantHomeReg string
	}{
		{
			name:      "No filter returns all",
			filter:    "",
			wantCount: 2,
		},
		{
			name:      "Filter by name",
			filter:    "svc2",
			wantCount: 1,
			wantNames: []string{"svc2"},
		},
		{
			name:        "Filter by region",
			filter:      "west",
			wantCount:   1,
			wantNames:   []string{"svc1"},
			wantHomeReg: "us-west",
		},
		{
			name:      "Filter by environment",
			filter:    "prod",
			wantCount: 1,
			wantEnv:   "prod",
		},
		{
			name:      "No match",
			filter:    "doesnotexist",
			wantCount: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rows := Filter(tenancies, tc.filter)
			if len(rows) != tc.wantCount {
				t.Errorf("filter %q: expected %d rows, got %d", tc.filter, tc.wantCount, len(rows))
			}
			if tc.wantNames != nil && len(rows) > 0 {
				for i, want := range tc.wantNames {
					if rows[i].Name != want {
						t.Errorf("filter %q: expected name %q, got %q", tc.filter, want, rows[i].Name)
					}
				}
			}
			if tc.wantEnv != "" && len(rows) > 0 {
				if rows[0].Environment != tc.wantEnv {
					t.Errorf("filter %q: expected environment %q, got %q", tc.filter, tc.wantEnv, rows[0].Environment)
				}
			}
			if tc.wantHomeReg != "" && len(rows) > 0 {
				if rows[0].HomeRegion != tc.wantHomeReg {
					t.Errorf("filter %q: expected home region %q, got %q", tc.filter, tc.wantHomeReg, rows[0].HomeRegion)
				}
			}
		})
	}
}
