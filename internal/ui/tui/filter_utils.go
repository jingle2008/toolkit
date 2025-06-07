// Package toolkit provides filter and debounce utilities for the TUI model.
package tui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// FilterTable updates the model's filter and rows if the filter has changed.
func FilterTable(m *Model, filter string) {
	if filter == m.curFilter {
		return
	}
	m.curFilter = filter
	m.updateRows()
}

// DebounceFilter triggers a debounced filter update for the model's text input.
func DebounceFilter(m *Model) tea.Cmd {
	m.newFilter = strings.ToLower(m.textInput.Value())
	return tea.Tick(100*time.Millisecond, func(_ time.Time) tea.Msg {
		return FilterMsg{Text: m.newFilter}
	})
}
