package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/stopwatch"

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

	// Two stale loads arrive (Gen=1 and a hypothetical Gen=1 typed
	// handler), both must drop. Both must still call endTask so the
	// matching beginTasks balance out. Tests every drop path
	// without needing a fully wired-up Model (the success path
	// requires textInput / table state that's out of scope here).
	stale := datasetLoadedMsg{Dataset: &models.Dataset{}, Gen: 1}
	_, _ = m.routeLoadingMsg(stale)

	if m.pendingTasks != 1 {
		t.Errorf("after stale datasetLoadedMsg drop: pendingTasks = %d, want 1 (stale drops must still endTask)", m.pendingTasks)
	}

	staleTyped := importedModelsLoadedMsg{Items: map[string][]models.ImportedModel{}, Gen: 1}
	_, _ = m.routeLoadingMsg(staleTyped)

	if m.pendingTasks != 0 {
		t.Errorf("after stale importedModelsLoadedMsg drop: pendingTasks = %d, want 0", m.pendingTasks)
	}
	if m.viewMode == common.LoadingView {
		t.Error("viewMode still LoadingView after all (stale) loads drained — startup hang regression")
	}
}
