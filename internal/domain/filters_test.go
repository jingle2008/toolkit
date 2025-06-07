package domain

import (
	"testing"

	"github.com/jingle2008/toolkit/pkg/models"
)

func TestGetEnvironments(t *testing.T) {
	t.Parallel()
	envs := []models.Environment{
		{Type: "k8s", Realm: "public", Region: "us-west"},
		{Type: "k8s", Realm: "private", Region: "us-east"},
		{Type: "baremetal", Realm: "public", Region: "eu-central"},
	}

	tests := []struct {
		name       string
		filter     string
		wantCount  int
		wantNames  []string
		wantRegion string
		wantRealm  string
	}{
		{
			name:      "No filter returns all",
			filter:    "",
			wantCount: 3,
		},
		{
			name:      "Filter by type",
			filter:    "baremetal",
			wantCount: 1,
			wantNames: []string{"baremetal-UNKNOWN"},
		},
		{
			name:       "Filter by region",
			filter:     "west",
			wantCount:  1,
			wantNames:  []string{"k8s-UNKNOWN"},
			wantRegion: "us-west",
		},
		{
			name:      "Filter by realm",
			filter:    "private",
			wantCount: 1,
			wantRealm: "private",
		},
		{
			name:      "No match",
			filter:    "doesnotexist",
			wantCount: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rows := FilterEnvironments(envs, tc.filter)
			if len(rows) != tc.wantCount {
				t.Errorf("filter %q: expected %d rows, got %d", tc.filter, tc.wantCount, len(rows))
			}
			if tc.wantNames != nil && len(rows) > 0 {
				for i, want := range tc.wantNames {
					if rows[i].GetName() != want {
						t.Errorf("filter %q: expected name %q, got %q", tc.filter, want, rows[i].GetName())
					}
				}
			}
			if tc.wantRegion != "" && len(rows) > 0 {
				if rows[0].Region != tc.wantRegion {
					t.Errorf("filter %q: expected region %q, got %q", tc.filter, tc.wantRegion, rows[0].Region)
				}
			}
			if tc.wantRealm != "" && len(rows) > 0 {
				if rows[0].Realm != tc.wantRealm {
					t.Errorf("filter %q: expected realm %q, got %q", tc.filter, tc.wantRealm, rows[0].Realm)
				}
			}
		})
	}
}

func TestGetServiceTenancies(t *testing.T) {
	t.Parallel()
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
			rows := FilterServiceTenancies(tenancies, tc.filter)
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

func TestGetTenants(t *testing.T) {
	t.Parallel()
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
			rows := FilterTenants(tenants, tc.filter)
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
