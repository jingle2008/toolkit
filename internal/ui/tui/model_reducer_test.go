package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/table"
	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/pkg/models"
	"github.com/stretchr/testify/assert"
)

func TestHandleTenancyOverridesGroup(t *testing.T) {
	t.Parallel()
	m := &Model{}
	cmd := m.handleTenancyOverridesGroup()
	assert.NotNil(t, cmd)
	assert.Equal(t, 1, m.pendingTasks)

	// Now with all required fields non-nil
	m = &Model{
		dataset: &models.Dataset{
			Tenants:                           []models.Tenant{},
			LimitTenancyOverrideMap:           map[string][]models.LimitTenancyOverride{},
			ConsolePropertyTenancyOverrideMap: map[string][]models.ConsolePropertyTenancyOverride{},
			PropertyTenancyOverrideMap:        map[string][]models.PropertyTenancyOverride{},
		},
	}
	m.pendingTasks = 0
	cmd = m.handleTenancyOverridesGroup()
	assert.NotNil(t, cmd)
	assert.Equal(t, 0, m.pendingTasks)
}

func TestHandleLimitRegionalOverrideCategory(t *testing.T) {
	t.Parallel()
	m := &Model{}
	cmd := m.handleLimitRegionalOverrideCategory()
	assert.NotNil(t, cmd)
	assert.Equal(t, 1, m.pendingTasks)

	m = &Model{dataset: &models.Dataset{LimitRegionalOverrides: []models.LimitRegionalOverride{}}}
	m.pendingTasks = 0
	cmd = m.handleLimitRegionalOverrideCategory()
	assert.NotNil(t, cmd)
	assert.Equal(t, 0, m.pendingTasks)
}

func TestHandleConsolePropertyRegionalOverrideCategory(t *testing.T) {
	t.Parallel()
	m := &Model{}
	cmd := m.handleConsolePropertyRegionalOverrideCategory()
	assert.NotNil(t, cmd)
	assert.Equal(t, 1, m.pendingTasks)

	m = &Model{dataset: &models.Dataset{ConsolePropertyRegionalOverrides: []models.ConsolePropertyRegionalOverride{}}}
	m.pendingTasks = 0
	cmd = m.handleConsolePropertyRegionalOverrideCategory()
	assert.NotNil(t, cmd)
	assert.Equal(t, 0, m.pendingTasks)
}

func TestHandlePropertyRegionalOverrideCategory(t *testing.T) {
	t.Parallel()
	m := &Model{}
	cmd := m.handlePropertyRegionalOverrideCategory()
	assert.NotNil(t, cmd)
	assert.Equal(t, 1, m.pendingTasks)

	m = &Model{dataset: &models.Dataset{PropertyRegionalOverrides: []models.PropertyRegionalOverride{}}}
	m.pendingTasks = 0
	cmd = m.handlePropertyRegionalOverrideCategory()
	assert.NotNil(t, cmd)
	assert.Equal(t, 0, m.pendingTasks)
}

func TestHandleGpuPoolCategory(t *testing.T) {
	t.Parallel()
	m := &Model{}
	cmd := m.handleGpuPoolCategory()
	assert.NotNil(t, cmd)
	assert.Equal(t, 1, m.pendingTasks)

	m = &Model{dataset: &models.Dataset{GpuPools: []models.GpuPool{}}}
	m.pendingTasks = 0
	cmd = m.handleGpuPoolCategory()
	assert.NotNil(t, cmd)
	assert.Equal(t, 0, m.pendingTasks)
}

func TestHandleGpuNodeCategory(t *testing.T) {
	t.Parallel()
	m := &Model{}
	cmd := m.handleGpuNodeCategory()
	assert.NotNil(t, cmd)
	assert.Equal(t, 1, m.pendingTasks)

	m = &Model{dataset: &models.Dataset{GpuNodeMap: map[string][]models.GpuNode{}}}
	m.pendingTasks = 0
	cmd = m.handleGpuNodeCategory()
	assert.NotNil(t, cmd)
	assert.Equal(t, 0, m.pendingTasks)
}

func TestHandleDedicatedAIClusterCategory(t *testing.T) {
	t.Parallel()
	m := &Model{}
	cmd := m.handleDedicatedAIClusterCategory()
	assert.NotNil(t, cmd)
	assert.Equal(t, 1, m.pendingTasks)

	m = &Model{dataset: &models.Dataset{DedicatedAIClusterMap: map[string][]models.DedicatedAICluster{}}}
	m.pendingTasks = 0
	cmd = m.handleDedicatedAIClusterCategory()
	assert.NotNil(t, cmd)
	assert.Equal(t, 0, m.pendingTasks)
}

func TestEnterContext(t *testing.T) {
	t.Parallel()
	m := &Model{
		table:    &table.Model{},
		category: domain.Tenant,
	}
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
