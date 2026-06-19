package columns

import (
	"testing"

	"github.com/jingle2008/toolkit/pkg/models"
)

func TestImportedModelColumns(t *testing.T) {
	t.Parallel()
	m := models.ImportedModel{
		BaseModel: models.BaseModel{
			Name:          "llama3-8b",
			DisplayName:   "Llama 3 8B",
			Vendor:        "Meta",
			Status:        "Ready",
			ParameterSize: "8B",
			MaxTokens:     8192,
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
		"size":         "8B",
		"context":      "8192",
		"vendor":       "Meta",
		"status":       "Ready",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("col %s: got %q, want %q", k, got[k], v)
		}
	}

	// Verify ratio sum is ~1.0.
	sum := ImportedModelColumns.RatioSum()
	if sum < 0.98 || sum > 1.02 {
		t.Errorf("ratio sum %.3f outside ±0.02 of 1.0", sum)
	}
}
