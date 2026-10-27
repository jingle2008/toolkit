package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/jingle2008/toolkit/internal/config"
	"github.com/jingle2008/toolkit/internal/configloader"
	"github.com/jingle2008/toolkit/internal/infra/loader"
	production "github.com/jingle2008/toolkit/internal/infra/loader/production"
	"github.com/jingle2008/toolkit/internal/logging"
	"github.com/jingle2008/toolkit/internal/mutation"
	"github.com/jingle2008/toolkit/internal/mutation/ops"
	"github.com/jingle2008/toolkit/internal/resolve"
	"github.com/jingle2008/toolkit/pkg/models"
)

// depsFor is the mutations' infra calls for a command's config.
type depsFor func(cfg config.Config) ops.Deps

// productionDeps is the live cluster and OCI, with lookups through a
// fresh loader for cfg and writes to its metadata file.
func productionDeps(cfg config.Config) ops.Deps {
	d := ops.Production()
	d.ResolveGPUNode = func(ctx context.Context, env models.Environment, name, ocid string) (*models.GPUNode, error) {
		return resolve.GPUNode(ctx, production.New(ctx, cfg.MetadataFile), cfg.RepoPath, cfg.KubeConfig, env, name, ocid)
	}
	d.ResolveGPUPool = func(ctx context.Context, env models.Environment, name string) (*models.GPUPool, error) {
		return resolve.GPUPool(ctx, production.New(ctx, cfg.MetadataFile), cfg.RepoPath, cfg.KubeConfig, env, name)
	}
	d.CheckTenant = func(entry models.TenantMetadata) (bool, error) {
		if cfg.MetadataFile == "" {
			return false, errors.New("no metadata file configured")
		}
		return configloader.CheckTenantUpsert(cfg.MetadataFile, entry)
	}
	// Same path the TUI uses: the loader's optional TenantMetadataWriter.
	d.UpsertTenant = func(ctx context.Context, entry models.TenantMetadata) error {
		writer, ok := production.New(ctx, cfg.MetadataFile).(loader.TenantMetadataWriter)
		if !ok {
			return errors.New("loader does not support writing metadata")
		}
		return writer.UpsertTenantMetadata(entry)
	}
	return d
}

// opsEnv is where a CLI mutation acts.
func opsEnv(cfg config.Config, env models.Environment) ops.Env {
	return ops.Env{KubeConfig: cfg.KubeConfig, Env: env}
}

// runNodeAction is the RunE of `restart node` and `terminate`: newOp, an
// OCI action on the instance backing GPU node name, or on the --ocid
// instance. That needs no lookup of the node but the Terraform repo,
// whose GPU pools the instance must belong to, and the cluster for the
// env's GPU compartment they're in; the OCID is then the target the
// prompt and audit name, and name only a label.
func runNodeAction(
	cmd *cobra.Command, cfgFile *string, deps depsFor, name, ocid string, gate cliGate,
	newOp func(d ops.Deps, e ops.Env, name, ocid string) mutation.Op,
) error {
	return withMutationSetup(cmd.Context(), cfgFile, true, ocid != "", true, func(ctx context.Context, cfg config.Config, env models.Environment) error {
		return runMutation(ctx, cmd.InOrStdin(), cmd.OutOrStdout(), gate, newOp(deps(cfg), opsEnv(cfg, env), name, ocid))
	})
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
	return withCommandSetup(ctx, cfgFile, "mutate", standardChecks(needsKube, needsRepo, needsEnv), fn)
}

// setupChecks are what a command checks of its config before it runs.
type setupChecks struct {
	// validate checks the normalized config: a command that can't run
	// fails before it logs anything.
	validate func(config.Config) error
	// checkEnv, when set, checks the environment against the repo's, with
	// the command's logger in ctx.
	checkEnv func(ctx context.Context, cfg config.Config) error
}

// standardChecks are the needs checks most commands make
// (validateMutationConfig), and the environment check when it's needed.
func standardChecks(needsKube, needsRepo, needsEnv bool) setupChecks {
	c := setupChecks{validate: func(cfg config.Config) error {
		return validateMutationConfig(cfg, needsKube, needsRepo, needsEnv)
	}}
	if needsEnv {
		c.checkEnv = checkEnvironment
	}
	return c
}

// withCommandSetup is withMutationSetup for any command that reaches the
// loader: config, its checks, a logger tagged cmd=name, and the env.
func withCommandSetup(
	ctx context.Context,
	cfgFile *string,
	name string,
	checks setupChecks,
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
	if err := checks.validate(cfg); err != nil {
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
	if checks.checkEnv != nil {
		if err := checks.checkEnv(ctx, cfg); err != nil {
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

// cliGate is a CLI mutation's confirmation flags, and what its success
// line adds.
type cliGate struct {
	// DryRun looks the target up, prints a "would do X" line, and audits
	// with dry_run=true, instead of performing.
	DryRun bool
	// Yes skips the interactive confirmation prompt. Required for an
	// Irreversible action (terminate, delete), which can't be confirmed
	// interactively, since typing "y" by reflex is unsafe.
	Yes bool
	// Outcome, when set, is appended to the success line after perform:
	// "<action> <kind>/<target>: OK (<outcome>)".
	Outcome func() string
}

// addGateFlags registers cmd's --dry-run and --yes, bound to g. --yes's
// help follows a's tier: an Irreversible action has no prompt to skip.
func addGateFlags(cmd *cobra.Command, a mutation.Action, g *cliGate) {
	cmd.Flags().BoolVarP(&g.DryRun, "dry-run", "n", false, "Look the target up, print what would happen and what was found, and exit")
	yes := "Skip the interactive confirmation prompt"
	if a.Tier == mutation.Irreversible {
		yes = "Required: this action has no interactive prompt"
	}
	cmd.Flags().BoolVarP(&g.Yes, "yes", "y", false, yes)
}

// errDeclined is the prompt answered no: the call ends "aborted".
var errDeclined = errors.New("declined at the prompt")

// runMutation runs op through mutation.Run, the flow MCP's
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
// Errors from Resolve and Perform are returned verbatim; callers should
// let Cobra surface them.
func runMutation(
	ctx context.Context,
	in io.Reader,
	out io.Writer,
	g cliGate,
	op mutation.Op,
) error {
	gate := mutation.Gate{
		DryRun: g.DryRun,
		Admit: func(p mutation.Plan) error {
			if p.Action.Tier == mutation.Irreversible && !g.Yes && !g.DryRun {
				return mutation.Refuse(fmt.Errorf("%s requires explicit --yes (no interactive prompt for destructive actions)", p.Action.Verb))
			}
			return nil
		},
		Confirm: func(ctx context.Context, p mutation.Plan) error {
			printNotice(out, p.Notice)
			if g.Yes {
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
	p, outcome, err := mutation.Run(ctx, logging.FromContext(ctx), surfaceCLI, gate, op)
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
		_, _ = fmt.Fprintln(out, p.String()+": OK"+parenthesized(g.outcome()))
	}
	return nil
}

// outcome is the gate's Outcome, "" when unset.
func (g cliGate) outcome() string {
	if g.Outcome == nil {
		return ""
	}
	return g.Outcome()
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
