package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/config"
	"github.com/jingle2008/toolkit/internal/logging"
	"github.com/jingle2008/toolkit/internal/mutation"
)

func TestConfirmAction(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"yes", "y\n", true},
		{"yes-long", "yes\n", true},
		{"yes-upper", "YES\n", true},
		{"yes-pad", "  y  \n", true},
		{"no", "n\n", false},
		{"empty", "\n", false},
		{"eof", "", false},
		{"garbage", "maybe\n", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var out bytes.Buffer
			got, err := confirmAction(t.Context(), strings.NewReader(tc.in), &out, "Confirm? ")
			if err != nil {
				t.Fatalf("confirmAction: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
			if !strings.Contains(out.String(), "Confirm? ") {
				t.Errorf("prompt missing from output: %q", out.String())
			}
		})
	}
}

// Ctrl+C at the prompt cancels the command's context; the prompt must
// give up then rather than wait for a line that never comes.
func TestConfirmAction_CancelledWhileWaiting(t *testing.T) {
	t.Parallel()
	pr, pw := io.Pipe()
	defer func() { _ = pw.Close() }()
	ctx, cancel := context.WithCancel(t.Context())
	var out bytes.Buffer
	done := make(chan error, 1)
	go func() {
		_, err := confirmAction(ctx, pr, &out, "Confirm? ")
		done <- err
	}()
	cancel()
	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(5 * time.Second):
		t.Fatal("confirmAction kept waiting after ctx was cancelled")
	}
}

// testPlan is a test's mutation: the op runPlan builds, and its flags.
type testPlan struct {
	Action  mutation.Action
	Target  string
	DryRun  bool
	Yes     bool
	Resolve func(context.Context, *mutation.Plan) error
}

// runPlan runs plan, performing with perform, through runMutation.
func runPlan(ctx context.Context, in io.Reader, out io.Writer, plan testPlan, perform func(context.Context) error) error {
	return runMutation(ctx, in, out, cliGate{DryRun: plan.DryRun, Yes: plan.Yes}, mutation.Op{
		Plan:    mutation.Plan{Action: plan.Action, Target: plan.Target},
		Resolve: plan.Resolve,
		Perform: func(ctx context.Context, _ mutation.Plan) error { return perform(ctx) },
	})
}

func TestRunMutation_DryRun(t *testing.T) {
	t.Parallel()
	plan := testPlan{Action: mutation.CordonNode, Target: "n1", DryRun: true}
	called := false
	var out bytes.Buffer
	err := runPlan(context.Background(), strings.NewReader(""), &out, plan, func(context.Context) error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatalf("runMutation: %v", err)
	}
	if called {
		t.Fatal("perform must not run on --dry-run")
	}
	if !strings.Contains(out.String(), "DRY-RUN: would cordon node/n1") {
		t.Errorf("expected DRY-RUN line, got: %q", out.String())
	}
}

func TestRunMutation_YesSkipsPrompt(t *testing.T) {
	t.Parallel()
	plan := testPlan{Action: mutation.CordonNode, Target: "n1", Yes: true}
	called := false
	var out bytes.Buffer
	// Stdin reader intentionally empty — Yes must not consume it.
	err := runPlan(context.Background(), strings.NewReader(""), &out, plan, func(context.Context) error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatalf("runMutation: %v", err)
	}
	if !called {
		t.Fatal("perform must run when --yes is set")
	}
	if !strings.Contains(out.String(), "cordon node/n1: OK") {
		t.Errorf("expected OK line, got: %q", out.String())
	}
	if strings.Contains(out.String(), "Confirm") {
		t.Errorf("--yes must not show prompt, got: %q", out.String())
	}
}

func TestRunMutation_InteractiveBail(t *testing.T) {
	t.Parallel()
	plan := testPlan{Action: mutation.CordonNode, Target: "n1"}
	called := false
	var out bytes.Buffer
	err := runPlan(context.Background(), strings.NewReader("n\n"), &out, plan, func(context.Context) error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatalf("runMutation must return nil on bail: %v", err)
	}
	if called {
		t.Fatal("perform must not run after user types n")
	}
	if !strings.Contains(out.String(), "Confirm cordon node/n1?") {
		t.Errorf("expected prompt, got: %q", out.String())
	}
	if !strings.Contains(out.String(), "aborted") {
		t.Errorf("expected aborted line, got: %q", out.String())
	}
}

