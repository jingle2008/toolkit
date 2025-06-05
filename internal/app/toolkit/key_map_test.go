package toolkit

import (
	"testing"

	"github.com/charmbracelet/bubbles/key"
	"github.com/jingle2008/toolkit/internal/app/domain"
	"github.com/stretchr/testify/assert"
)

func TestKeyMap_ShortHelp(t *testing.T) {
	t.Parallel()
	km := keyMap{
		Help: key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Quit: key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
	}
	short := km.ShortHelp()
	assert.NotEmpty(t, short)
	assert.Equal(t, km.Help, short[0])
	assert.Equal(t, km.Quit, short[1])
}

func TestKeyMap_FullHelp(t *testing.T) {
	t.Parallel()
	km := keyMap{
		NextCategory: key.NewBinding(key.WithKeys("shift+right"), key.WithHelp("shift+→", "next category")),
		PrevCategory: key.NewBinding(key.WithKeys("shift+left"), key.WithHelp("shift+←", "previous category")),
		FilterItems:  key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter items")),
		JumpTo:       key.NewBinding(key.WithKeys(":"), key.WithHelp(":", "jump to category")),
		ViewDetails:  key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "view details")),
		ApplyContext: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "apply context")),
		Help:         key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Quit:         key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
		Additionals:  map[domain.Category][]key.Binding{domain.BaseModel: {key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "view artifacts"))}},
		Category:     domain.BaseModel,
	}
	full := km.FullHelp()
	assert.NotEmpty(t, full)
	// Check that Additionals row matches
	assert.Equal(t, km.Additionals[km.Category], full[0])
	// Check that all bindings in other rows have non-empty help
	for _, row := range full[1:] {
		for _, b := range row {
			assert.NotEmpty(t, b.Help().Key)
			assert.NotEmpty(t, b.Help().Desc)
		}
	}
}
