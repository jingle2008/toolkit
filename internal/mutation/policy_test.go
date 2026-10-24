package mutation

import (
	"errors"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/pkg/infra/logging"
	"github.com/jingle2008/toolkit/pkg/models"
)

// The tier table every surface gates on: irreversible actions need the
// strongest confirmation (TUI capital Y, CLI --yes, MCP confirm_target).
func TestActions_Tiers(t *testing.T) {
	t.Parallel()
	irreversible := map[Action]bool{
		TerminateNode:               true,
		DeleteDAC:                   true,
		DeleteInferenceService:      true,
		ForceDeleteInferenceService: true,
	}
	seen := map[[2]string]bool{}
	for _, a := range Actions() {
		key := [2]string{a.Verb, a.Kind}
		assert.False(t, seen[key], "%s listed twice", a)
		seen[key] = true
		assert.Equal(t, irreversible[a], a.Tier == Irreversible, "%s tier", a)
	}
	assert.Len(t, Actions(), 11)
}

func TestLookup(t *testing.T) {
	t.Parallel()
	a, ok := Lookup("force-delete", "inference_service")
	require.True(t, ok)
	assert.Equal(t, ForceDeleteInferenceService, a)
	_, ok = Lookup("delete", "node")
	assert.False(t, ok)
}

func TestInferenceServiceDelete(t *testing.T) {
	t.Parallel()
	assert.Equal(t, DeleteInferenceService, InferenceServiceDelete(false))
	assert.Equal(t, ForceDeleteInferenceService, InferenceServiceDelete(true))
}

func TestAction_String(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "terminate node", TerminateNode.String())
}

func fields(e logging.Entry) map[string]any {
	m := map[string]any{}
	for i := 0; i+1 < len(e.Fields); i += 2 {
		m[e.Fields[i].(string)] = e.Fields[i+1] //nolint:forcetypeassert // keys are strings
	}
	return m
}

// The record keeps the field names the CLI and MCP have always written,
// so existing log queries (msg=mutation, phase=…) keep working.
func TestAudit_Record(t *testing.T) {
	t.Parallel()
	sink := logging.NewRingSink(8)
	Audit(sink, "cli", TerminateNode, "n1", PhaseBegin, nil)
	Audit(sink, "mcp", DeleteDAC, "d1", PhaseFailed, errors.New("boom"))
	Audit(sink, "cli", CordonNode, "n1", PhaseDryRun, nil)

	got := sink.Snapshot()
	require.Len(t, got, 3)
	for _, e := range got {
		assert.Equal(t, "mutation", e.Message)
	}
	assert.Equal(t, logging.LevelInfo, got[0].Level)
	assert.Equal(t, map[string]any{
		"action": "terminate", "kind": "node", "target": "n1", "surface": "cli",
		"phase": "begin", "dry_run": false,
	}, fields(got[0]))

	assert.Equal(t, logging.LevelError, got[1].Level)
	f := fields(got[1])
	assert.Equal(t, "failed", f["phase"])
	assert.Equal(t, "mcp", f["surface"])
	assert.EqualError(t, f["error"].(error), "boom") //nolint:forcetypeassert // asserted by EqualError

	assert.Equal(t, true, fields(got[2])["dry_run"])
	assert.Equal(t, "dry_run", fields(got[2])["phase"])
}

func TestResolved(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "actual_size=1, add=3, pool_id=p, size=2, target_size=4", ResolvedPool("p", 2, 1, 3).String())
	assert.Equal(t, "instance_id=i", ResolvedNode("i").String())
	assert.Equal(t, "gpu_pool=p, instance_id=i", ResolvedPoolInstance("i", "p").String())
	assert.Equal(t, "namespace=ns", ResolvedNamespace("ns").String())
	assert.Empty(t, Resolved(nil).String())
}

// Without a count only the shortfall is added, so a full pool needs an
// explicit one to go past its declared size.
func TestPoolScaleAdd(t *testing.T) {
	t.Parallel()
	short, full := models.GPUPool{Size: 4, ActualSize: 1}, models.GPUPool{Size: 4, ActualSize: 4}
	for _, tc := range []struct {
		name      string
		pool      models.GPUPool
		requested int
		given     bool
		want      int
	}{
		{"short, no count fills", short, 0, false, 3},
		{"full, no count does nothing", full, 0, false, 0},
		{"full, count goes past", full, 2, true, 2},
		{"short, count overrides the fill", short, 5, true, 5},
	} {
		got, err := PoolScaleAdd(tc.pool, tc.requested, tc.given)
		require.NoError(t, err, tc.name)
		assert.Equal(t, tc.want, got, tc.name)
	}
	_, err := PoolScaleAdd(full, 0, true)
	require.Error(t, err, "an explicit 0 isn't a scale-up")
	_, err = PoolScaleAdd(full, -1, true)
	require.Error(t, err)
}

// One scale-up adds at most PoolScaleAddLimit instances, or the pool's
// shortfall when that is more; a huge count is refused rather than
// overflowing into a negative size.
func TestCheckPoolScaleAdd_Limit(t *testing.T) {
	t.Parallel()
	full := models.GPUPool{Size: 4, ActualSize: 4}
	require.NoError(t, CheckPoolScaleAdd(full, PoolScaleAddLimit))
	require.ErrorContains(t, CheckPoolScaleAdd(full, PoolScaleAddLimit+1), "the most one scale-up adds")
	_, err := PoolScaleAdd(full, math.MaxInt, true)
	require.ErrorContains(t, err, "the most one scale-up adds")

	empty := models.GPUPool{Size: 150, ActualSize: 0}
	require.NoError(t, CheckPoolScaleAdd(empty, 150), "filling to the declared size is one step")
	require.Error(t, CheckPoolScaleAdd(empty, 151))
}

func TestPoolScaleNotice(t *testing.T) {
	t.Parallel()
	pool := models.GPUPool{Name: "p", Size: 4, ActualSize: 2}
	assert.Equal(t, "Grows the instance pool from 2 to 4 instances.", PoolScaleNotice(pool, 2))
	notice := PoolScaleNotice(pool, 3)
	assert.Contains(t, notice, "from 2 to 5 instances")
	assert.Contains(t, notice, "Terraform declares 4; the next terraform apply shrinks it back")
	assert.Empty(t, PoolOverSizeWarning(4, 4))
}

// A mutation's own fields follow the standard ones in its record.
func TestAudit_Fields(t *testing.T) {
	t.Parallel()
	sink := logging.NewRingSink(2)
	Audit(sink, "cli", ScaleGPUPool, "p", PhaseDone, nil, "add", 2, "to", 6)
	got := sink.Snapshot()
	require.Len(t, got, 1)
	f := fields(got[0])
	assert.Equal(t, "p", f["target"])
	assert.Equal(t, 2, f["add"])
	assert.Equal(t, 6, f["to"])
}