func TestRunMutation_InteractiveYes(t *testing.T) {
	t.Parallel()
	plan := testPlan{Action: mutation.CordonNode, Target: "n1"}
	called := false
	var out bytes.Buffer
	err := runPlan(context.Background(), strings.NewReader("y\n"), &out, plan, func(context.Context) error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatalf("runMutation: %v", err)
	}
	if !called {
		t.Fatal("perform must run after user types y")
	}
}

func TestRunMutation_Irreversible_NoPrompt(t *testing.T) {
	t.Parallel()
	plan := testPlan{
		Action: mutation.TerminateNode, Target: "n1",
	}
	called := false
	var out bytes.Buffer
	// Stdin says "y" — should still fail because an irreversible action
	// blocks interactive consent.
	err := runPlan(context.Background(), strings.NewReader("y\n"), &out, plan, func(context.Context) error {
		called = true
		return nil
	})
	if err == nil {
		t.Fatal("expected error when an irreversible action without --yes")
	}
	if !strings.Contains(err.Error(), "--yes") {
		t.Errorf("error should mention --yes, got: %v", err)
	}
	if called {
		t.Fatal("perform must not run")
	}
}

func TestRunMutation_Irreversible_PassesWithYes(t *testing.T) {
	t.Parallel()
	plan := testPlan{
		Action: mutation.TerminateNode, Target: "n1", Yes: true,
	}
	called := false
	var out bytes.Buffer
	err := runPlan(context.Background(), strings.NewReader(""), &out, plan, func(context.Context) error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatalf("runMutation: %v", err)
	}
	if !called {
		t.Fatal("perform must run for an irreversible action with --yes")
	}
}

func TestRunMutation_PerformError(t *testing.T) {
	t.Parallel()
	plan := testPlan{Action: mutation.CordonNode, Target: "n1", Yes: true}
	want := errors.New("kube unreachable")
	var out bytes.Buffer
	err := runPlan(context.Background(), strings.NewReader(""), &out, plan, func(context.Context) error {
		return want
	})
	if !errors.Is(err, want) {
		t.Errorf("expected error to be %v, got %v", want, err)
	}
	if strings.Contains(out.String(), "OK") {
		t.Errorf("must not print OK on perform error, got: %q", out.String())
	}
}

//nolint:paralleltest // viper global state
func TestValidateMutationConfig_NeedsEnvFalse_SkipsEnvTriple(t *testing.T) {
	// No env type/region/realm set, but needsEnv=false → must pass.
	cfg := config.Config{}
	if err := validateMutationConfig(cfg, false, false, false); err != nil {
		t.Fatalf("needsEnv=false should not require env triple, got: %v", err)
	}
}

func TestValidateMutationConfig_NeedsEnvTrue_RequiresEnvTriple(t *testing.T) {
	t.Parallel()
	cfg := config.Config{}
	err := validateMutationConfig(cfg, false, false, true)
	if err == nil {
		t.Fatal("needsEnv=true with empty env must error")
	}
}

// TestWithMutationSetup_InitLoggerError covers the rarely-exercised
// failure path of the prelude every mutation subcommand shares: a
// bad log_format must surface as an error from withMutationSetup
// without invoking fn. Without this test, a future change that broke
// initLogger error propagation would only be caught at runtime by
// six near-identical subcommands.
func TestWithMutationSetup_InitLoggerError(t *testing.T) {
	stageMutationEnv(t)
	t.Setenv("TOOLKIT_LOG_FORMAT", "bad-format")

	_, err := runRootCmd(t, []string{"cordon", "node-a", "-y"})
	if err == nil {
		t.Fatal("expected error from invalid log_format")
	}
	if !strings.Contains(err.Error(), "log-format") {
		t.Errorf("error should mention log_format, got: %v", err)
	}
}

