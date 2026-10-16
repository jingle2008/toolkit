package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/pkg/models"
)

// Ctrl+Z lists rows naming a parent that doesn't exist alongside rows
// whose own state is faulty: an endpoint whose DAC is gone reports Ready
// (the live case on dp-prod-ord) but serves nothing.
func TestFaultyFilter_Dangling(t *testing.T) {
	t.Parallel()
	ds := &models.Dataset{
		DedicatedAIClusterMap: map[string][]models.DedicatedAICluster{"t": {{Name: "amaaaaaa-dac1"}, {Name: "amaaaaaa-dac2"}}},
		InferenceServiceMap: map[string][]models.InferenceService{
			"amaaaaaa-dac1": {{Name: "ok", Namespace: "amaaaaaa-dac1", Type: models.InferenceServiceDedicated, Ready: "True"}},
			"amaaaaaa-gone": {{Name: "orphan", Namespace: "amaaaaaa-gone", Type: models.InferenceServiceDedicated, Ready: "True"}},
			"amaaaaaa-dac2": {{Name: "down", Namespace: "amaaaaaa-dac2", Type: models.InferenceServiceDedicated, Ready: "False"}},
		},
	}
	on := faultyFilter{on: true}

	rows, _ := computeTableRows(ds, domain.InferenceService, nil, "", "", true, on)
	assert.Equal(t, []string{"down", "orphan"}, rowNames(rows))

	// DACs not loaded yet: a missing DAC may just be unloaded.
	ds.DedicatedAIClusterMap = nil
	rows, _ = computeTableRows(ds, domain.InferenceService, nil, "", "", true, on)
	assert.Equal(t, []string{"down"}, rowNames(rows))
}

func TestFaultyFilter_PartialGPUPools(t *testing.T) {
	t.Parallel()
	ds := &models.Dataset{
		GPUPools:   []models.GPUPool{{Name: "pool-a"}},
		GPUNodeMap: map[string][]models.GPUNode{"pool-x": {{Name: "node-x", InstanceType: "BM.GPU4.8", Allocatable: 8, IsReady: true}}},
	}
	rows, _ := computeTableRows(ds, domain.GPUNode, nil, "", "", true, faultyFilter{on: true})
	assert.Len(t, rows, 1, "pool-x isn't in GPUPools")
	rows, _ = computeTableRows(ds, domain.GPUNode, nil, "", "", true, faultyFilter{on: true, partial: []domain.Category{domain.GPUPool}})
	assert.Empty(t, rows, "pools loaded partially: pool-x may just be unloaded")
}

// A partial pool load marks pools partial for the gate until a full
// reload or an environment switch.
func TestModel_FaultyFilterTracksPartialPools(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.showFaulty = true
	m.handleGPUPoolsLoaded([]models.GPUPool{{Name: "pool-a"}}, true, m.gens.msg)
	assert.Equal(t, faultyFilter{on: true, partial: []domain.Category{domain.GPUPool}}, m.faultyFilter())

	m.handleGPUPoolsReloaded(gpuPoolsReloadedMsg{Items: []models.GPUPool{{Name: "pool-a"}}})
	assert.Equal(t, faultyFilter{on: true}, m.faultyFilter())

	m.handleGPUPoolsLoaded([]models.GPUPool{{Name: "pool-a"}}, true, m.gens.msg)
	m.switchEnvironment(m.environment, domain.Tenant)
	assert.Empty(t, m.faultyFilter().partial)
}
