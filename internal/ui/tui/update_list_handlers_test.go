package tui

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	"github.com/jingle2008/toolkit/pkg/models"
)

// stubClipboard swaps both clipboard seams for the duration of a test and
// returns a pointer to whatever was last written.
//
// Not parallel-safe: the seams are package-level, so tests using this must
// stay serial (no t.Parallel()) — same constraint the actions package's
// clipboard tests work under.
func stubClipboard(t *testing.T, readVal string, readErr error) *string {
	t.Helper()
	origRead, origWrite := clipboardReadAll, clipboardWriteAll
	t.Cleanup(func() {
		clipboardReadAll, clipboardWriteAll = origRead, origWrite
	})

	var written string
	clipboardReadAll = func() (string, error) { return readVal, readErr }
	clipboardWriteAll = func(s string) error {
		written = s
		return nil
	}
	return &written
}

func TestToggleAliases_EntersAliasCategory(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	require.NotEqual(t, domain.Alias, m.category, "precondition: not already on Alias")

	cmds := m.toggleAliases()
	assert.NotEmpty(t, cmds)
	assert.Equal(t, domain.Alias, m.category, "toggling should switch to the Alias category")
}

func TestToggleAliases_FromAliasGoesBack(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	// Enter Alias first so the second toggle takes the "go back" branch
	// instead of re-entering the same category.
	m.toggleAliases()
	require.Equal(t, domain.Alias, m.category)

	cmds := m.toggleAliases()
	require.Len(t, cmds, 1, "the back branch returns exactly one history command")
	assert.NotEqual(t, domain.Alias, m.category, "second toggle should leave Alias")
}

func TestEnterHelpView(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	before := m.viewMode

	m.enterHelpView()
	assert.Equal(t, common.HelpView, m.viewMode)
	assert.Equal(t, before, m.lastViewMode, "previous view must be remembered for Back")
}

func TestEnterExportView_NoPickerPath(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.dirPicker.Path = ""
	before := m.viewMode

	cmds := m.enterExportView()
	assert.Equal(t, common.ExportView, m.viewMode)
	assert.Equal(t, before, m.lastViewMode)
	require.Len(t, cmds, 1)
	assert.NotNil(t, cmds[0], "empty path should initialize the picker")
}

func TestEnterExportView_WithPickerPath(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.dirPicker.Path = t.TempDir()

	cmds := m.enterExportView()
	assert.Equal(t, common.ExportView, m.viewMode)
	require.Len(t, cmds, 1)
	require.NotNil(t, cmds[0])

	// With a path already chosen the picker is nudged with a backspace so
	// it re-renders the current directory rather than resetting.
	msg := cmds[0]()
	keyMsg, ok := msg.(tea.KeyMsg)
	require.True(t, ok, "expected a tea.KeyMsg, got %T", msg)
	assert.Equal(t, tea.KeyBackspace, keyMsg.Type)
}

//nolint:paralleltest // stubClipboard swaps package-level seams; these must stay serial
func TestPasteFilter_SetsFilterFromClipboard(t *testing.T) {
	stubClipboard(t, "  tenant-a  ", nil)
	m := newTestModel(t)

	msg := m.pasteFilter()()
	got, ok := msg.(setFilterMsg)
	require.True(t, ok, "expected setFilterMsg, got %T", msg)
	assert.Equal(t, "tenant-a", string(got), "clipboard text should be trimmed")
}

//nolint:paralleltest // stubClipboard swaps package-level seams; these must stay serial
func TestPasteFilter_ClipboardError(t *testing.T) {
	stubClipboard(t, "", errors.New("no clipboard"))
	m := newTestModel(t)

	// A clipboard failure is not worth interrupting the user over.
	assert.Nil(t, m.pasteFilter()())
}

//nolint:paralleltest // stubClipboard swaps package-level seams; these must stay serial
func TestPasteFilter_BlankClipboardIgnored(t *testing.T) {
	stubClipboard(t, "   \n\t ", nil)
	m := newTestModel(t)

	// Whitespace-only would otherwise apply an empty filter and look like
	// a no-op bug.
	assert.Nil(t, m.pasteFilter()())
}

//nolint:paralleltest // stubClipboard swaps package-level seams; these must stay serial
func TestCopyTenantID(t *testing.T) {
	// The write itself happens behind actions.CopyTenantID, whose seam is
	// private to that package (and covered by its own tests). Exercise the
	// wrapper through the two branches that don't reach the clipboard, so
	// this stays a pure unit test rather than clobbering the real one.
	written := stubClipboard(t, "", nil)
	m := newTestModel(t)

	for name, item := range map[string]any{
		"nil item":         nil,
		"unsupported type": "not-a-tenant",
	} {
		t.Run(name, func(t *testing.T) {
			cmd := m.copyTenantID(item)
			require.NotNil(t, cmd)
			assert.Nil(t, cmd(), "copy is fire-and-forget; it emits no message")
		})
	}
	assert.Empty(t, *written, "neither branch should touch the clipboard")
}

//nolint:paralleltest // stubClipboard swaps package-level seams; these must stay serial
func TestCopyItemJSON(t *testing.T) {
	written := stubClipboard(t, "", nil)
	m := newTestModel(t)

	item := models.Tenant{Name: "tenant1", IDs: []string{"id1"}}
	cmd := m.copyItemJSON(item)
	require.NotNil(t, cmd)
	assert.Nil(t, cmd())
	assert.Contains(t, *written, "tenant1", "clipboard should hold the item's JSON")
	assert.Contains(t, *written, "\n", "JSON should be pretty-printed")
}

//nolint:paralleltest // stubClipboard swaps package-level seams; these must stay serial
func TestCopyItemJSON_UnmarshalableItem(t *testing.T) {
	stubClipboard(t, "", nil)
	m := newTestModel(t)

	// A channel can't be marshaled; the command logs and returns nil
	// rather than propagating a failure into the update loop.
	cmd := m.copyItemJSON(make(chan int))
	require.NotNil(t, cmd)
	assert.Nil(t, cmd())
}
