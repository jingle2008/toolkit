package columns

import (
	"testing"

	"github.com/jingle2008/toolkit/pkg/models"
)

func TestLimitDefinitionColumns(t *testing.T) {
	t.Parallel()
	d := models.LimitDefinition{
		Name:        "compute-cores",
		Description: "Number of compute cores",
		Scope:       "AD",
		DefaultMin:  "0",
		DefaultMax:  "100",
	}
	got := map[string]string{}
	for _, c := range LimitDefinitionColumns.Columns {
		got[c.Key] = c.Render(d)
	}

	want := map[string]string{
		"name":        "compute-cores",
		"description": "Number of compute cores",
		"scope":       "AD",
		"min":         "0",
		"max":         "100",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("col %s: got %q, want %q", k, got[k], v)
		}
	}
}
