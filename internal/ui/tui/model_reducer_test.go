package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/stopwatch"
	"github.com/charmbracelet/bubbles/table"
	"github.com/jingle2008/toolkit/internal/domain"
	logging "github.com/jingle2008/toolkit/pkg/infra/logging"
	"github.com/jingle2008/toolkit/pkg/models"
	"github.com/stretchr/testify/assert"
)

func TestHandleTenancyOverridesGroup(t *testing.T) {
	t.Parallel()
	s := spinner.New()
	m := &Model{loadingSpinner: &s}
	cmd := m.handleTenancyOverridesGroup()
	assert.NotNil(t, cmd)

	// Now with all required fields non-nil
	m = &Model{
		dataset: &models.Dataset{
			Tenants:                           []models.Tenant{},
			LimitTenancyOverrideMap:           map[string][]models.LimitTenancyOverride{},
			ConsolePropertyTenancyOverrideMap: map[string][]models.ConsolePropertyTenancyOverride{},
			PropertyTenancyOverrideMap:        map[string][]models.PropertyTenancyOverride{},
		},
		loadingSpinner: &s,
	}
	cmd = m.handleTenancyOverridesGroup()
	assert.Nil(t, cmd)
}

func TestHandleLimitRegionalOverrideCategory(t *testing.T) {
	t.Parallel()
	s := spinner.New()
	m := &Model{loadingSpinner: &s}
	cmd := m.handleLimitRegionalOverrideCategory()
	assert.NotNil(t, cmd)

	m = &Model{
		dataset:        &models.Dataset{LimitRegionalOverrides: []models.LimitRegionalOverride{}},
		loadingSpinner: &s,
	}
	cmd = m.handleLimitRegionalOverrideCategory()
	assert.Nil(t, cmd)
}

func TestHandleConsolePropertyRegionalOverrideCategory(t *testing.T) {
	t.Parallel()
	s := spinner.New()
	m := &Model{loadingSpinner: &s}
	cmd := m.handleConsolePropertyRegionalOverrideCategory()
	assert.NotNil(t, cmd)

	m = &Model{
		dataset:        &models.Dataset{ConsolePropertyRegionalOverrides: []models.ConsolePropertyRegionalOverride{}},
		loadingSpinner: &s,
	}
	cmd = m.handleConsolePropertyRegionalOverrideCategory()
	assert.Nil(t, cmd)
}

func TestHandlePropertyRegionalOverrideCategory(t *testing.T) {
	t.Parallel()
	s := spinner.New()
	m := &Model{loadingSpinner: &s}
	cmd := m.handlePropertyRegionalOverrideCategory()
	assert.NotNil(t, cmd)

	m = &Model{
		dataset:        &models.Dataset{PropertyRegionalOverrides: []models.PropertyRegionalOverride{}},
		loadingSpinner: &s,
	}
	cmd = m.handlePropertyRegionalOverrideCategory()
	assert.Nil(t, cmd)
}

func TestHandleGpuPoolCategory(t *testing.T) {
	t.Parallel()
	s := spinner.New()
	m := &Model{loadingSpinner: &s}
	cmd := m.handleGpuPoolCategory()
	assert.NotNil(t, cmd)

	m = &Model{
		dataset:        &models.Dataset{GpuPools: []models.GpuPool{}},
		loadingSpinner: &s,
	}
	cmd = m.handleGpuPoolCategory()
	assert.Nil(t, cmd)
}

func TestHandleGpuNodeCategory(t *testing.T) {
	t.Parallel()
	s := spinner.New()
	m := &Model{loadingSpinner: &s}
	cmd := m.handleGpuNodeCategory(false)
	assert.NotNil(t, cmd)

	m = &Model{
		dataset:        &models.Dataset{GpuNodeMap: map[string][]models.GpuNode{}},
		loadingSpinner: &s,
	}
	cmd = m.handleGpuNodeCategory(false)
	assert.Nil(t, cmd)
}

