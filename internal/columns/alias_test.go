package columns

import (
	"strings"
	"testing"

	"github.com/jingle2008/toolkit/internal/domain"
)

func TestAliasColumns(t *testing.T) {
	t.Parallel()
	cat := domain.Tenant
	got := map[string]string{}
	for _, c := range AliasColumns.Columns {
		got[c.Key] = c.Render(cat)
	}

	if got["name"] != "Tenant" {
		t.Errorf("col name: got %q, want %q", got["name"], "Tenant")
	}
	if !strings.Contains(got["aliases"], "tenant") {
		t.Errorf("col aliases: got %q, expected to contain %q", got["aliases"], "tenant")
	}
	if !strings.Contains(got["aliases"], "t") {
		t.Errorf("col aliases: got %q, expected to contain %q", got["aliases"], "t")
	}
}
