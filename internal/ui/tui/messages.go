/*
Package tui defines message types for the TUI model.
*/
package tui

import (
	"github.com/charmbracelet/bubbles/table"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/pkg/models"
)

// errMsg is a failed-load message tagged with the generation that issued the
// load, so a superseded load's error can be dropped (the user navigated on).
// Gen 0 is the always-apply sentinel (the foundational Init load) and is
// never dropped — mirroring dataMsg/the typed loaded messages.
type errMsg struct {
	err error
	Gen int
}

// Error implements error so errMsg can carry the failure to the toast and so
// errors.Is reaches the wrapped cause (e.g. context.Canceled) via Unwrap.
func (e errMsg) Error() string {
	if e.err == nil {
		return ""
	}
	return e.err.Error()
}

// Unwrap exposes the wrapped error for errors.Is/As.
func (e errMsg) Unwrap() error { return e.err }

// dataMsg is a message containing generic data and a generation id to avoid stale updates.
type dataMsg struct {
	Data any
	Gen  int
}

// datasetLoadedMsg is a typed message for the initial dataset load, with generation guard.
type datasetLoadedMsg struct {
	Dataset *models.Dataset
	Gen     int
}

// filterMsg is a message containing filter text.
type filterMsg string

// setFilterMsg is a message to set the filter text in the model.
type setFilterMsg string

// filterApplyMsg carries a debounced filter value and a gen to drop stale updates.
type filterApplyMsg struct {
	Value string
	Gen   int
}

type tableRowsComputedMsg struct {
	Rows  []table.Row
	Stats tableStats
	Gen   int
}

type detailContentRenderedMsg struct {
	Content string
	Err     error
	Gen     int
}

type deleteDoneMsg struct {
	category domain.Category
	key      models.ItemKey
}

type deleteErrMsg struct {
	err       error
	category  domain.Category
	key       models.ItemKey
	prevState string
}

type updateDoneMsg struct {
	err      error
	category domain.Category
}

type gpuPoolScaleStartedMsg struct {
	key models.ItemKey
}

type gpuPoolScaleResultMsg struct {
	key models.ItemKey
	err error
}

type cordonNodeResultMsg struct {
	key   models.ItemKey
	state bool
	err   error
}

type drainNodeResultMsg struct {
	key models.ItemKey
	err error
}

type rebootNodeResultMsg struct {
	key models.ItemKey
	err error
}

type exportDoneMsg struct{}

type exportErrMsg struct {
	err error
}

type baseModelsLoadedMsg struct {
	Items []models.BaseModel
	Gen   int
}

type importedModelsLoadedMsg struct {
	Items map[string][]models.ImportedModel
	Gen   int
}

type gpuPoolsLoadedMsg struct {
	Items []models.GPUPool
	Gen   int
}

type gpuNodesLoadedMsg struct {
	Items map[string][]models.GPUNode
	Gen   int
}

type gpuWorkloadsLoadedMsg struct {
	Items map[string][]models.GPUWorkload
	Gen   int
}

type dedicatedAIClustersLoadedMsg struct {
	Items map[string][]models.DedicatedAICluster
	Gen   int
}

type tenancyOverridesLoadedMsg struct {
	Group models.TenancyOverrideGroup
	Gen   int
}

type limitRegionalOverridesLoadedMsg struct {
	Items []models.LimitRegionalOverride
	Gen   int
}

type consolePropertyRegionalOverridesLoadedMsg struct {
	Items []models.ConsolePropertyRegionalOverride
	Gen   int
}

type propertyRegionalOverridesLoadedMsg struct {
	Items []models.PropertyRegionalOverride
	Gen   int
}

// k8sWatchStartedMsg signals that a category watch is live; Trigger yields
// a value on each debounced cluster change.
type k8sWatchStartedMsg struct {
	Cat     domain.Category
	Trigger <-chan struct{}
	Gen     int
}

// k8sWatchTriggeredMsg signals one debounced change; the reducer re-runs
// the category's loader and re-arms the listener.
type k8sWatchTriggeredMsg struct {
	Cat domain.Category
	Gen int
}

// k8sWatchClosedMsg signals the trigger channel closed (ctx cancel or
// stream death); the reducer falls back to a final one-shot load.
type k8sWatchClosedMsg struct {
	Cat domain.Category
	Gen int
}

// k8sWatchUnavailableMsg signals watch setup failed or is unsupported; the
// static load result stays on screen, no live indicator.
type k8sWatchUnavailableMsg struct {
	Cat domain.Category
	Gen int
}

// --- repo (working-tree) watch: session-scoped, no Gen/Cat. ---

// repoWatchStartedMsg signals the working-tree watch is live.
type repoWatchStartedMsg struct{ Trigger <-chan struct{} }

// repoWatchTriggeredMsg signals one debounced working-tree change; the reducer
// issues quiet background reloads and re-arms the listener.
type repoWatchTriggeredMsg struct{}

// repoWatchClosedMsg signals the working-tree watch is unavailable or died;
// the live repo indicator drops. No auto-reconnect.
type repoWatchClosedMsg struct{}

// datasetReloadedMsg carries a freshly loaded dataset to be merged into the
// in-memory one (repo-owned fields only; live k8s fields preserved).
type datasetReloadedMsg struct{ Dataset *models.Dataset }

// gpuPoolsReloadedMsg carries freshly loaded GPU pools (repo-sourced via their
// own loader, not LoadDataset) to refresh the cached pool list.
type gpuPoolsReloadedMsg struct{ Items []models.GPUPool }
