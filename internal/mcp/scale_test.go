package mcp

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/infra/oci"
	"github.com/jingle2008/toolkit/internal/mutation/ops"
	"github.com/jingle2008/toolkit/pkg/models"
)

// stubScalePool resolves every pool to pool and records the count a
// scale-up adds (-1 when it never ran).
func stubScalePool(d *ops.Deps, pool models.GPUPool) *int {
	stubPerformSeams(d)
	gotAdd := -1
	d.ResolveGPUPool = func(context.Context, models.Environment, string) (*models.GPUPool, error) {
		p := pool
		return &p, nil
	}
	d.IncreasePoolSize = func(_ context.Context, p *models.GPUPool, add int, _ models.Environment, _ *slog.Logger) (oci.PoolScale, error) {
		gotAdd = add
		return oci.PoolScale{Shown: p.ActualSize, Before: p.ActualSize, After: p.ActualSize + add}, nil
	}
	return &gotAdd
}

// add goes past the Terraform size; the result reports the sizes and
// warns that terraform apply reverts it.
func TestScaleGPUPool_AddPastTerraformSize(t *testing.T) {
	t.Parallel()
	d := &ops.Deps{}
	gotAdd := stubScalePool(d, models.GPUPool{Name: "pool-a", ID: "ocid1.pool", Size: 4, ActualSize: 4})
	sc := structured(t, callToolWith(t, d, "scale_gpu_pool", map[string]any{"name": "pool-a", "add": 2, "confirm": true}))
	assert.Equal(t, 2, *gotAdd)
	assert.Equal(t, "OK", sc["status"])
	assert.Equal(t, map[string]any{"shown": float64(4), "before": float64(4), "after": float64(6)}, sc["scale"])
	assert.Contains(t, sc["note"], "4 → 6 instances")
	assert.Contains(t, sc["note"], "the next terraform apply shrinks it back")
}

// Without add, a short pool is filled from OCI's size at the time, not
// by the shortfall the lookup saw, which may be stale.
func TestScaleGPUPool_WithoutAddFills(t *testing.T) {
	t.Parallel()
	d := &ops.Deps{}
	gotAdd := stubScalePool(d, models.GPUPool{Name: "pool-a", ID: "ocid1.pool", Size: 6, ActualSize: 4})
	sc := structured(t, callToolWith(t, d, "scale_gpu_pool", map[string]any{"name": "pool-a", "confirm": true}))
	assert.Equal(t, oci.FillPool, *gotAdd)
	assert.Equal(t, "OK", sc["status"])
}

// Without add, a pool at its Terraform size is left alone: status NOOP
// says how to go past it, and nothing reaches OCI.
func TestScaleGPUPool_AtSizeWithoutAddIsNoop(t *testing.T) {
	t.Parallel()
	d := &ops.Deps{}
	gotAdd := stubScalePool(d, models.GPUPool{Name: "pool-a", ID: "ocid1.pool", Size: 4, ActualSize: 4})
	sc := structured(t, callToolWith(t, d, "scale_gpu_pool", map[string]any{"name": "pool-a", "confirm": true}))
	assert.Equal(t, -1, *gotAdd, "must not scale")
	assert.Equal(t, "NOOP", sc["status"])
	assert.Contains(t, sc["note"], "already at its Terraform size (4); nothing to scale. Pass add N")
}

func TestScaleGPUPool_Refusals(t *testing.T) {
	t.Parallel()
	d := &ops.Deps{}
	gotAdd := stubScalePool(d, models.GPUPool{Name: "pool-a", ID: "ocid1.pool", Size: 4, ActualSize: 4})
	res := callToolWith(t, d, "scale_gpu_pool", map[string]any{"name": "pool-a", "add": 0, "confirm": true})
	require.True(t, res.IsError, "an explicit add=0 isn't a scale-up")
	assert.Equal(t, -1, *gotAdd)

	gotAdd = stubScalePool(d, models.GPUPool{Name: "pool-a", Size: 4, Status: models.GPUPoolStatusNonexistent})
	res = callToolWith(t, d, "scale_gpu_pool", map[string]any{"name": "pool-a", "add": 1, "confirm": true})
	require.True(t, res.IsError, "a pool OCI lacks")
	assert.Equal(t, -1, *gotAdd)
}
