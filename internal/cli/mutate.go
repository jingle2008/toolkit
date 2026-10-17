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

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/jingle2008/toolkit/internal/config"
	production "github.com/jingle2008/toolkit/internal/infra/loader/production"
	"github.com/jingle2008/toolkit/internal/mutation"
	"github.com/jingle2008/toolkit/internal/resolve"
	"github.com/jingle2008/toolkit/pkg/infra/logging"
	"github.com/jingle2008/toolkit/pkg/models"
)

// resolveGPUNodeFn is the seam tests use to fake the k8s lookup.
// In production it constructs a fresh loader per call and delegates
// to internal/resolve.GPUNode.
var resolveGPUNodeFn = func(ctx context.Context, cfg config.Config, env models.Environment, name string) (*models.GPUNode, error) {
	ld := production.New(ctx, cfg.MetadataFile)
	return resolve.GPUNode(ctx, ld, cfg.KubeConfig, env, name, "")
}

// resolveGPUNode produces a *GPUNode suitable for handing to the
// OCI compute actions. If ocid is set, a stub node is synthesized
// (no cluster call); otherwise the live cluster is consulted via
// resolveGPUNodeFn. name is always carried for audit / log
// readability.
func resolveGPUNode(ctx context.Context, cfg config.Config, env models.Environment, name, ocid string) (*models.GPUNode, error) {
	if ocid != "" {
		return &models.GPUNode{Name: name, ID: ocid}, nil
	}
	return resolveGPUNodeFn(ctx, cfg, env, name)
}

// resolveNode is a mutationPlan.Resolve for a GPU node: it looks the node
// up (or, given --ocid, takes the instance as is) into *node.
func resolveNode(cfg config.Config, env models.Environment, name, ocid string, node **models.GPUNode) func(context.Context, *mutationPlan) error {
	return func(ctx context.Context, p *mutationPlan) error {
		n, err := resolveGPUNode(ctx, cfg, env, name, ocid)
		if err != nil {
			return err
		}
		*node, p.Resolved = n, mutation.ResolvedNode(n.ID)
		return nil
	}
}

// runNodeAction is the RunE of `restart node` and `terminate`: act, an
// OCI action on the instance backing GPU node name (or the --ocid
// instance, which needs no cluster lookup).
func runNodeAction(
	cmd *cobra.Command, cfgFile *string, a mutation.Action, name, ocid string, dryRun, yes bool,
	act func(context.Context, *models.GPUNode, models.Environment, logging.Logger) error,
) error {
	return withMutationSetup(cfgFile, ocid == "", false, true, func(ctx context.Context, cfg config.Config, env models.Environment) error {
		var node *models.GPUNode
		return runMutation(ctx, cmd.InOrStdin(), cmd.OutOrStdout(), mutationPlan{
			Action:  a,
			Target:  name,
			DryRun:  dryRun,
			Yes:     yes,
			Resolve: resolveNode(cfg, env, name, ocid, &node),
		}, func(ctx context.Context) error {
			return act(ctx, node, env, logging.FromContext(ctx))
		})
	})
}

// resolveISVC is a mutationPlan.Resolve for an InferenceService: it
// finds the namespace (searching all when namespace is empty) into *ns,
// so the target, prompt and audit name <namespace>/<name>.
func resolveISVC(cfg config.Config, env models.Environment, namespace, name string, ns *string) func(context.Context, *mutationPlan) error {
	return func(ctx context.Context, p *mutationPlan) error {
		found, err := resolveInferenceServiceFn(ctx, cfg.KubeConfig, env.KubeContext(), namespace, name)
		if err != nil {
			return err
		}
		*ns, p.Target, p.Resolved = found, found+"/"+name, mutation.ResolvedNamespace(found)
		return nil
	}
}

// isvcTarget is an InferenceService target as named on the command line.
func isvcTarget(namespace, name string) string {
	if namespace != "" {
		return namespace + "/" + name
	}
	return name
}

// resolveGPUPoolFn is the seam tests use to fake gpu-pool resolution.
// In production it constructs a fresh loader and delegates to
// internal/resolve.GPUPool.
var resolveGPUPoolFn = func(ctx context.Context, cfg config.Config, env models.Environment, name string) (*models.GPUPool, error) {
	ld := production.New(ctx, cfg.MetadataFile)
	return resolve.GPUPool(ctx, ld, cfg.RepoPath, cfg.KubeConfig, env, name)
}

