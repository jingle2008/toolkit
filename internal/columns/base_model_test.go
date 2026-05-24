package columns

import (
	"testing"

	"github.com/jingle2008/toolkit/pkg/models"
)

func TestBaseModelColumns(t *testing.T) {
	t.Parallel()
	m := models.BaseModel{
		Name:          "cohere.command",
		DisplayName:   "Command",
		InternalName:  "cohere-command-internal",
		Vendor:        "Cohere",
		Type:          "CHAT",
		Version:       "v1",
		MaxTokens:     4096,
		ParameterSize: "7B",
		Status:        "Ready",
		IsExperimental: true,
		DacShapeConfigs: &models.DacShapeConfigs{
			CompatibleDACShapes: []models.DACShape{
				{Name: "BM.GPU.A10.4", QuotaUnit: 2, Default: true},
			},
		},
	}

	got := map[string]string{}
	for _, c := range BaseModelColumns.Columns {
		got[c.Key] = c.Render(m)
	}

	want := map[string]string{
		"name":         "cohere.command",
		"display-name": "Command",
		"internal":     "cohere-command-internal",
		"vendor":       "Cohere",
		"type":         "CHAT",
		"version":      "v1",
		"dac-shape":    "2x BM.GPU.A10.4",
		"size":         "7B",
		"context":      "4096",
		"flags":        "EXP",
		"status":       "Ready",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("col %s: got %q, want %q", k, got[k], v)
		}
	}

	// Verify Default flags.
	defaults := map[string]bool{}
	for _, c := range BaseModelColumns.Columns {
		defaults[c.Key] = c.Default
	}
	defaultTrue := []string{"name", "internal", "vendor", "type", "version", "flags", "status"}
	for _, k := range defaultTrue {
		if !defaults[k] {
			t.Errorf("col %s: expected Default=true", k)
		}
	}
	defaultFalse := []string{"display-name", "dac-shape", "size", "context"}
	for _, k := range defaultFalse {
		if defaults[k] {
			t.Errorf("col %s: expected Default=false", k)
		}
	}

	// Verify ratio sum is ~1.0.
	sum := BaseModelColumns.RatioSum()
	if sum < 0.98 || sum > 1.02 {
		t.Errorf("ratio sum %.3f outside ±0.02 of 1.0", sum)
	}

	// Verify nil DAC shape renders as empty.
	noShape := models.BaseModel{Name: "x"}
	gotNoShape := map[string]string{}
	for _, c := range BaseModelColumns.Columns {
		gotNoShape[c.Key] = c.Render(noShape)
	}
	if gotNoShape["dac-shape"] != "" {
		t.Errorf("dac-shape with nil config: got %q, want empty", gotNoShape["dac-shape"])
	}
}
