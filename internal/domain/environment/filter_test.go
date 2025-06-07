package environment

import (
	"testing"

	"github.com/jingle2008/toolkit/pkg/models"
)

func TestGetEnvironments(t *testing.T) {
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
			rows := Filter(envs, tc.filter)
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
