package tui

import (
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"

	"github.com/jingle2008/toolkit/internal/domain"
)

func TestEmptyMessage(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		setup func(m *Model)
		want  string
	}{
		{"loading", func(m *Model) { m.pendingTasks = 1 }, "Loading GPUNode…"},
		{"filter", func(m *Model) { m.filter = "zzz" }, "Nothing matches /zzz · esc clears the filter"},
		{"faulty", func(m *Model) { m.showFaulty = true }, "No faulty GPUNode · ^z shows all"},
		{"scope", func(m *Model) { m.scope = &domain.Scope{Category: domain.GPUPool, Name: "pool-a"} }, "No GPUNode in pool-a"},
		{"environment", func(*Model) {}, "No GPUNode in dp-prod-iad"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			m := newGoldenModel(t, 120, 40)
			m.category = domain.GPUNode
			tc.setup(m)
			assert.Equal(t, tc.want, m.emptyMessage())
		})
	}
}

func TestEmptyMessage_RealmWideCategoryNamesNoContext(t *testing.T) {
	t.Parallel()
	m := newGoldenModel(t, 120, 40)
	m.category = domain.LimitDefinition
	assert.Equal(t, "No LimitDefinition", m.emptyMessage())
}

func TestListBody_EmptyShowsMessageUnderHeader(t *testing.T) {
	t.Parallel()
	m := newGoldenModel(t, 120, 40)
	showCategory(t, m, domain.GPUNode)
	m.filter = "zzz"
	m.updateRows(true)

	body := ansi.Strip(m.listBody())
	assert.Contains(t, body, "Name")
	assert.Contains(t, body, "Nothing matches /zzz")
}
