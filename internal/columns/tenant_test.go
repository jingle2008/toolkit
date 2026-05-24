package columns

import (
	"testing"

	"github.com/jingle2008/toolkit/pkg/models"
)

func TestTenantColumns(t *testing.T) {
	t.Parallel()
	tt := models.Tenant{
		Name:       "alpha",
		IDs:        []string{"ocid1.tenancy.oc1..a"},
		IsInternal: true,
		Note:       "n/a",
	}
	got := map[string]string{}
	for _, c := range TenantColumns.Columns {
		got[c.Key] = c.Render(tt)
	}
	want := map[string]string{
		"name":     "alpha",
		"ids":      "ocid1.tenancy.oc1..a",
		"internal": "true",
		"note":     "n/a",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("col %s: got %q, want %q", k, got[k], v)
		}
	}
}
