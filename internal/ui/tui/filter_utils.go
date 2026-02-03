/*
Package tui provides filter and debounce utilities for the TUI model.
*/
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
	m.updateRows(true)
}

/*
DebounceFilter triggers a debounced filter update for the model's text input.
Emits FilterApplyMsg with a nonce so only the most recent tick applies.
*/
func DebounceFilter(m *Model) tea.Cmd {
	val := strings.ToLower(m.textInput.Value())
	m.filterNonce++
	nonce := m.filterNonce
	return tea.Tick(100*time.Millisecond, func(_ time.Time) tea.Msg {
		return FilterApplyMsg{Value: val, Nonce: nonce}
	})
}
