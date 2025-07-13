package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/table"
	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestAliasDetailViewIntegration(t *testing.T) {
	t.Parallel()
	// Simulate a row as produced by the Alias handler
	row := table.Row{"Tenant", "t/tenant"}
	key := getItemKey(domain.Alias, row)
	assert.Equal(t, "Tenant", key)

	item := findItem(nil, domain.Alias, key)
	cat, ok := item.(domain.Category)
	assert.True(t, ok)
	assert.Equal(t, domain.Tenant, cat)
}
