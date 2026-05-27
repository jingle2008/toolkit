package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/stopwatch"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	"github.com/jingle2008/toolkit/pkg/models"
)

// TestStartupHang_LazyCategory reproduces and pins the fix for the
// "stuck in loading forever" bug on `toolkit -c <lazy-category>`.
//
// Init() on a lazy-loaded category (ImportedModel, BaseModel,
// GPUPool, GPUNode, DAC) issues TWO beginTask calls:
//
//  1. loadData() bumps gen to 1 and begins the dataset load.
//  2. updateCategory() bumps gen to 2 and begins the lazy load.
//
// The dataset load's response carries Gen=1, which the gen guard
// drops (1 != m.gen=2). Before the fix, the drop path returned
// early without endTask, leaving pendingTasks elevated by 1; even
// after the lazy load completes and decrements once, the model is
// stuck in LoadingView.
//
// After the fix, stale drops still call endTask so the begin/end
// pair stays balanced and the view transitions back to ListView.
func TestStartupHang_LazyCategory(t *testing.T) {
	t.Parallel()

	m := &Model{
		category:       domain.ImportedModel,
		loadingSpinner: &spinner.Model{},
		loadingTimer:   &stopwatch.Model{},
		logger:         fakeLogger{},
		viewMode:       common.ListView,
	}

	// Simulate the two beginTask calls Init() makes for a lazy
	// category: dataset (gen=1) and lazy load (gen=2). Each
	// beginTask increments pendingTasks; the first also flips the
	// view to LoadingView.
	_ = m.beginTask()
	m.gen = 1
	_ = m.beginTask()
	m.gen = 2

	if m.pendingTasks != 2 {
		t.Fatalf("pendingTasks after two beginTasks = %d, want 2", m.pendingTasks)
	}
	if m.viewMode != common.LoadingView {
		t.Fatalf("viewMode after first beginTask = %v, want LoadingView", m.viewMode)
	}

	// Two stale loads arrive (Gen=1 each), both must drop. Both must
	// still call endTask so the matching beginTasks balance out. Tests
	// the drop path without needing a fully wired-up Model (the
	// success path requires textInput / table state that's out of
	// scope here).
	_ = m.handleDataMsg(dataMsg{Data: &models.Dataset{}, Gen: 1})

	if m.pendingTasks != 1 {
		t.Errorf("after stale datasetLoadedMsg drop: pendingTasks = %d, want 1 (stale drops must still endTask)", m.pendingTasks)
	}

	m.handleImportedModelsLoaded(map[string][]models.ImportedModel{}, 1)

	if m.pendingTasks != 0 {
		t.Errorf("after stale importedModelsLoadedMsg drop: pendingTasks = %d, want 0", m.pendingTasks)
	}
	if m.viewMode == common.LoadingView {
		t.Error("viewMode still LoadingView after all (stale) loads drained — startup hang regression")
	}
}

// TestLoadedMsg_ArrivesDuringDetailsView pins the routing fix from
// code review issue #2: once load failures stop trapping the user in
// ErrorView, they can navigate into DetailsView while a load is still
// pending. The typed *LoadedMsg must drain pendingTasks regardless of
// the active view — if it only fires through ListView delegation, the
// inline spinner ticks forever.
func TestLoadedMsg_ArrivesDuringDetailsView(t *testing.T) {
	t.Parallel()

	s := spinner.New()
	w := stopwatch.New()
	tbl := table.New()
	ti := textinput.New()
	m := &Model{
		category:       domain.ImportedModel,
		loadingSpinner: &s,
		loadingTimer:   &w,
		logger:         fakeLogger{},
		viewMode:       common.DetailsView, // user navigated away during the load
		dataset:        &models.Dataset{},  // already past first boot
		table:          &tbl,
		textInput:      &ti,
	}
	_ = m.beginTask()
	if m.pendingTasks != 1 {
		t.Fatalf("pendingTasks = %d before load, want 1", m.pendingTasks)
	}

	// Drive the message through Update — this is the path a real
	// completed load takes. The top-level switch must route to the
	// typed handler, not delegateToActiveView(DetailsView) which
	// would silently drop it.
	m.gen = 1
	loaded := importedModelsLoadedMsg{
		Items: map[string][]models.ImportedModel{"acme": nil},
		Gen:   1,
	}
	_, _ = m.Update(loaded)

	if m.pendingTasks != 0 {
		t.Errorf("pendingTasks = %d after loaded msg, want 0 (must drain regardless of viewMode)", m.pendingTasks)
	}
	if len(m.dataset.ImportedModelMap) == 0 {
		t.Error("dataset was not updated with the imported models — top-level route did not fire")
	}
}

// TestBeginTask_KeepsActiveViewWhenDatasetExists pins the inline-loading
// invariant: once a dataset is loaded, subsequent loads must not switch
// to the full-screen LoadingView; the user keeps seeing the active view
// while the status-bar spinner indicates progress.
func TestBeginTask_KeepsActiveViewWhenDatasetExists(t *testing.T) {
	t.Parallel()

	m := &Model{
		loadingSpinner: &spinner.Model{},
		loadingTimer:   &stopwatch.Model{},
		logger:         fakeLogger{},
		viewMode:       common.ListView,
		dataset:        &models.Dataset{}, // already loaded
	}

	cmd := m.beginTask()
	if cmd == nil {
		t.Fatal("beginTask should still return spinner/timer tick cmd even when not switching view")
	}
	if m.viewMode != common.ListView {
		t.Errorf("viewMode = %v, want ListView (dataset != nil so we should stay inline)", m.viewMode)
	}
	if m.pendingTasks != 1 {
		t.Errorf("pendingTasks = %d, want 1", m.pendingTasks)
	}

	m.endTask(true)
	if m.viewMode != common.ListView {
		t.Errorf("viewMode after endTask = %v, want ListView", m.viewMode)
	}
}
