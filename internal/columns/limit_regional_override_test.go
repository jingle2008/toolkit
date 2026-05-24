package columns

import (
	"testing"

	"github.com/jingle2008/toolkit/pkg/models"
)

func TestLimitRegionalOverrideColumns(t *testing.T) {
	t.Parallel()
	o := models.LimitRegionalOverride{
		Name:    "compute-cores",
		Regions: []string{"us-ashburn-1", "us-phoenix-1"},
		Values:  []models.LimitRange{{Min: 0, Max: 50}},
	}
	got := map[string]string{}
	for _, c := range LimitRegionalOverrideColumns.Columns {
		got[c.Key] = c.Render(o)
	}

	want := map[string]string{
		"name":    "compute-cores",
		"regions": "us-ashburn-1, us-phoenix-1",
		"min":     "0",
		"max":     "50",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("col %s: got %q, want %q", k, got[k], v)
		}
	}

	// Verify Default flags: name and regions are true; min and max are false.
	defaults := map[string]bool{}
	for _, c := range LimitRegionalOverrideColumns.Columns {
		defaults[c.Key] = c.Default
	}
	if !defaults["name"] {
		t.Error("col name: expected Default=true")
	}
	if !defaults["regions"] {
		t.Error("col regions: expected Default=true")
	}
	if defaults["min"] {
		t.Error("col min: expected Default=false")
	}
	if defaults["max"] {
		t.Error("col max: expected Default=false")
	}

	// Verify empty Values doesn't panic.
	empty := models.LimitRegionalOverride{Name: "x", Regions: []string{}}
	if limitOverrideMin(empty.Values) != "" {
		t.Error("limitOverrideMin with empty values: expected empty string")
	}
	if limitOverrideMax(empty.Values) != "" {
		t.Error("limitOverrideMax with empty values: expected empty string")
	}
}
