package mcp

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/infra/oci"
	"github.com/jingle2008/toolkit/pkg/models"
)

// stubScalePool resolves every pool to pool and records the count a
// scale-up adds (-1 when it never ran).
func stubScalePool(t *testing.T, pool models.GPUPool) *int {
	t.Helper()
	stubPerformSeams(t)
	gotAdd := -1
	mcpResolveGPUPoolFn = func(context.Context, *Server, models.Environment, string) (*models.GPUPool, error) {
		p := pool
		return &p, nil
	}
	mcpIncreasePoolSizeFn = func(_ context.Context, p *models.GPUPool, add int, _ models.Environment, _ *slog.Logger) (oci.PoolScale, error) {
		gotAdd = add
		return oci.PoolScale{Shown: p.ActualSize, Before: p.ActualSize, After: p.ActualSize + add}, nil
	}
	return &gotAdd
}

// add goes past the Terraform size; the result reports the sizes and
// warns that terraform apply reverts it.
//
//nolint:paralleltest // replaces package-level seams
func TestScaleGPUPool_AddPastTerraformSize(t *testing.T) {
	gotAdd := stubScalePool(t, models.GPUPool{Name: "pool-a", ID: "ocid1.pool", Size: 4, ActualSize: 4})
	sc := structured(t, callTool(t, "scale_gpu_pool", map[string]any{"name": "pool-a", "add": 2, "confirm": true}))
	assert.Equal(t, 2, *gotAdd)
	assert.Equal(t, "OK", sc["status"])
	assert.Equal(t, map[string]any{"shown": float64(4), "before": float64(4), "after": float64(6)}, sc["scale"])
	assert.Contains(t, sc["note"], "4 → 6 instances")
	assert.Contains(t, sc["note"], "the next terraform apply shrinks it back")
}

// Without add, a short pool is filled from OCI's size at the time, not
// by the shortfall the lookup saw, which may be stale.
//
//nolint:paralleltest // replaces package-level seams
func TestScaleGPUPool_WithoutAddFills(t *testing.T) {
	gotAdd := stubScalePool(t, models.GPUPool{Name: "pool-a", ID: "ocid1.pool", Size: 6, ActualSize: 4})
	sc := structured(t, callTool(t, "scale_gpu_pool", map[string]any{"name": "pool-a", "confirm": true}))
	assert.Equal(t, oci.FillPool, *gotAdd)
	assert.Equal(t, "OK", sc["status"])
}

// Without add, a pool at its Terraform size is left alone: status NOOP
// says how to go past it, and nothing reaches OCI.
//
//nolint:paralleltest // replaces package-level seams
func TestScaleGPUPool_AtSizeWithoutAddIsNoop(t *testing.T) {
	gotAdd := stubScalePool(t, models.GPUPool{Name: "pool-a", ID: "ocid1.pool", Size: 4, ActualSize: 4})
	sc := structured(t, callTool(t, "scale_gpu_pool", map[string]any{"name": "pool-a", "confirm": true}))
	assert.Equal(t, -1, *gotAdd, "must not scale")
	assert.Equal(t, "NOOP", sc["status"])
	assert.Contains(t, sc["note"], "already at its Terraform size (4); nothing to scale. Pass add N")
}

//nolint:paralleltest // replaces package-level seams
func TestScaleGPUPool_Refusals(t *testing.T) {
	gotAdd := stubScalePool(t, models.GPUPool{Name: "pool-a", ID: "ocid1.pool", Size: 4, ActualSize: 4})
	res := callTool(t, "scale_gpu_pool", map[string]any{"name": "pool-a", "add": 0, "confirm": true})
	require.True(t, res.IsError, "an explicit add=0 isn't a scale-up")
	assert.Equal(t, -1, *gotAdd)

	gotAdd = stubScalePool(t, models.GPUPool{Name: "pool-a", Size: 4, Status: models.GPUPoolStatusNonexistent})
	res = callTool(t, "scale_gpu_pool", map[string]any{"name": "pool-a", "add": 1, "confirm": true})
	require.True(t, res.IsError, "a pool OCI lacks")
	assert.Equal(t, -1, *gotAdd)
}
