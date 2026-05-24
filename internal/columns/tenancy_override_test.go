package columns

import (
	"testing"

	"github.com/jingle2008/toolkit/pkg/models"
)

func TestConsolePropertyTenancyOverrideColumns(t *testing.T) {
	t.Parallel()
	o := models.ConsolePropertyTenancyOverride{
		ConsolePropertyRegionalOverride: models.ConsolePropertyRegionalOverride{
			Name:    "dark-mode",
			Regions: []string{"us-ashburn-1", "us-phoenix-1"},
			Values: []struct {
				Value string `json:"value"`
			}{{Value: "true"}},
		},
	}
	key := "tenant-1"

	got := map[string]string{}
	for _, c := range ConsolePropertyTenancyOverrideColumns.Columns {
		got[c.Key] = c.Render(key, o)
	}

	want := map[string]string{
		"name":    "dark-mode",
		"tenant":  "tenant-1",
		"regions": "us-ashburn-1, us-phoenix-1",
		"value":   "true",
	}
	for k, wv := range want {
		if got[k] != wv {
			t.Errorf("col %s: got %q, want %q", k, got[k], wv)
		}
	}
}

func TestPropertyTenancyOverrideColumns(t *testing.T) {
	t.Parallel()
	o := models.PropertyTenancyOverride{
		PropertyRegionalOverride: models.PropertyRegionalOverride{
			Name:    "timeout",
			Regions: []string{"us-ashburn-1", "us-phoenix-1"},
			Values: []struct {
				Value string `json:"value"`
			}{{Value: "30s"}},
		},
	}
	key := "tenant-2"

	got := map[string]string{}
	for _, c := range PropertyTenancyOverrideColumns.Columns {
		got[c.Key] = c.Render(key, o)
	}

	want := map[string]string{
		"name":    "timeout",
		"tenant":  "tenant-2",
		"regions": "us-ashburn-1, us-phoenix-1",
		"value":   "30s",
	}
	for k, wv := range want {
		if got[k] != wv {
			t.Errorf("col %s: got %q, want %q", k, got[k], wv)
		}
	}
}
