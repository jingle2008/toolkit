package columns

import (
	"testing"

	"github.com/jingle2008/toolkit/pkg/models"
)

func TestImportedModelColumns(t *testing.T) {
	t.Parallel()
	m := models.ImportedModel{
		BaseModel: models.BaseModel{
			Name:        "llama3-8b",
			DisplayName: "Llama 3 8B",
			Vendor:      "Meta",
			Version:     "1.0",
			Status:      "Ready",
		},
		Namespace: "ns-prod",
	}

	got := map[string]string{}
	for _, c := range ImportedModelColumns.Columns {
		got[c.Key] = c.Render("tenant-xyz", m)
	}

	want := map[string]string{
		"name":         "llama3-8b",
		"tenant":       "tenant-xyz",
		"namespace":    "ns-prod",
		"display-name": "Llama 3 8B",
		"vendor":       "Meta",
		"version":      "1.0",
		"status":       "Ready",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("col %s: got %q, want %q", k, got[k], v)
		}
	}

	// Verify Default flags.
	defaults := map[string]bool{}
	for _, c := range ImportedModelColumns.Columns {
		defaults[c.Key] = c.Default
	}
	defaultTrue := []string{"name", "tenant", "namespace", "display-name", "status"}
	for _, k := range defaultTrue {
		if !defaults[k] {
			t.Errorf("col %s: expected Default=true", k)
		}
	}
	defaultFalse := []string{"vendor", "version"}
	for _, k := range defaultFalse {
		if defaults[k] {
			t.Errorf("col %s: expected Default=false", k)
		}
	}

	// Verify ratio sum is ~1.0.
	sum := ImportedModelColumns.RatioSum()
	if sum < 0.98 || sum > 1.02 {
		t.Errorf("ratio sum %.3f outside ±0.02 of 1.0", sum)
	}
}
