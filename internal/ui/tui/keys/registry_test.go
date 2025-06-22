package keys

import (
	"testing"

	"github.com/charmbracelet/bubbles/key"
	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
)

func TestGlobalKeys(t *testing.T) {
	keys := GlobalKeys()
	if len(keys) == 0 {
		t.Error("GlobalKeys() returned empty slice")
	}
}

func TestListModeKeys(t *testing.T) {
	keys := ListModeKeys()
	if len(keys) == 0 {
		t.Error("ListModeKeys() returned empty slice")
	}
}

func TestDetailsModeKeys(t *testing.T) {
	keys := DetailsModeKeys()
	if len(keys) == 0 {
		t.Error("DetailsModeKeys() returned empty slice")
	}
}

func TestCatContext(t *testing.T) {
	ctx := CatContext()
	if len(ctx) == 0 {
		t.Error("CatContext() returned empty map")
	}
}

func TestFullKeyMap(t *testing.T) {
	km := FullKeyMap()
	if len(km.Global) == 0 || len(km.Mode) == 0 {
		t.Error("FullKeyMap() missing keys")
	}
}

func TestResolveKeys(t *testing.T) {
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
	b := key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "desc"))
	if b.Help().Key != "x" || b.Help().Desc != "desc" {
		t.Error("key.Binding Help() did not return expected values")
	}
}
