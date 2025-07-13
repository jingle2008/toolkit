package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseCategory_GpuNodeShortAlias(t *testing.T) {
	t.Parallel()
	cat, err := ParseCategory("gn")
	assert.NoError(t, err)
	assert.Equal(t, GpuNode, cat)
}

func TestAliases_ContainsAllCatLookupKeys(t *testing.T) {
	t.Parallel()
	// catLookup is private, but we can check that all aliases in Aliases are parseable
	for _, alias := range Aliases {
		cat, err := ParseCategory(alias)
		assert.NoError(t, err, "Alias %q should be parseable", alias)
		assert.NotEqual(t, CategoryUnknown, cat, "Alias %q should not map to CategoryUnknown", alias)
	}
}

func TestAliases_IterationRange(t *testing.T) {
	t.Parallel()
	for c := Tenant; c <= Alias; c++ {
		aliases := c.GetAliases()
		assert.NotEmpty(t, aliases, "Category %v should have at least one alias", c)
	}
}

func TestParseCategory_Unknown(t *testing.T) {
	t.Parallel()
	cat, err := ParseCategory("not-real")
	assert.Error(t, err)
	assert.Equal(t, CategoryUnknown, cat)
}
