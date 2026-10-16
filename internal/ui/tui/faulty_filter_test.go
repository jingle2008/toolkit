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

	rows, _ := computeTableRows(ds, domain.InferenceService, nil, "", "", true, true)
	assert.Equal(t, []string{"down", "orphan"}, rowNames(rows))

	// DACs not loaded yet: a missing DAC may just be unloaded.
	ds.DedicatedAIClusterMap = nil
	rows, _ = computeTableRows(ds, domain.InferenceService, nil, "", "", true, true)
	assert.Equal(t, []string{"down"}, rowNames(rows))
}

// A node in a pool missing from terraform is faulty, even when the pool
// may only be unloaded (a pool source failed): a broken source should
// catch attention as much as a missing pool.
func TestFaultyFilter_NodeInMissingPool(t *testing.T) {
	t.Parallel()
	ds := &models.Dataset{
		GPUPools: []models.GPUPool{{Name: "pool-a"}},
		GPUNodeMap: map[string][]models.GPUNode{
			"pool-a": {{Name: "node-a", InstanceType: "BM.GPU4.8", Allocatable: 8, IsReady: true}},
			"pool-x": {{Name: "node-x", InstanceType: "BM.GPU4.8", Allocatable: 8, IsReady: true}},
		},
	}
	rows, _ := computeTableRows(ds, domain.GPUNode, nil, "", "", true, true)
	assert.Equal(t, []string{"node-x"}, rowNames(rows))
}
