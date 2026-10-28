package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/jingle2008/toolkit/internal/ui/tui/common"
)

// Overlays opened over each other unwind in order, each to the view it
// opened over.
func TestViewStack_UnwindsInOrder(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.setBaseView(common.DetailsView)
	m.openView(common.HelpView)
	m.openView(common.ConfirmView)
	assert.Equal(t, common.HelpView, m.parentView(), "the confirmation draws over help")

	m.closeView()
	assert.Equal(t, common.HelpView, m.viewMode)
	m.closeView()
	assert.Equal(t, common.DetailsView, m.viewMode)
	m.closeView()
	assert.Equal(t, common.ListView, m.viewMode, "nothing left: the list")
}

// A load that ends while something is open over the loading view drops
// it from under that, so closing it returns past the finished spinner —
// where a single return slot pointed back at the loading view.
func TestViewStack_LoadEndsUnderAnOverlay(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.openView(common.LoadingView)
	m.openView(common.HelpView)

	m.leaveLoading()
	assert.Equal(t, common.HelpView, m.viewMode, "the open overlay stays")
	m.closeView()
	assert.Equal(t, common.ListView, m.viewMode)
}

// A full-screen view drops the returns of overlays it replaced.
func TestViewStack_BaseViewClears(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.openView(common.HelpView)
	m.setBaseView(common.DetailsView)
	assert.Empty(t, m.viewStack)
	assert.Equal(t, common.ListView, m.parentView())
}
