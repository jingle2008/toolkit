/*
Package tui provides filter and debounce utilities for the TUI model.
*/
package tui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func filterTableAsync(m *Model, filter string) tea.Cmd {
	if filter == m.filter {
		return nil
	}
	m.filter = filter
	return m.updateRowsAsync()
}

/*
DebounceFilter triggers a debounced filter update for the model's text input.
Emits filterApplyMsg with a nonce so only the most recent tick applies.
*/
func DebounceFilter(m *Model) tea.Cmd {
	val := strings.ToLower(m.textInput.Value())
	m.filterNonce++
	nonce := m.filterNonce
	return tea.Tick(100*time.Millisecond, func(_ time.Time) tea.Msg {
		return filterApplyMsg{Value: val, Nonce: nonce}
	})
}
