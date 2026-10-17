package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/jingle2008/toolkit/internal/config"
	"github.com/jingle2008/toolkit/internal/mutation"
	"github.com/jingle2008/toolkit/pkg/infra/logging"
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
			got, err := confirmAction(strings.NewReader(tc.in), &out, "Confirm? ")
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

func TestRunMutation_DryRun(t *testing.T) {
	t.Parallel()
	plan := mutationPlan{Action: mutation.CordonNode, Target: "n1", DryRun: true}
	called := false
	var out bytes.Buffer
	err := runMutation(context.Background(), strings.NewReader(""), &out, plan, func(context.Context) error {
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
	plan := mutationPlan{Action: mutation.CordonNode, Target: "n1", Yes: true}
	called := false
	var out bytes.Buffer
	// Stdin reader intentionally empty — Yes must not consume it.
	err := runMutation(context.Background(), strings.NewReader(""), &out, plan, func(context.Context) error {
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
	plan := mutationPlan{Action: mutation.CordonNode, Target: "n1"}
	called := false
	var out bytes.Buffer
	err := runMutation(context.Background(), strings.NewReader("n\n"), &out, plan, func(context.Context) error {
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
	plan := mutationPlan{Action: mutation.CordonNode, Target: "n1"}
	called := false
	var out bytes.Buffer
	err := runMutation(context.Background(), strings.NewReader("y\n"), &out, plan, func(context.Context) error {
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
	plan := mutationPlan{
		Action: mutation.TerminateNode, Target: "n1",
	}
	called := false
	var out bytes.Buffer
	// Stdin says "y" — should still fail because an irreversible action
	// blocks interactive consent.
	err := runMutation(context.Background(), strings.NewReader("y\n"), &out, plan, func(context.Context) error {
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
	plan := mutationPlan{
		Action: mutation.TerminateNode, Target: "n1", Yes: true,
	}
	called := false
	var out bytes.Buffer
	err := runMutation(context.Background(), strings.NewReader(""), &out, plan, func(context.Context) error {
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
	plan := mutationPlan{Action: mutation.CordonNode, Target: "n1", Yes: true}
	want := errors.New("kube unreachable")
	var out bytes.Buffer
	err := runMutation(context.Background(), strings.NewReader(""), &out, plan, func(context.Context) error {
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
//
//nolint:paralleltest // mutates process env + viper singleton
func TestWithMutationSetup_InitLoggerError(t *testing.T) {
	stageMutationEnv(t)
	t.Setenv("TOOLKIT_LOG_FORMAT", "bad-format")

	_, err := runRootCmd(t, []string{"cordon", "node-a", "-y"}, "")
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
	phases := func(plan mutationPlan, perform func(context.Context) error) []string {
		sink := logging.NewRingSink(8)
		ctx := logging.WithContext(context.Background(), sink)
		_ = runMutation(ctx, strings.NewReader(""), &bytes.Buffer{}, plan, perform)
		var got []string
		for _, e := range sink.Snapshot() {
			kv := map[any]any{}
			for i := 0; i+1 < len(e.Fields); i += 2 {
				kv[e.Fields[i]] = e.Fields[i+1]
			}
			if e.Message == "mutation" && kv["surface"] == "cli" && kv["action"] == plan.Action.Verb {
				got = append(got, kv["phase"].(string)) //nolint:forcetypeassert // Audit writes a string
			}
		}
		return got
	}
	ok := func(context.Context) error { return nil }
	fail := func(context.Context) error { return errors.New("boom") }

	assert.Equal(t, []string{"begin", "done"}, phases(mutationPlan{Action: mutation.DrainNode, Target: "n1", Yes: true}, ok))
	assert.Equal(t, []string{"begin", "failed"}, phases(mutationPlan{Action: mutation.DrainNode, Target: "n1", Yes: true}, fail))
	assert.Equal(t, []string{"dry_run"}, phases(mutationPlan{Action: mutation.TerminateNode, Target: "n1", DryRun: true}, ok))
}
