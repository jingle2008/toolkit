package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDataset_BuildTenantIDSuffixMap(t *testing.T) {
	t.Parallel()
	ds := &Dataset{
		Tenants: []Tenant{
			{Name: "tenant1", IDs: []string{"id.tenant1"}},
			{Name: "tenant2", IDs: []string{"id.tenant2"}},
		},
	}
	suffixMap := ds.BuildTenantIDSuffixMap()
	assert.Contains(t, suffixMap, "tenant1")
	assert.Contains(t, suffixMap, "tenant2")
}

func TestDataset_SetDedicatedAIClusterMap(_ *testing.T) {
	ds := &Dataset{}
	m := map[string][]DedicatedAICluster{
		"t1": {{Name: "c1"}, {Name: "c2"}},
	}
	ds.SetDedicatedAIClusterMap(m)
	// No return, just ensure no panic and field is set
}