// Every CLI mutation writes the shared audit record: begin, then done or
// failed, from surface cli; a dry run writes one dry_run record.
func TestRunMutation_Audits(t *testing.T) {
	t.Parallel()
	phasesIn := func(stdin string, plan testPlan, perform func(context.Context) error) []string {
		sink := logging.NewRingSink(8)
		ctx := logging.WithContext(context.Background(), slog.New(sink))
		_ = runPlan(ctx, strings.NewReader(stdin), &bytes.Buffer{}, plan, perform)
		var got []string
		for _, e := range sink.Snapshot() {
			kv := map[any]any{}
			for i := 0; i+1 < len(e.Fields); i += 2 {
				kv[e.Fields[i]] = e.Fields[i+1]
			}
			if e.Message == "mutation" && kv["surface"] == "cli" && kv["action"] == plan.Action.Verb {
				got = append(got, kv["phase"].(string))
			}
		}
		return got
	}
	phases := func(plan testPlan, perform func(context.Context) error) []string {
		return phasesIn("", plan, perform)
	}
	ok := func(context.Context) error { return nil }
	fail := func(context.Context) error { return errors.New("boom") }

	assert.Equal(t, []string{"begin", "done"}, phases(testPlan{Action: mutation.DrainNode, Target: "n1", Yes: true}, ok))
	assert.Equal(t, []string{"begin", "failed"}, phases(testPlan{Action: mutation.DrainNode, Target: "n1", Yes: true}, fail))
	assert.Equal(t, []string{"dry_run"}, phases(testPlan{Action: mutation.TerminateNode, Target: "n1", DryRun: true}, ok))

	// Refusals are audited, as on MCP and in the TUI: declining the
	// prompt, and an irreversible action without --yes.
	assert.Equal(t, []string{"refused"}, phasesIn("n\n", testPlan{Action: mutation.DrainNode, Target: "n1"}, ok))
	assert.Equal(t, []string{"refused"}, phases(testPlan{Action: mutation.TerminateNode, Target: "n1"}, ok))

	// A failed lookup is audited as failed, without a begin (nothing ran),
	// except in a dry run.
	lost := func(context.Context, *mutation.Plan) error { return errors.New("node not found") }
	assert.Equal(t, []string{"failed"}, phases(testPlan{Action: mutation.DrainNode, Target: "n1", Yes: true, Resolve: lost}, ok))
	assert.Empty(t, phases(testPlan{Action: mutation.DrainNode, Target: "n1", DryRun: true, Resolve: lost}, ok))
}

// An irreversible action without --yes is refused before its target is
// looked up: refusing touches nothing, as on MCP.
func TestRunMutation_IrreversibleRefusedBeforeLookup(t *testing.T) {
	t.Parallel()
	looked := false
	err := runPlan(context.Background(), strings.NewReader("y\n"), &bytes.Buffer{}, testPlan{
		Action: mutation.TerminateNode, Target: "n1",
		Resolve: func(context.Context, *mutation.Plan) error { looked = true; return nil },
	}, func(context.Context) error { return nil })
	require.ErrorContains(t, err, "--yes")
	assert.False(t, looked)
}

// The lookup may refine the action and target; the dry run and prompt
// name what it found.
func TestRunMutation_ResolveRefinesPlan(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	err := runPlan(context.Background(), strings.NewReader(""), &out, testPlan{
		Action: mutation.DeleteInferenceService, Target: "svc", DryRun: true,
		Resolve: func(_ context.Context, p *mutation.Plan) error {
			p.Action, p.Target, p.Resolved = mutation.ForceDeleteInferenceService, "ns1/svc", mutation.ResolvedNamespace("ns1")
			return nil
		},
	}, func(context.Context) error { return nil })
	require.NoError(t, err)
	assert.Equal(t, "DRY-RUN: would force-delete inference_service/ns1/svc (namespace=ns1)\n", out.String())
}

// A mutation's log session is named for its command path, so
// `toolkit logs --cmd drain` finds it.
func TestMutationName(t *testing.T) {
	t.Parallel()
	root := testRootCmd(nil)
	for path, want := range map[string]string{
		"drain":                    "drain",
		"cordon":                   "cordon",
		"restart node":             "restart-node",
		"delete inference-service": "delete-inference-service",
		"scale gpu-pool":           "scale-gpu-pool",
		"set tenant":               "set-tenant",
	} {
		cmd, _, err := root.Find(strings.Fields(path))
		if err != nil || cmd.CommandPath() != root.Name()+" "+path {
			t.Fatalf("find %q: %v (got %q)", path, err, cmd.CommandPath())
		}
		if got := mutationName(cmd); got != want {
			t.Errorf("mutationName(%q) = %q, want %q", path, got, want)
		}
	}
}
