package keys

import (
	"testing"

	"github.com/charmbracelet/bubbles/key"
	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
)

func TestFullKeyMap(t *testing.T) {
	t.Parallel()
	km := FullKeyMap()
	if len(km.Global) == 0 || len(km.Mode) == 0 {
		t.Error("FullKeyMap() missing keys")
	}
}

func TestResolveKeys(t *testing.T) {
	t.Parallel()
	km := ResolveKeys(domain.Tenant, common.ListView)
	if len(km.Global) == 0 {
		t.Error("ResolveKeys() missing global keys")
	}
	if len(km.Mode) == 0 {
		t.Error("ResolveKeys() missing mode keys")
	}
	if len(km.Context) == 0 {
		t.Error("ResolveKeys() missing context keys for Tenant/ListView")
	}
}

func TestKeyBindingHelp(t *testing.T) {
	t.Parallel()
	b := key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "desc"))
	if b.Help().Key != "x" || b.Help().Desc != "desc" {
		t.Error("key.Binding Help() did not return expected values")
	}
}
