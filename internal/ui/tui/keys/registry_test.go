package keys

import (
	"testing"

	"github.com/charmbracelet/bubbles/key"
	"github.com/stretchr/testify/assert"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
)

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

func TestResolveKeys_GlobalAndContext(t *testing.T) {
	t.Parallel()
	km := ResolveKeys(domain.BaseModel, common.ListView)
	assert.NotNil(t, km.Global)
	assert.NotEmpty(t, km.Mode)
	assert.NotEmpty(t, km.Context)

	km2 := ResolveKeys(domain.BaseModel, common.DetailsView)
	assert.NotNil(t, km2.Global)
	assert.NotEmpty(t, km2.Mode)
	assert.Empty(t, km2.Context)

	km3 := ResolveKeys(domain.Tenant, common.ListView)
	assert.NotNil(t, km3.Global)
	assert.NotEmpty(t, km3.Context)
}
