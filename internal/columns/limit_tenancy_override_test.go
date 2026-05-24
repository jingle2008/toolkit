package columns

import (
	"testing"

	"github.com/jingle2008/toolkit/pkg/models"
)

func TestLimitTenancyOverrideColumns(t *testing.T) {
	t.Parallel()
	v := models.LimitTenancyOverride{
		LimitRegionalOverride: models.LimitRegionalOverride{
			Name:    "compute-cores",
			Regions: []string{"us-ashburn-1", "us-phoenix-1"},
			Values:  []models.LimitRange{{Min: 5, Max: 100}},
		},
	}
	key := "tenant-1"

	got := map[string]string{}
	for _, c := range LimitTenancyOverrideColumns.Columns {
		got[c.Key] = c.Render(key, v)
	}

	want := map[string]string{
		"name":    "compute-cores",
		"tenant":  "tenant-1",
		"regions": "us-ashburn-1, us-phoenix-1",
		"min":     "5",
		"max":     "100",
	}
	for k, wv := range want {
		if got[k] != wv {
			t.Errorf("col %s: got %q, want %q", k, got[k], wv)
		}
	}

	// Verify Default flags: all columns are Default=true.
	defaults := map[string]bool{}
	for _, c := range LimitTenancyOverrideColumns.Columns {
		defaults[c.Key] = c.Default
	}
	for _, k := range []string{"name", "tenant", "regions", "min", "max"} {
		if !defaults[k] {
			t.Errorf("col %s: expected Default=true", k)
		}
	}
}
