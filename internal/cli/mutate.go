package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/viper"

	"github.com/jingle2008/toolkit/internal/config"
	production "github.com/jingle2008/toolkit/internal/infra/loader/production"
	"github.com/jingle2008/toolkit/internal/resolve"
	"github.com/jingle2008/toolkit/pkg/infra/logging"
	"github.com/jingle2008/toolkit/pkg/models"
)

// gpuNodeResolverFn is the seam tests use to fake the k8s lookup.
// In production it constructs a fresh loader per call and delegates
// to internal/resolve.GPUNode.
var gpuNodeResolverFn = func(ctx context.Context, cfg config.Config, env models.Environment, name string) (*models.GPUNode, error) {
	ld := production.NewLoader(ctx, cfg.MetadataFile)
	return resolve.GPUNode(ctx, ld, cfg.KubeConfig, env, name, "")
}

// resolveGPUNode produces a *GPUNode suitable for handing to the
// OCI compute actions. If ocid is set, a stub node is synthesized
// (no cluster call); otherwise the live cluster is consulted via
// gpuNodeResolverFn. name is always carried for audit / log
// readability.
func resolveGPUNode(ctx context.Context, cfg config.Config, env models.Environment, name, ocid string) (*models.GPUNode, error) {
	if ocid != "" {
		return &models.GPUNode{Name: name, ID: ocid}, nil
	}
	return gpuNodeResolverFn(ctx, cfg, env, name)
}

// gpuPoolResolverFn is the seam tests use to fake gpu-pool resolution.
// In production it constructs a fresh loader and delegates to
// internal/resolve.GPUPool.
var gpuPoolResolverFn = func(ctx context.Context, cfg config.Config, env models.Environment, name string) (*models.GPUPool, error) {
	ld := production.NewLoader(ctx, cfg.MetadataFile)
	return resolve.GPUPool(ctx, ld, cfg.RepoPath, cfg.KubeConfig, env, name)
}

// validateMutationConfig checks the minimum settings a mutation
// subcommand needs.
//
//   - needsKube=true  → cluster-scoped mutations (cordon, drain) and
//     OCI mutations resolving a node by name without --ocid (reboot,
//     terminate without --ocid). Verifies the kubeconfig path is
//     readable; the path itself is always populated by the persistent
//     --kubeconfig flag's default (~/.kube/config), so the "missing
//     flag" case isn't reachable in normal use.
//   - needsRepo=true  → mutations sourced from Terraform (scale). The
//     repo path has no default and must be supplied.
func validateMutationConfig(cfg config.Config, needsKube, needsRepo bool) error {
	var missing []string
	if needsRepo && cfg.RepoPath == "" {
		missing = append(missing, "--repo_path")
	}
	if cfg.EnvType == "" {
		missing = append(missing, "--env_type")
	}
	if cfg.EnvRegion == "" {
		missing = append(missing, "--env_region")
	}
	if cfg.EnvRealm == "" {
		missing = append(missing, "--env_realm")
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

// withMutationSetup runs the standard prelude every mutation
// subcommand shares — read the config file, unmarshal, validate per
// needsKube/needsRepo, init the logger (deferred Sync), wire a
// signal-cancellable context with the logger attached, and build the
// Environment triple — then invokes fn with the resolved cfg / env /
// ctx. Keeps the setup uniform so individual subcommands focus only
// on flag parsing and their perform closure.
func withMutationSetup(
	cfgFile *string,
	needsKube, needsRepo bool,
	fn func(ctx context.Context, cfg config.Config, env models.Environment) error,
) error {
	if err := readConfigFile(cfgFile); err != nil {
		return err
	}
	var cfg config.Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return fmt.Errorf("unmarshal config: %w", err)
	}
	if err := validateMutationConfig(cfg, needsKube, needsRepo); err != nil {
		return err
	}
	logger, err := initLogger(cfg)
	if err != nil {
		return err
	}
	defer func() { _ = logger.Sync() }()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx = logging.WithContext(ctx, logger)

	env := models.Environment{
		Type:   cfg.EnvType,
		Region: cfg.EnvRegion,
		Realm:  cfg.EnvRealm,
	}
	return fn(ctx, cfg, env)
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
		_, _ = fmt.Fprintf(out, "DRY-RUN: would %s\n", desc)
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
			_, _ = fmt.Fprintln(out, "aborted")
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
		logger.Errorw("mutation",
			"action", plan.Action,
			"kind", plan.Kind,
			"target", plan.Target,
			"surface", plan.Surface,
			"phase", "failed",
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
	_, _ = fmt.Fprintf(out, "%s: OK\n", desc)
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
