/*
Package tui defines message types for the TUI model.
*/
package tui

import (
	"github.com/charmbracelet/bubbles/table"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/pkg/models"
)

// errMsg is a message containing an error.
type errMsg error

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

// filterApplyMsg carries a debounced filter value and a nonce to drop stale updates.
type filterApplyMsg struct {
	Value string
	Nonce int
}

type tableRowsComputedMsg struct {
	Rows  []table.Row
	Stats tableStats
	Nonce int
}

type detailContentRenderedMsg struct {
	Content string
	Err     error
	Nonce   int
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