// validateMutationConfig checks the minimum settings a mutation
// subcommand needs.
//
//   - needsKube=true  → cluster-scoped mutations (cordon, drain) and
//     OCI mutations resolving a node by name without --ocid (restart node,
//     terminate without --ocid). Verifies the kubeconfig path is
//     readable; the path itself is always populated by the persistent
//     --kubeconfig flag's default (~/.kube/config), so the "missing
//     flag" case isn't reachable in normal use.
//   - needsRepo=true  → mutations sourced from Terraform (scale). The
//     repo path has no default and must be supplied.
func validateMutationConfig(cfg config.Config, needsKube, needsRepo, needsEnv bool) error {
	var missing []string
	if needsRepo && cfg.RepoPath == "" {
		missing = append(missing, "--repo-path")
	}
	if needsEnv {
		if cfg.EnvType == "" {
			missing = append(missing, "--env-type")
		}
		if cfg.EnvRegion == "" {
			missing = append(missing, "--env-region")
		}
		if cfg.EnvRealm == "" {
			missing = append(missing, "--env-realm")
		}
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
// needsKube/needsRepo/needsEnv, init the logger (deferred Sync), wire a
// signal-cancellable context with the logger attached, and build the
// Environment triple — then invokes fn with the resolved cfg / env /
// ctx. Keeps the setup uniform so individual subcommands focus only
// on flag parsing and their perform closure.
func withMutationSetup(
	cfgFile *string,
	needsKube, needsRepo, needsEnv bool,
	fn func(ctx context.Context, cfg config.Config, env models.Environment) error,
) error {
	if err := readConfigFile(cfgFile); err != nil {
		return err
	}
	var cfg config.Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return fmt.Errorf("unmarshal config: %w", err)
	}
	rawType, rawRegion := cfg.EnvType, cfg.EnvRegion
	if err := cfg.Normalize(); err != nil {
		return err
	}
	if err := validateMutationConfig(cfg, needsKube, needsRepo, needsEnv); err != nil {
		return err
	}
	logger, err := initLogger(cfg)
	if err != nil {
		return err
	}
	logger = logger.WithFields("cmd", "mutate")
	defer func() { _ = logger.Sync() }()
	// Mutations act on whatever env this resolves to, so the expansion
	// belongs in the audit trail alongside the action itself.
	logEnvAliases(logger, rawType, rawRegion, cfg)

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
	// Action is what the subcommand does. Its tier decides the
	// confirmation: an Irreversible action (terminate, delete) can't
	// be confirmed interactively — only --yes lets it proceed, since
	// typing "y" by reflex is unsafe.
	Action mutation.Action
	// Target is a human-readable identifier (e.g. node name,
	// "<namespace>/<name>"). Used in prompts and audit fields.
	Target string
	// DryRun short-circuits before perform runs, prints a "would do X"
	// line, and audits with dry_run=true.
	DryRun bool
	// Yes skips the interactive confirmation prompt. Required for an
	// Irreversible action.
	Yes bool
	// Resolve, when set, looks the target up without changing anything,
	// before the dry run or the prompt — as MCP's plan step does — so a
	// dry run proves the target exists and the prompt names what was
	// found. It may refine the plan's Action and Target, and records in
	// Resolved what a dry run reports. Its perform closure uses what it
	// found.
	Resolve func(ctx context.Context, p *mutationPlan) error
	// Resolved is what Resolve found (mutation.ResolvedNode, ...).
	Resolved mutation.Resolved
}

// runMutation orchestrates the standard refuse / resolve / dry-run /
// confirm / audit / perform flow shared by every mutation subcommand, in
// the order MCP's runMutationCall uses, with the same audit records:
//
//  1. An Irreversible action without --yes (and not a dry run): refused,
//     before touching anything.
//  2. Resolve looks the target up; a failure ends the call, audited as
//     failed unless a dry run.
//  3. --dry-run: report what would run and what was resolved.
//  4. Without --yes, the prompt; declining is audited as refused.
//  5. perform, audited begin, then done or failed.
//
// Output contract (writes to out):
//   - dry-run: "DRY-RUN: would <action> <kind>/<target>[ (<resolved>)]\n"
//   - interactive abort: "aborted\n"
//   - success: "<action> <kind>/<target>: OK\n"
//
// Errors from Resolve and perform are returned verbatim; callers should
// let Cobra surface them.
func runMutation(
	ctx context.Context,
	in io.Reader,
	out io.Writer,
	plan mutationPlan,
	perform func(context.Context) error,
) error {
	logger := logging.FromContext(ctx)
	audit := func(phase mutation.Phase, err error) {
		mutation.Audit(logger, surfaceCLI, plan.Action, plan.Target, phase, err)
	}

	if plan.Action.Tier == mutation.Irreversible && !plan.Yes && !plan.DryRun {
		audit(mutation.PhaseRefused, nil)
		return fmt.Errorf("%s requires explicit --yes (no interactive prompt for destructive actions)", plan.Action.Verb)
	}
	if plan.Resolve != nil {
		if err := plan.Resolve(ctx, &plan); err != nil {
			if !plan.DryRun {
				audit(mutation.PhaseFailed, err)
			}
			return err
		}
	}
	desc := fmt.Sprintf("%s %s/%s", plan.Action.Verb, plan.Action.Kind, plan.Target)

	if plan.DryRun {
		line := "DRY-RUN: would " + desc
		if r := plan.Resolved.String(); r != "" {
			line += " (" + r + ")"
		}
		_, _ = fmt.Fprintln(out, line)
		audit(mutation.PhaseDryRun, nil)
		return nil
	}

	if !plan.Yes {
		ok, err := confirmAction(in, out, fmt.Sprintf("Confirm %s? [y/N]: ", desc))
		if err != nil {
			return fmt.Errorf("read confirmation: %w", err)
		}
		if !ok {
			audit(mutation.PhaseRefused, nil)
			_, _ = fmt.Fprintln(out, "aborted")
			return nil
		}
	}

	audit(mutation.PhaseBegin, nil)
	if err := perform(ctx); err != nil {
		audit(mutation.PhaseFailed, err)
		return err
	}
	audit(mutation.PhaseDone, nil)
	_, _ = fmt.Fprintf(out, "%s: OK\n", desc)
	return nil
}

// surfaceCLI is the audit record's surface for every CLI mutation.
const surfaceCLI = "cli"

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
