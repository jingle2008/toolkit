package models

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDataset_MergeReloadedRepoData_PreservesK8sFields(t *testing.T) {
	d := &Dataset{
		Tenants:    []Tenant{{Name: "old"}},
		BaseModels: []BaseModel{{Name: "bm1"}},
		GPUPools:   []GPUPool{{Name: "p1"}},
		GPUNodeMap: map[string][]GPUNode{"p1": {{Name: "n1"}}},
	}
	bm := d.BaseModels
	pools := d.GPUPools
	nodes := d.GPUNodeMap

	fresh := &Dataset{
		Tenants:      []Tenant{{Name: "new1"}, {Name: "new2"}},
		Environments: []Environment{{Type: "dev"}},
		// k8s-backed fields left nil, as LoadDataset returns them
	}

	d.MergeReloadedRepoData(fresh)

	// Repo-owned fields are replaced by the freshly loaded values.
	require.Len(t, d.Tenants, 2)
	require.Equal(t, "new1", d.Tenants[0].Name)
	require.Len(t, d.Environments, 1)

	// Lazily-loaded k8s fields are preserved untouched.
	require.Equal(t, bm, d.BaseModels)
	require.Equal(t, pools, d.GPUPools)
	require.Equal(t, nodes, d.GPUNodeMap)
}
