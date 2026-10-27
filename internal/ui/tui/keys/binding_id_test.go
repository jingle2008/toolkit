package keys

import (
	"testing"

	"charm.land/bubbles/v2/key"
	"github.com/stretchr/testify/assert"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
)

func TestSortColumnOf(t *testing.T) {
	t.Parallel()
	col, ok := SortColumnOf(SortTenant)
	assert.True(t, ok)
	assert.Equal(t, common.TenantCol, col)

	_, ok = SortColumnOf(Refresh)
	assert.False(t, ok, "an action is no sort declaration")
}

// A keymap's copy of a declaration is the same binding; another binding
// on the same key is not.
func TestSame(t *testing.T) {
	t.Parallel()
	km := ResolveKeys(domain.GPUNode, common.ListView)
	var refresh key.Binding
	for _, b := range km.Context {
		if Same(b, Refresh) {
			refresh = b
		}
	}
	assert.True(t, ListLevel(refresh), "the keymap's Refresh is list-level")
	assert.False(t, Same(NextCategory, ToggleJSON), "both on tab, different actions")
	assert.False(t, Same(SortTenant, SortAge))
}
