package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDedicatedAICluster_Getters(t *testing.T) {
	t.Parallel()
	cluster := DedicatedAICluster{
		Name:      "cluster1",
		Type:      "A100",
		UnitShape: "shapeA",
		Status:    "Ready",
		TenantID:  "tenant1",
	}
	assert.Equal(t, "cluster1", cluster.GetName())
	assert.ElementsMatch(t, []string{"cluster1", "A100", "shapeA", "Ready", "tenant1", "", "", "", ""}, cluster.GetFilterableFields())
	assert.Equal(t, "A100-shapeA-cluster1", cluster.GetKey())
}
