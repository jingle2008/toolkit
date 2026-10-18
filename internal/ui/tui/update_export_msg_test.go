package tui

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/ui/tui/common"
)

func TestUpdateExportView_DoneRestoresViewAndSaysWhere(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.lastViewMode = common.DetailsView
	m.viewMode = common.ExportView

	got, _ := m.updateExportView(exportDoneMsg{path: "/tmp/out.csv"})
	require.Same(t, m, got)
	assert.Equal(t, common.DetailsView, m.viewMode, "success returns to the view the popup came from")
	require.NotNil(t, m.toasts.active)
	assert.Equal(t, "exported to /tmp/out.csv", m.toasts.active.msg)
}

func TestUpdateExportView_ErrRestoresPreviousViewAndToasts(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.lastViewMode = common.DetailsView
	m.viewMode = common.ExportView

	got, _ := m.updateExportView(exportErrMsg{err: errors.New("disk full")})
	require.Same(t, m, got)
	assert.Equal(t, common.DetailsView, m.viewMode, "failure should restore the view the popup came from")
	require.NotNil(t, m.toasts.active, "an error should surface as a toast")
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

	path := t.TempDir() + "/out.csv"
	msg := m.exportTableCSVCmd(path)()
	done, ok := msg.(exportDoneMsg)
	require.True(t, ok, "expected exportDoneMsg, got %T", msg)
	assert.Equal(t, path, done.path)
}
