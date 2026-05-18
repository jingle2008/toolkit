package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/jingle2008/toolkit/internal/config"
	"github.com/jingle2008/toolkit/pkg/infra/logging"
)

// validateMutationConfig checks the minimum settings a mutation
// subcommand needs. needsKube=true adds the kubeconfig requirement
// (cluster-scoped mutations: cordon, uncordon, drain). Repo path is
// not required — mutations resolve targets by name from the live
// cluster or by --ocid bypass.
func validateMutationConfig(cfg config.Config, needsKube bool) error {
	var missing []string
	if cfg.EnvType == "" {
		missing = append(missing, "--env_type")
	}
	if cfg.EnvRegion == "" {
		missing = append(missing, "--env_region")
	}
	if cfg.EnvRealm == "" {
		missing = append(missing, "--env_realm")
	}
	if needsKube && cfg.KubeConfig == "" {
		missing = append(missing, "--kubeconfig")
	}
	if len(missing) > 0 {
		return fmt.Errorf(
			"missing required setting(s): %s\n"+
				"  set them via flags, environment (TOOLKIT_*), or `toolkit init`",
			strings.Join(missing, ", "),
		)
	}
	if needsKube {
		if _, err := os.Stat(cfg.KubeConfig); err != nil {
			return fmt.Errorf("kubeconfig %q not readable: %w", cfg.KubeConfig, err)
		}
	}
	return nil
}

// mutationPlan captures everything a mutation subcommand needs to
// confirm, audit, and execute uniformly. Subcommands build a plan +
// a perform closure; runMutation handles confirmation prompt,
// dry-run short-circuit, and structured audit logging.
type mutationPlan struct {
	// Action is the verb (e.g. "cordon", "drain", "scale", "terminate").
	Action string
	// Kind is the resource type (e.g. "node", "gpu_pool", "dac").
	Kind string
	// Target is a human-readable identifier (e.g. node name,
	// "<tenant>/<dac>"). Used in prompts and audit fields.
	Target string
	// Surface is the entry point — "cli" today; "mcp" once exposed.
	Surface string
	// DryRun short-circuits before perform runs, prints a "would do X"
	// line, and audits with dry_run=true.
	DryRun bool
	// Yes skips the interactive confirmation prompt. Required when
	// RequireExplicitYes is true.
	Yes bool
	// RequireExplicitYes makes interactive prompting impossible: only
	// --yes lets the action proceed. Used by destructive actions
	// (terminate, delete dac) where typing "y" by reflex is unsafe.
	RequireExplicitYes bool
}

// runMutation orchestrates the standard confirm / dry-run / audit /
// perform flow shared by every mutation subcommand.
//
// Output contract (writes to out):
//   - dry-run: "DRY-RUN: would <action> <kind>/<target>\n"
//   - interactive abort: "aborted\n"
//   - success: "<action> <kind>/<target>: OK\n"
//
// Errors from perform are returned verbatim; callers should let
// Cobra surface them.
func runMutation(
	ctx context.Context,
	in io.Reader,
	out io.Writer,
	plan mutationPlan,
	perform func(context.Context) error,
) error {
	logger := logging.FromContext(ctx)
	desc := fmt.Sprintf("%s %s/%s", plan.Action, plan.Kind, plan.Target)

	if plan.DryRun {
		fmt.Fprintf(out, "DRY-RUN: would %s\n", desc)
		logger.Infow("mutation",
			"action", plan.Action,
			"kind", plan.Kind,
			"target", plan.Target,
			"surface", plan.Surface,
			"dry_run", true,
		)
		return nil
	}

	if plan.RequireExplicitYes && !plan.Yes {
		return fmt.Errorf("%s requires explicit --yes (no interactive prompt for destructive actions)", plan.Action)
	}

	if !plan.Yes {
		ok, err := confirmAction(in, out, fmt.Sprintf("Confirm %s? [y/N]: ", desc))
		if err != nil {
			return fmt.Errorf("read confirmation: %w", err)
		}
		if !ok {
			fmt.Fprintln(out, "aborted")
			return nil
		}
	}

	logger.Infow("mutation",
		"action", plan.Action,
		"kind", plan.Kind,
		"target", plan.Target,
		"surface", plan.Surface,
		"dry_run", false,
		"phase", "begin",
	)
	if err := perform(ctx); err != nil {
		logger.Errorw("mutation failed",
			"action", plan.Action,
			"kind", plan.Kind,
			"target", plan.Target,
			"surface", plan.Surface,
			"error", err,
		)
		return err
	}
	logger.Infow("mutation",
		"action", plan.Action,
		"kind", plan.Kind,
		"target", plan.Target,
		"surface", plan.Surface,
		"dry_run", false,
		"phase", "done",
	)
	fmt.Fprintf(out, "%s: OK\n", desc)
	return nil
}

// confirmAction reads one line from in and reports whether the user
// said yes. Anything other than "y" / "yes" (case-insensitive,
// trimmed) is treated as no — including EOF, blank line, and any
// other input. Conservative-by-default: typos must mean abort.
func confirmAction(in io.Reader, out io.Writer, prompt string) (bool, error) {
	if _, err := fmt.Fprint(out, prompt); err != nil {
		return false, err
	}
	sc := bufio.NewScanner(in)
	if !sc.Scan() {
		if err := sc.Err(); err != nil {
			return false, err
		}
		return false, nil
	}
	ans := strings.ToLower(strings.TrimSpace(sc.Text()))
	return ans == "y" || ans == "yes", nil
}
