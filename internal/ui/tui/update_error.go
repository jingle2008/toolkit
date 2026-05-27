package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

/*
handleErrMsg centralizes error handling for async operations.

It dismisses LoadingView via endTask and surfaces the failure as a
transient toast over the restored view — so the user can keep
navigating instead of being trapped.
*/
func (m *Model) handleErrMsg(msg errMsg) tea.Cmd {
	m.endTask(false)
	return m.showToast(msg.Error(), toastError)
}
