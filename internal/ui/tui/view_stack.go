package tui

import (
	"slices"

	"github.com/jingle2008/toolkit/internal/ui/tui/common"
)

// The views overlays return to. A modal (help, export, a confirmation,
// the palette, …) and the full-screen loading and log views open over
// the view the user was in, and return to it when they close; the stack
// holds those views, the innermost last, so overlays opened over each
// other unwind in order and none can overwrite another's way back.

// openView shows v over the current view, which closeView returns to.
func (m *Model) openView(v common.ViewMode) {
	m.viewStack = append(m.viewStack, m.viewMode)
	m.viewMode = v
}

// closeView returns from the current view to the one it opened over.
func (m *Model) closeView() {
	m.viewMode = m.parentView()
	if n := len(m.viewStack); n > 0 {
		m.viewStack = m.viewStack[:n-1]
	}
}

// parentView is the view the current one opened over, which a modal
// draws over and closing it returns to: the list when there is none.
func (m *Model) parentView() common.ViewMode {
	if n := len(m.viewStack); n > 0 {
		return m.viewStack[n-1]
	}
	return common.ListView
}

// setBaseView switches to a full-screen view of its own (the list, an
// item's details), which no overlay is open over.
func (m *Model) setBaseView(v common.ViewMode) {
	m.viewMode = v
	m.viewStack = m.viewStack[:0]
}

// leaveLoading ends the full-screen loading view: closes it when it is
// current, or, when the user opened something over it, drops it from
// under them so closing that returns past it.
func (m *Model) leaveLoading() {
	if m.viewMode == common.LoadingView {
		m.closeView()
		return
	}
	if i := slices.Index(m.viewStack, common.LoadingView); i >= 0 {
		m.viewStack = slices.Delete(m.viewStack, i, i+1)
	}
}
