package tui

import (
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"
)

func TestUpdateExportView_ExitAndQuit(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.viewMode = common.ExportView
	m.viewStack = []common.ViewMode{common.ListView}

	model, cmd := m.updateExportView(keyPress(keys.ExportCSV.Keys()[0]))
	assert.Equal(t, common.ListView, model.(*Model).viewMode)
	assert.Nil(t, cmd)

	model, cmd = m.updateExportView(keyPress(keys.Quit.Keys()[0]))
	assert.Equal(t, common.ListView, model.(*Model).viewMode)
	if cmd == nil {
		t.Fatal("expected quit cmd")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("expected quit message")
	}
}

// Quitting from the export popup must cancel in-flight loads, like every
// other view's Quit handler — otherwise a load running when the user quits
// from the popup is left uncancelled during teardown.
func TestUpdateExportView_QuitCancelsInFlight(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.viewMode = common.ExportView
	m.viewStack = []common.ViewMode{common.ListView}

	canceled := false
	m.loadCancel = func() { canceled = true }

	_, cmd := m.updateExportView(keyPress(keys.Quit.Keys()[0]))
	if !canceled {
		t.Fatal("quit from export view did not cancel in-flight tasks")
	}
	if cmd == nil {
		t.Fatal("expected quit cmd")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("expected quit message")
	}
}

func TestUpdateExportView_EscDismisses(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.viewMode = common.ExportView
	m.viewStack = []common.ViewMode{common.ListView}

	model, cmd := m.updateExportView(keyPress("esc"))
	assert.Equal(t, common.ListView, model.(*Model).viewMode)
	assert.Nil(t, cmd)
}

// TestUpdateExportView_DismissRestoresLastViewMode guards against a
// regression where esc or e is changed from "return to the view it opened over" to
// a hardcoded ListView. The popup must return the user to whichever
// view they opened it from, including DetailsView.
func TestUpdateExportView_DismissRestoresLastViewMode(t *testing.T) {
	t.Parallel()
	for _, k := range []tea.KeyMsg{
		keyPress("esc"),
		keyPress(keys.ExportCSV.Keys()[0]),
	} {
		m := newTestModel(t)
		m.viewMode = common.ExportView
		m.viewStack = []common.ViewMode{common.DetailsView}

		model, cmd := m.updateExportView(k)
		assert.Equal(t, common.DetailsView, model.(*Model).viewMode, "key=%v", k)
		assert.Nil(t, cmd, "key=%v", k)
	}
}

// newExportModel opens the export modal on a folder with two subfolders.
func newExportModel(t *testing.T) (*Model, string) {
	t.Helper()
	m := newGoldenModel(t, 120, 40)
	showCategory(t, m, domain.Tenant)
	dir := t.TempDir()
	for _, d := range []string{"alpha", "beta"} {
		require.NoError(t, os.Mkdir(filepath.Join(dir, d), 0o750))
	}
	m.dirPicker.CurrentDirectory = dir
	m.enterExportView()
	updated, _ := m.dirPicker.Update(m.dirPicker.Init()())
	m.dirPicker = &updated
	return m, dir
}

// enter exports into the folder shown, not the highlighted subfolder.
func TestExport_EnterWritesToShownFolder(t *testing.T) {
	t.Parallel()
	m, dir := newExportModel(t)

	_, cmd := m.updateExportView(keyPress("enter"))
	require.NotNil(t, cmd)
	done, ok := cmd().(exportDoneMsg)
	require.True(t, ok)
	assert.Equal(t, filepath.Join(dir, m.exportFilename()), done.path)
	assert.FileExists(t, done.path)
}

// → opens the highlighted folder; enter then exports there.
func TestExport_OpenThenExport(t *testing.T) {
	t.Parallel()
	m, dir := newExportModel(t)

	_, cmd := m.updateExportView(keyPress("right"))
	require.NotNil(t, cmd, "opening a folder re-reads it")
	assert.Equal(t, filepath.Join(dir, "alpha"), m.dirPicker.CurrentDirectory)
}

// tab focuses the name: typing edits it, and e or q are text, not keys.
func TestExport_RenameTakesKeysAsText(t *testing.T) {
	t.Parallel()
	m, dir := newExportModel(t)
	m.updateExportView(keyPress("tab"))
	require.True(t, m.naming)
	m.exportName.SetValue("")
	for _, r := range "qe.csv" {
		m.updateExportView(keyPress(string(r)))
	}
	assert.Equal(t, common.ExportView, m.viewMode, "e and q are typed, not run")
	assert.Equal(t, "qe.csv", m.exportName.Value())

	_, cmd := m.updateExportView(keyPress("enter"))
	done, ok := cmd().(exportDoneMsg)
	require.True(t, ok)
	assert.Equal(t, filepath.Join(dir, "qe.csv"), done.path)
}

// A blank name or one with a path separator is refused with a warning.
func TestExport_BadNamesWarn(t *testing.T) {
	t.Parallel()
	for name, want := range map[string]string{
		"  ":      "file name is required",
		"a/b.csv": "file name can't contain /",
	} {
		m, _ := newExportModel(t)
		m.exportName.SetValue(name)
		m.updateExportView(keyPress("enter"))
		require.NotNil(t, m.toasts.Active(), name)
		assert.Equal(t, want, m.toasts.Active().Msg)
		assert.Equal(t, common.ExportView, m.viewMode)
	}
}

func TestTildePath(t *testing.T) {
	t.Parallel()
	home, err := os.UserHomeDir()
	require.NoError(t, err)
	assert.Equal(t, "~", tildePath(home))
	assert.Equal(t, "~/Downloads", tildePath(filepath.Join(home, "Downloads")))
	assert.Equal(t, "/tmp", tildePath("/tmp"))
}

func TestTruncateLeft(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "~/a", truncateLeft("~/a", 5))
	assert.Equal(t, "…/c/d", truncateLeft("/a/b/c/d", 5))
	assert.Equal(t, "…", truncateLeft("abc", 1))
}