func TestHandleDedicatedAIClusterCategory(t *testing.T) {
	t.Parallel()
	s := spinner.New()
	m := &Model{loadingSpinner: &s}
	cmd := m.handleDedicatedAIClusterCategory(false)
	assert.NotNil(t, cmd)

	m = &Model{
		dataset:        &models.Dataset{DedicatedAIClusterMap: map[string][]models.DedicatedAICluster{}},
		loadingSpinner: &s,
	}
	cmd = m.handleDedicatedAIClusterCategory(false)
	assert.Nil(t, cmd)
}

func TestEnterContext(t *testing.T) {
	t.Parallel()
	s := spinner.New()
	w := stopwatch.Model{}
	m := &Model{
		table:          &table.Model{},
		category:       domain.Tenant,
		loadingSpinner: &s,
		loadingTimer:   &w,
	}
	// Seed initial history as in NewModel
	m.history = []domain.Category{m.category}
	m.historyIdx = 0
	// Simulate a selected row
	m.table.SetRows([]table.Row{{"row1"}})
	// Select the first row (SetCursor(0) if available)
	if m.table.Cursor() != 0 {
		m.table.SetCursor(0)
	}
	cmd := m.enterContext()
	assert.NotNil(t, cmd) // Should not panic or error
}

// mockEnv returns a mock environment with the given name.
func TestFindContextIndex_Environment(t *testing.T) {
	t.Parallel()
	m := &Model{}
	m.category = domain.Environment
	m.environment = models.Environment{Type: "dev", Region: "test"}
	rows := []table.Row{
		{"prod-UNKNOWN", "realm1", "type1", "region1"},
		{"dev-UNKNOWN", "realm2", "type2", "region2"},
		{"test-UNKNOWN", "realm3", "type3", "region3"},
	}
	idx := m.findContextIndex(rows)
	assert.Equal(t, 1, idx)
}

func TestFindContextIndex_ContextCategory(t *testing.T) {
	t.Parallel()
	m := &Model{}
	m.category = domain.Tenant
	m.context = &domain.ToolkitContext{
		Category: domain.Tenant,
		Name:     "tenant2",
	}
	rows := []table.Row{
		{"tenant1", "id1", "overrides1"},
		{"tenant2", "id2", "overrides2"},
		{"tenant3", "id3", "overrides3"},
	}
	idx := m.findContextIndex(rows)
	assert.Equal(t, 1, idx)
}

func TestFindContextIndex_NoMatch(t *testing.T) {
	t.Parallel()
	m := &Model{}
	m.category = domain.Environment
	m.environment = models.Environment{Type: "notfound"}
	rows := []table.Row{
		{"prod", "realm1", "type1", "region1"},
		{"dev", "realm2", "type2", "region2"},
	}
	idx := m.findContextIndex(rows)
	assert.Equal(t, -1, idx)
}

func TestFindContextIndex_ContextCategory_NoMatch(t *testing.T) {
	t.Parallel()
	m := &Model{}
	m.category = domain.Tenant
	m.context = &domain.ToolkitContext{
		Category: domain.Tenant,
		Name:     "notfound",
	}
	rows := []table.Row{
		{"tenant1", "id1", "overrides1"},
		{"tenant2", "id2", "overrides2"},
	}
	idx := m.findContextIndex(rows)
	assert.Equal(t, -1, idx)
}

func TestShowFaultyToggleAllowed(t *testing.T) {
	t.Parallel()
	m := &Model{
		category:       domain.Tenant,
		curFilter:      "",
		context:        nil,
		table:          &table.Model{},
		loadingSpinner: &spinner.Model{},
		logger:         logging.NewNoOpLogger(),
		dataset:        &models.Dataset{},
	}
	assert.False(t, m.showFaulty)
	_ = m.toggleFaultyList()
	assert.True(t, m.showFaulty)
	_ = m.toggleFaultyList()
	assert.False(t, m.showFaulty)
}
