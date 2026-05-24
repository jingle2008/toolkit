package columns

import (
	"testing"

	"github.com/jingle2008/toolkit/pkg/models"
)

func TestConsolePropertyDefinitionColumns(t *testing.T) {
	t.Parallel()
	d := models.ConsolePropertyDefinition{
		Name:        "my-property",
		Description: "A console property",
		Value:       "enabled",
	}
	got := map[string]string{}
	for _, c := range ConsolePropertyDefinitionColumns.Columns {
		got[c.Key] = c.Render(d)
	}

	want := map[string]string{
		"name":        "my-property",
		"description": "A console property",
		"value":       "enabled",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("col %s: got %q, want %q", k, got[k], v)
		}
	}
}

func TestPropertyDefinitionColumns(t *testing.T) {
	t.Parallel()
	d := models.PropertyDefinition{
		Name:         "feature-flag",
		Description:  "Enables the new feature",
		DefaultValue: "false",
	}
	got := map[string]string{}
	for _, c := range PropertyDefinitionColumns.Columns {
		got[c.Key] = c.Render(d)
	}

	want := map[string]string{
		"name":        "feature-flag",
		"description": "Enables the new feature",
		"value":       "false",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("col %s: got %q, want %q", k, got[k], v)
		}
	}
}
