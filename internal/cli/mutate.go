package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/jingle2008/toolkit/internal/config"
	production "github.com/jingle2008/toolkit/internal/infra/loader/production"
	"github.com/jingle2008/toolkit/internal/logging"
	"github.com/jingle2008/toolkit/internal/mutation"
	"github.com/jingle2008/toolkit/internal/resolve"
	"github.com/jingle2008/toolkit/pkg/models"
)

// resolveGPUNodeFn is the seam tests use to fake the node lookup — in
// the cluster by name, or, given an OCID, in the env's GPU instance
// pools. In production it constructs a fresh loader per call and
// delegates to internal/resolve.GPUNode.
var resolveGPUNodeFn = func(ctx context.Context, cfg config.Config, env models.Environment, name, ocid string) (*models.GPUNode, error) {
	ld := production.New(ctx, cfg.MetadataFile)
	return resolve.GPUNode(ctx, ld, cfg.RepoPath, cfg.KubeConfig, env, name, ocid)
}

// resolveNode is a mutationPlan.Resolve for a GPU node: it looks the node
// up (or, given --ocid, verifies the instance is in one of the env's GPU
// pools) into *node.
func resolveNode(cfg config.Config, env models.Environment, name, ocid string, node **models.GPUNode) func(context.Context, *mutation.Plan) error {
	return func(ctx context.Context, p *mutation.Plan) error {
		n, err := resolveGPUNodeFn(ctx, cfg, env, name, ocid)
		if err != nil {
			return err
		}
		*node, p.Resolved = n, mutation.ResolvedNode(n.ID)
		if ocid != "" {
			p.Resolved = mutation.ResolvedPoolInstance(n.ID, n.NodePool)
		}
		return nil
	}
}

// runNodeAction is the RunE of `restart node` and `terminate`: act, an
// OCI action on the instance backing GPU node name, or on the --ocid
// instance. That needs no lookup of the node but the Terraform repo,
// whose GPU pools the instance must belong to, and the cluster for the
// env's GPU compartment they're in; the OCID is then the target the
// prompt and audit name, and name only a label.
func runNodeAction(
	cmd *cobra.Command, cfgFile *string, a mutation.Action, name, ocid string, dryRun, yes bool,
	act func(context.Context, *models.GPUNode, models.Environment, *slog.Logger) error,
) error {
	target, fields := name, func() []any { return nil }
	if ocid != "" {
		target, fields = ocid, func() []any { return []any{"node", name} }
	}
	return withMutationSetup(cmd.Context(), cfgFile, true, ocid != "", true, func(ctx context.Context, cfg config.Config, env models.Environment) error {
		var node *models.GPUNode
		return runMutation(ctx, cmd.InOrStdin(), cmd.OutOrStdout(), mutationPlan{
			Action:      a,
			Target:      target,
			DryRun:      dryRun,
			Yes:         yes,
			Resolve:     resolveNode(cfg, env, name, ocid, &node),
			AuditFields: fields,
		}, func(ctx context.Context) error {
			return act(ctx, node, env, logging.FromContext(ctx))
		})
	})
}

