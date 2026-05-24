package columns

import (
	"testing"

	"github.com/jingle2008/toolkit/pkg/models"
)

func TestConsolePropertyRegionalOverrideColumns(t *testing.T) {
	t.Parallel()
	o := models.ConsolePropertyRegionalOverride{
		Name:    "dark-mode",
		Regions: []string{"us-ashburn-1", "us-phoenix-1"},
		Values: []struct {
			Value string `json:"value"`
		}{{Value: "true"}},
	}
	got := map[string]string{}
	for _, c := range ConsolePropertyRegionalOverrideColumns.Columns {
		got[c.Key] = c.Render(o)
	}

	want := map[string]string{
		"name":    "dark-mode",
		"regions": "us-ashburn-1,us-phoenix-1",
		"value":   "true",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("col %s: got %q, want %q", k, got[k], v)
		}
	}
}

func TestPropertyRegionalOverrideColumns(t *testing.T) {
	t.Parallel()
	o := models.PropertyRegionalOverride{
		Name:    "timeout",
		Regions: []string{"us-ashburn-1", "us-phoenix-1"},
		Values: []struct {
			Value string `json:"value"`
		}{{Value: "30s"}},
	}
	got := map[string]string{}
	for _, c := range PropertyRegionalOverrideColumns.Columns {
		got[c.Key] = c.Render(o)
	}

	want := map[string]string{
		"name":    "timeout",
		"regions": "us-ashburn-1,us-phoenix-1",
		"value":   "30s",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("col %s: got %q, want %q", k, got[k], v)
		}
	}
}
