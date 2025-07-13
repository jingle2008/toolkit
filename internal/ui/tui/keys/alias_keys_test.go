package keys

import (
	"testing"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	"github.com/stretchr/testify/assert"
)

func TestResolveKeys_DisablesViewDetailsForAlias(t *testing.T) {
	t.Parallel()
	km := ResolveKeys(domain.Alias, common.ListView)
	for _, b := range km.Global {
		if b.Help().Desc == "Toggle Details" {
			assert.False(t, b.Enabled(), "ViewDetails should be disabled for Alias category")
		}
	}
}
