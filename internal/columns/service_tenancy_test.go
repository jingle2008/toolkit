package columns

import (
	"testing"

	"github.com/jingle2008/toolkit/pkg/models"
)

func TestServiceTenancyColumns(t *testing.T) {
	t.Parallel()
	s := models.ServiceTenancy{
		Name:        "svc-alpha",
		Realm:       "oc1",
		Environment: "preprod",
		HomeRegion:  "us-ashburn-1",
		Regions:     []string{"us-ashburn-1", "us-phoenix-1"},
	}
	got := map[string]string{}
	for _, c := range ServiceTenancyColumns.Columns {
		got[c.Key] = c.Render(s)
	}

	want := map[string]string{
		"name":        "svc-alpha",
		"realm":       "oc1",
		"environment": "preprod",
		"home-region": "us-ashburn-1",
		"regions":     "us-ashburn-1, us-phoenix-1",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("col %s: got %q, want %q", k, got[k], v)
		}
	}
}
