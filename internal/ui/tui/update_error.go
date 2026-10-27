package tui

import (
	"context"
	"errors"
	"slices"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jingle2008/toolkit/internal/domain"
	keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"
)

/*
handleErrMsg centralizes error handling for async operations.

It dismisses LoadingView via endTask and surfaces the failure as a
toast over the restored view — naming r to retry where the category can
refresh — so the user can keep navigating instead of being trapped.
*/
func (m *Model) handleErrMsg(msg errMsg) tea.Cmd {
	err := msg.err

	// Drop errors from a superseded load: the user has navigated on, so the
	// old category's failure is no longer relevant to show. Gen 0 is the
	// always-apply sentinel (the foundational Init load) and is never dropped.
	// endTask still runs to keep pendingTasks balanced.
	if msg.Gen != 0 && msg.Gen != m.gens.msg {
		if m.logger != nil {
			m.logger.Debug("dropping stale load error", "msgGen", msg.Gen, "gen", m.gens.msg, "error", err)
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
			m.logger.Debug("in-flight load canceled", "category", m.category, "error", err)
		}
		m.endTask(false)
		return nil
	}

	// Log the underlying error before it's transient-only in the toast.
	// Without this the only trace of a failed load lives in the banner
	// for 8s, then nothing.
	if m.logger != nil {
		m.logger.Error("data load failed", "category", m.category, "error", err)
	}
	if m.ownLoadFailed(msg.Cat) {
		m.loadFailed = true
	}
	m.endTask(false)
	// An error toast outlives an environment switch, so one from an
	// environment-scoped load names where it failed.
	text := msg.Error()
	if m.category.IsEnvironmentScoped() {
		text = m.environment.KubeContext() + ": " + text
	}
	if m.canRefresh() {
		return m.showErrorWithNext(text, keys.Refresh.Help().Key+" retry")
	}
	return m.showToast(text, toastError)
}

// ownLoadFailed reports whether a failed load of cat is the on-screen
// category's own (its rows), so the empty list should say the load failed.
// The tenants load (Tenant) is an override category's own data, but for a
// tenant-owned category it only resolves owners: its rows did load. The
// foundational dataset load (CategoryUnknown) holds the repo categories.
func (m *Model) ownLoadFailed(cat domain.Category) bool {
	switch cat {
	case m.category:
		return true
	case domain.CategoryUnknown:
		return !m.category.NeedsKubeConfig()
	case domain.Tenant:
		_, override := tenancyOverrideCategories[m.category]
		return override
	default:
		return false
	}
}

// canRefresh reports whether r (refresh) is bound in the current category.
func (m *Model) canRefresh() bool {
	return slices.ContainsFunc(m.keys.Context, func(b key.Binding) bool { return keys.Same(b, keys.Refresh) })
}
