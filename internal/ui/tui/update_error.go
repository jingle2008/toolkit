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
	// Log the underlying error before it's transient-only in the toast.
	// Without this the only trace of a failed load lives in the banner
	// for 8s, then nothing.
	if m.logger != nil {
		m.logger.Errorw("data load failed", "category", m.category, "err", error(msg))
	}
	m.endTask(false)
	return m.showToast(msg.Error(), toastError)
}
