package columns

import (
	"testing"

	"github.com/jingle2008/toolkit/pkg/models"
)

func TestEnvironmentColumns(t *testing.T) {
	t.Parallel()
	e := models.Environment{Type: "preprod", Region: "us-ashburn-1", Realm: "oc1"}
	got := map[string]string{}
	for _, c := range EnvironmentColumns.Columns {
		got[c.Key] = c.Render(e)
	}

	if got["name"] != e.GetName() {
		t.Errorf("col name: got %q, want %q", got["name"], e.GetName())
	}
	if got["realm"] != "oc1" {
		t.Errorf("col realm: got %q, want %q", got["realm"], "oc1")
	}
	if got["type"] != "preprod" {
		t.Errorf("col type: got %q, want %q", got["type"], "preprod")
	}
	if got["region"] != "us-ashburn-1" {
		t.Errorf("col region: got %q, want %q", got["region"], "us-ashburn-1")
	}
}