// resolveISVC is a mutationPlan.Resolve for an InferenceService: it
// finds the namespace (searching all when namespace is empty) into *ns,
// so the target, prompt and audit name <namespace>/<name>.
func resolveISVC(cfg config.Config, env models.Environment, namespace, name string, ns *string) func(context.Context, *mutation.Plan) error {
	return func(ctx context.Context, p *mutation.Plan) error {
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
//     OCI mutations on a node (restart node, terminate; with --ocid, for
//     the env's GPU compartment). Verifies the kubeconfig path is
//     readable; the path itself is always populated by the persistent
//     --kubeconfig flag's default (~/.kube/config), so the "missing
//     flag" case isn't reachable in normal use.
//   - needsRepo=true  → mutations sourced from Terraform (scale, and
//     restart node / terminate with --ocid, whose instance must be in
//     one of the declared GPU pools). The repo path has no default and
//     must be supplied.
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
// needsKube/needsRepo/needsEnv, init the logger (deferred Sync), attach
// the logger to ctx (the command's signal-cancellable context from
// Execute), and build the Environment triple — then invokes fn with the
// resolved cfg / env / ctx. Keeps the setup uniform so individual
// subcommands focus only on flag parsing and their perform closure.
func withMutationSetup(
	ctx context.Context,
	cfgFile *string,
	needsKube, needsRepo, needsEnv bool,
	fn func(ctx context.Context, cfg config.Config, env models.Environment) error,
) error {
	return withCommandSetup(ctx, cfgFile, "mutate", needsKube, needsRepo, needsEnv, fn)
}

// withCommandSetup is withMutationSetup for any command that reaches the
// loader: config, the needs checks, a logger tagged cmd=name, and the env.
func withCommandSetup(
	ctx context.Context,
	cfgFile *string,
	name string,
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
	logger, logFile, _, err := initLogger(cfg, name)
	if err != nil {
		return err
	}
	logger = logger.With("cmd", name)
	defer func() { _ = logFile.Close() }()
	// Commands act on whatever env this resolves to, so the expansion
	// belongs in the log (for mutations, the audit trail) alongside them.
	logEnvAliases(logger, rawType, rawRegion, cfg)

	ctx = logging.WithContext(ctx, logger)
	if needsEnv {
		if err := checkEnvironment(ctx, cfg); err != nil {
			return err
		}
	}

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
	// found. It may refine the plan's Action and Target, records in
	// Resolved what a dry run reports, and may set Noop (printed; nothing
	// prompted, performed or audited) and Notice (printed before the
	// prompt, with --yes too, and after the dry-run line). Its perform
	// closure uses what it found.
	Resolve func(ctx context.Context, p *mutation.Plan) error
	// AuditFields, when set, adds the action's own fields to every audit
	// record from Resolve on. It is called per record, so fields the
	// perform closure fills in (a scale's actual sizes) reach the done
	// record.
	AuditFields func() []any
	// Outcome, when set, is appended to the success line after perform:
	// "<action> <kind>/<target>: OK (<outcome>)".
	Outcome func() string
}

// errDeclined is the prompt answered no: the call ends "aborted".
var errDeclined = errors.New("declined at the prompt")

// runMutation runs plan through mutation.Run, the flow MCP's
// runMutationCall shares, with the CLI's gate: an Irreversible action
// without --yes (and not a dry run) is refused before anything is
// touched, and a real run without --yes asks at the prompt; declining is
// audited as refused.
//
// Output contract (writes to out):
//   - noop: "<noop>\n"
//   - dry-run: "DRY-RUN: would <action> <kind>/<target>[ (<resolved>)]\n[<notice>\n]"
//   - interactive abort: "aborted\n"
//   - success: "<action> <kind>/<target>: OK[ (<outcome>)]\n"
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
	gate := mutation.Gate{
		DryRun: plan.DryRun,
		Admit: func(p mutation.Plan) error {
			if p.Action.Tier == mutation.Irreversible && !plan.Yes && !plan.DryRun {
				return mutation.Refuse(fmt.Errorf("%s requires explicit --yes (no interactive prompt for destructive actions)", p.Action.Verb))
			}
			return nil
		},
		Confirm: func(ctx context.Context, p mutation.Plan) error {
			printNotice(out, p.Notice)
			if plan.Yes {
				return nil
			}
			ok, err := confirmAction(ctx, in, out, fmt.Sprintf("Confirm %s? [y/N]: ", p))
			if err != nil {
				return fmt.Errorf("read confirmation: %w", err)
			}
			if !ok {
				return mutation.Refuse(errDeclined)
			}
			return nil
		},
	}
	p, outcome, err := mutation.Run(ctx, logging.FromContext(ctx), surfaceCLI, gate, mutation.Op{
		Plan:        mutation.Plan{Action: plan.Action, Target: plan.Target},
		Resolve:     plan.Resolve,
		Perform:     func(ctx context.Context, _ mutation.Plan) error { return perform(ctx) },
		AuditFields: plan.AuditFields,
	})
	if errors.Is(err, errDeclined) {
		_, _ = fmt.Fprintln(out, "aborted")
		return nil
	}
	if err != nil {
		return err
	}
	switch outcome {
	case mutation.Noop:
		_, _ = fmt.Fprintln(out, p.Noop)
	case mutation.DryRun:
		_, _ = fmt.Fprintln(out, "DRY-RUN: would "+p.String()+parenthesized(p.Resolved.String()))
		printNotice(out, p.Notice)
	case mutation.Performed:
		_, _ = fmt.Fprintln(out, p.String()+": OK"+parenthesized(plan.outcome()))
	}
	return nil
}

// outcome is the plan's Outcome, "" when unset.
func (p *mutationPlan) outcome() string {
	if p.Outcome == nil {
		return ""
	}
	return p.Outcome()
}

// printNotice prints notice, when there is one.
func printNotice(out io.Writer, notice string) {
	if notice != "" {
		_, _ = fmt.Fprintln(out, notice)
	}
}

// parenthesized is " (s)", or "" for an empty s.
func parenthesized(s string) string {
	if s == "" {
		return ""
	}
	return " (" + s + ")"
}

// surfaceCLI is the audit record's surface for every CLI mutation.
const surfaceCLI = "cli"

// confirmAction reads one line from in and reports whether the user
// said yes. Anything other than "y" / "yes" (case-insensitive,
// trimmed) is treated as no — including EOF, blank line, and any
// other input. Conservative-by-default: typos must mean abort.
//
// A cancelled ctx (Ctrl+C at the prompt) returns ctx's error at once. The
// read itself can't be interrupted, so its goroutine is left blocked on
// in; the process is about to exit.
func confirmAction(ctx context.Context, in io.Reader, out io.Writer, prompt string) (bool, error) {
	if _, err := fmt.Fprint(out, prompt); err != nil {
		return false, err
	}
	type answer struct {
		yes bool
		err error
	}
	done := make(chan answer, 1)
	go func() {
		sc := bufio.NewScanner(in)
		if !sc.Scan() {
			done <- answer{err: sc.Err()}
			return
		}
		ans := strings.ToLower(strings.TrimSpace(sc.Text()))
		done <- answer{yes: ans == "y" || ans == "yes"}
	}()
	select {
	case a := <-done:
		return a.yes, a.err
	case <-ctx.Done():
		return false, ctx.Err()
	}
}
