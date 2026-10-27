package ops

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/infra/k8s"
	"github.com/jingle2008/toolkit/internal/infra/oci"
	"github.com/jingle2008/toolkit/internal/logging"
	"github.com/jingle2008/toolkit/internal/mutation"
	"github.com/jingle2008/toolkit/pkg/models"
)

// run performs op through mutation.Run, ungated.
func run(t *testing.T, op mutation.Op) mutation.Plan {
	t.Helper()
	p, outcome, err := mutation.Run(context.Background(), slog.New(logging.NewRingSink(8)), "test", mutation.Gate{}, op)
	require.NoError(t, err)
	assert.Equal(t, mutation.Performed, outcome)
	return p
}

// Given an OCID, a node action targets it and audits the node beside it.
func TestNodeAction_OCIDTarget(t *testing.T) {
	t.Parallel()
	var acted *models.GPUNode
	d := Deps{
		ResolveGPUNode: func(_ context.Context, _ models.Environment, name, ocid string) (*models.GPUNode, error) {
			assert.Equal(t, "node-1", name)
			assert.Equal(t, "ocid1.instance.x", ocid)
			return &models.GPUNode{ID: ocid, NodePool: "pool-a"}, nil
		},
		Terminate: func(_ context.Context, n *models.GPUNode, _ models.Environment, _ *slog.Logger) error {
			acted = n
			return nil
		},
	}
	op := TerminateNode(d, Env{}, "node-1", "ocid1.instance.x")
	assert.Equal(t, "ocid1.instance.x", op.Plan.Target)
	assert.Equal(t, []any{"node", "node-1"}, op.AuditFields())

	p := run(t, op)
	require.NotNil(t, acted)
	assert.Equal(t, "ocid1.instance.x", acted.ID)
	assert.Equal(t, mutation.ResolvedPoolInstance("ocid1.instance.x", "pool-a"), p.Resolved)

	byName := RestartNode(d, Env{}, "node-1", "")
	assert.Equal(t, "node-1", byName.Plan.Target)
	assert.Nil(t, byName.AuditFields)
}

func TestScalePool(t *testing.T) {
	t.Parallel()
	pool := &models.GPUPool{Name: "pool-a", ID: "ocid1.pool", Size: 4, ActualSize: 4}
	var asked int
	d := Deps{
		ResolveGPUPool: func(context.Context, models.Environment, string) (*models.GPUPool, error) { return pool, nil },
		IncreasePoolSize: func(_ context.Context, _ *models.GPUPool, add int, _ models.Environment, _ *slog.Logger) (oci.PoolScale, error) {
			asked = add
			return oci.PoolScale{Before: 4, After: 6, Shown: 4}, nil
		},
	}

	op, scale := ScalePool(d, Env{}, "pool-a", 2, true, "--add")
	assert.Empty(t, scale.Sizes(), "nothing performed yet")
	assert.Equal(t, []any{"add", 0}, op.AuditFields())
	run(t, op)
	assert.Equal(t, 2, asked)
	assert.Equal(t, []any{"add", 2, "from", 4, "to", 6}, op.AuditFields())
	assert.Equal(t, "4 → 6 instances", scale.Sizes())
	assert.Contains(t, scale.Summary(), "4 → 6 instances. Terraform declares 4")

	// Filling a pool already at its Terraform size is a noop, named by
	// the front-end's own option.
	op, _ = ScalePool(d, Env{}, "pool-a", 0, false, "add")
	p, outcome, err := mutation.Run(context.Background(), slog.New(logging.NewRingSink(8)), "test", mutation.Gate{}, op)
	require.NoError(t, err)
	assert.Equal(t, mutation.Noop, outcome)
	assert.Contains(t, p.Noop, "add")
}

func TestScalePool_NoInstancePool(t *testing.T) {
	t.Parallel()
	d := Deps{ResolveGPUPool: func(context.Context, models.Environment, string) (*models.GPUPool, error) {
		return &models.GPUPool{Name: "pool-a"}, nil
	}}
	op, _ := ScalePool(d, Env{}, "pool-a", 1, true, "--add")
	_, _, err := mutation.Run(context.Background(), slog.New(logging.NewRingSink(8)), "test", mutation.Gate{}, op)
	require.ErrorIs(t, err, oci.ErrNoInstancePool)
}

// The orphan delete resolves the namespace into the target and picks
// force-delete for a deletion stuck on its finalizer.
func TestDeleteOrphanISVC(t *testing.T) {
	t.Parallel()
	d := Deps{
		ResolveISVC: func(context.Context, string, string, string, string) (string, error) { return "ns1", nil },
		PlanOrphanISVC: func(context.Context, string, string, string, string) (k8s.OrphanDeleteOutcome, error) {
			return k8s.OrphanFinalized, nil
		},
		DeleteOrphanISVC: func(_ context.Context, _, _, ns, name string) (k8s.OrphanDeleteOutcome, error) {
			assert.Equal(t, "ns1", ns)
			assert.Equal(t, "svc", name)
			return k8s.OrphanFinalized, nil
		},
	}
	op, done := DeleteOrphanISVC(d, Env{}, "", "svc")
	assert.Equal(t, "svc", op.Plan.Target)
	p := run(t, op)
	assert.Equal(t, mutation.ForceDeleteInferenceService, p.Action)
	assert.Equal(t, "ns1/svc", p.Target)
	assert.Equal(t, k8s.OrphanFinalized, *done)
}

func TestTenantEntry(t *testing.T) {
	t.Parallel()
	internal := false
	entry, err := TenantEntry("ocid1.tenancy.oc1..a", "Acme", &internal, "vip")
	require.NoError(t, err)
	assert.Equal(t, "Acme", *entry.Name)
	assert.False(t, *entry.IsInternal)
	assert.Equal(t, "vip", *entry.Note)

	entry, err = TenantEntry("ocid1.tenancy.oc1..a", "Acme", nil, "")
	require.NoError(t, err)
	assert.Nil(t, entry.IsInternal, "nil keeps the recorded flag")
	assert.Nil(t, entry.Note)

	_, err = TenantEntry("ocid1.instance.oc1..a", "Acme", nil, "")
	require.ErrorContains(t, err, `must start with "ocid1.tenancy."`)
	_, err = TenantEntry("ocid1.tenancy.oc1..a", " ", nil, "")
	require.Error(t, err)
}

// A failed write is the op's error; the lookup's checks run first.
func TestSetTenant(t *testing.T) {
	t.Parallel()
	boom := errors.New("read-only")
	entry := models.TenantMetadata{ID: "ocid1.tenancy.oc1..a"}
	op := SetTenant(Deps{
		CheckTenant:  func(models.TenantMetadata) (bool, error) { return true, nil },
		UpsertTenant: func(context.Context, models.TenantMetadata) error { return boom },
	}, entry)
	p, _, err := mutation.Run(context.Background(), slog.New(logging.NewRingSink(8)), "test", mutation.Gate{}, op)
	require.ErrorIs(t, err, boom)
	assert.Equal(t, mutation.ResolvedTenant(true), p.Resolved)
}
