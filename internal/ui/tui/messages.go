/*
Package tui defines message types for the TUI model.
*/
package tui

import (
	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/pkg/models"
)

// ErrMsg is a message containing an error.
type ErrMsg error

// DataMsg is a message containing generic data and a generation id to avoid stale updates.
type DataMsg struct {
	Data any
	Gen  int
}

// datasetLoadedMsg is a typed message for the initial dataset load, with generation guard.
type datasetLoadedMsg struct {
	Dataset *models.Dataset
	Gen     int
}

// FilterMsg is a message containing filter text.
type FilterMsg string

// SetFilterMsg is a message to set the filter text in the model.
type SetFilterMsg string

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

type baseModelsLoadedMsg struct {
	Items []models.BaseModel
}

type gpuPoolsLoadedMsg struct {
	Items []models.GpuPool
}

type gpuNodesLoadedMsg struct {
	Items map[string][]models.GpuNode
}

type dedicatedAIClustersLoadedMsg struct {
	Items map[string][]models.DedicatedAICluster
}

type tenancyOverridesLoadedMsg struct {
	Group models.TenancyOverrideGroup
}

type limitRegionalOverridesLoadedMsg struct {
	Items []models.LimitRegionalOverride
}

type consolePropertyRegionalOverridesLoadedMsg struct {
	Items []models.ConsolePropertyRegionalOverride
}

type propertyRegionalOverridesLoadedMsg struct {
	Items []models.PropertyRegionalOverride
}
