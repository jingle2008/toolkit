package mutation

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/logging"
)

// runAudited runs op through gate and returns the phases audited, in
// order, beside Run's results.
func runAudited(t *testing.T, gate Gate, op Op) (Plan, Outcome, []string, error) {
	t.Helper()
	sink := logging.NewRingSink(16)
	p, outcome, err := Run(context.Background(), slog.New(sink), "test", gate, op)
	entries := sink.Snapshot()
	phases := make([]string, 0, len(entries))
	for _, e := range entries {
		phases = append(phases, fields(e)["phase"].(string))
	}
	return p, outcome, phases, err
}

func TestRun_Phases(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	ok := func(context.Context, Plan) error { return nil }
	fail := func(context.Context, Plan) error { return boom }
	lost := func(context.Context, *Plan) error { return boom }
	plan := Plan{Action: DrainNode, Target: "n1"}

	cases := []struct {
		name    string
		gate    Gate
		op      Op
		outcome Outcome
		err     error
		phases  []string
	}{
		{"performed", Gate{}, Op{Plan: plan, Perform: ok}, Performed, nil, []string{"begin", "done"}},
		{"perform fails", Gate{}, Op{Plan: plan, Perform: fail}, Performed, boom, []string{"begin", "failed"}},
		{"dry run", Gate{DryRun: true}, Op{Plan: plan, Perform: fail}, DryRun, nil, []string{"dry_run"}},
		{"resolve fails", Gate{}, Op{Plan: plan, Resolve: lost, Perform: ok}, Performed, boom, []string{"failed"}},
		{"resolve fails, dry run", Gate{DryRun: true}, Op{Plan: plan, Resolve: lost, Perform: ok}, Performed, boom, nil},
		{
			"not admitted",
			Gate{Admit: func(Plan) error { return Refuse(boom) }},
			Op{Plan: plan, Resolve: lost, Perform: fail},
			Performed, boom,
			[]string{"refused"},
		},
		{
			"refused at confirm",
			Gate{Confirm: func(context.Context, Plan) error { return Refuse(boom) }},
			Op{Plan: plan, Perform: fail},
			Performed, boom,
			[]string{"refused"},
		},
		{
			// The prompt couldn't be read: not a decision, so not audited.
			"confirm error",
			Gate{Confirm: func(context.Context, Plan) error { return boom }},
			Op{Plan: plan, Perform: fail},
			Performed, boom, nil,
		},
		{
			"dry run skips confirm",
			Gate{DryRun: true, Confirm: func(context.Context, Plan) error { return Refuse(boom) }},
			Op{Plan: plan, Perform: fail},
			DryRun, nil,
			[]string{"dry_run"},
		},
		{
			"noop",
			Gate{Confirm: func(context.Context, Plan) error { return Refuse(boom) }},
			Op{Plan: plan, Resolve: func(_ context.Context, p *Plan) error { p.Noop = "at size"; return nil }, Perform: fail},
			Noop, nil, nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, outcome, phases, err := runAudited(t, tc.gate, tc.op)
			assert.Equal(t, tc.outcome, outcome)
			if tc.err == nil {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, tc.err)
			}
			if tc.phases == nil {
				assert.Empty(t, phases)
			} else {
				assert.Equal(t, tc.phases, phases)
			}
		})
	}
}

// What Resolve refines is what the gate checks, Perform acts on, and the
// later audit records name.
func TestRun_ResolveRefinesPlan(t *testing.T) {
	t.Parallel()
	sink := logging.NewRingSink(8)
	var confirmed, performed Plan
	p, _, err := Run(context.Background(), slog.New(sink), "test", Gate{
		Confirm: func(_ context.Context, p Plan) error { confirmed = p; return nil },
	}, Op{
		Plan: Plan{Action: DeleteInferenceService, Target: "svc"},
		Resolve: func(_ context.Context, p *Plan) error {
			p.Action, p.Target = ForceDeleteInferenceService, "ns/svc"
			p.Resolved = ResolvedNamespace("ns")
			return nil
		},
		Perform:     func(_ context.Context, p Plan) error { performed = p; return nil },
		AuditFields: func() []any { return []any{"extra", 1} },
	})
	require.NoError(t, err)
	assert.Equal(t, p, confirmed)
	assert.Equal(t, p, performed)
	assert.Equal(t, "force-delete inference_service/ns/svc", p.String())

	got := sink.Snapshot()
	require.Len(t, got, 2)
	for _, e := range got {
		f := fields(e)
		assert.Equal(t, "force-delete", f["action"])
		assert.Equal(t, "ns/svc", f["target"])
		assert.Equal(t, int64(1), f["extra"])
	}
}
