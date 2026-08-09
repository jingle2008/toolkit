package tui

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/ui/tui/common"
)

func TestUpdateExportView_DoneReturnsToList(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.viewMode = common.ExportView

	got, cmd := m.updateExportView(exportDoneMsg{})
	require.Same(t, m, got)
	assert.Nil(t, cmd)
	// A successful export always lands on the list, not lastViewMode —
	// the popup was opened from the list in the first place.
	assert.Equal(t, common.ListView, m.viewMode)
}

func TestUpdateExportView_ErrRestoresPreviousViewAndToasts(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.lastViewMode = common.DetailsView
	m.viewMode = common.ExportView

	got, cmd := m.updateExportView(exportErrMsg{err: errors.New("disk full")})
	require.Same(t, m, got)
	assert.Equal(t, common.DetailsView, m.viewMode, "failure should restore the view the popup came from")
	require.NotNil(t, cmd, "an error should surface as a toast")
}

func TestExportTableCSVCmd_ErrorBecomesExportErrMsg(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)

	// A path under a nonexistent directory can't be created, so the
	// command must translate the failure into a message instead of
	// panicking inside the update loop.
	msg := m.exportTableCSVCmd("/nonexistent/dir/out.csv")()
	errMsg, ok := msg.(exportErrMsg)
	require.True(t, ok, "expected exportErrMsg, got %T", msg)
	require.Error(t, errMsg.err)
}

func TestExportTableCSVCmd_SuccessBecomesExportDoneMsg(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)

	msg := m.exportTableCSVCmd(t.TempDir() + "/out.csv")()
	_, ok := msg.(exportDoneMsg)
	assert.True(t, ok, "expected exportDoneMsg, got %T", msg)
}
