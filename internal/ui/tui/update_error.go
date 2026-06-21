package tui

import (
	"context"
	"errors"

	tea "github.com/charmbracelet/bubbletea"
)

/*
handleErrMsg centralizes error handling for async operations.

It dismisses LoadingView via endTask and surfaces the failure as a
transient toast over the restored view — so the user can keep
navigating instead of being trapped.
*/
func (m *Model) handleErrMsg(msg errMsg) tea.Cmd {
	err := msg.err

	// Drop errors from a superseded load: the user has navigated on, so the
	// old category's failure is no longer relevant to show. Gen 0 is the
	// always-apply sentinel (the foundational Init load) and is never dropped.
	// endTask still runs to keep pendingTasks balanced.
	if msg.Gen != 0 && msg.Gen != m.gen {
		if m.logger != nil {
			m.logger.Debugw("dropping stale load error", "msgGen", msg.Gen, "gen", m.gen, "error", err)
		}
		m.endTask(false)
		return nil
	}

	// A canceled in-flight load is the expected result of navigating away
	// (newLoadContext cancels loadCtx) or quitting — not a failure. Drop it
	// quietly: no error-level log with a stack trace, no red toast flashed
	// over the view the user just moved to. endTask still runs to keep
	// pendingTasks balanced.
	if err != nil && errors.Is(err, context.Canceled) {
		if m.logger != nil {
			m.logger.Debugw("in-flight load canceled", "category", m.category, "error", err)
		}
		m.endTask(false)
		return nil
	}

	// Log the underlying error before it's transient-only in the toast.
	// Without this the only trace of a failed load lives in the banner
	// for 8s, then nothing.
	if m.logger != nil {
		m.logger.Errorw("data load failed", "category", m.category, "error", err)
	}
	m.endTask(false)
	return m.showToast(msg.Error(), toastError)
}
