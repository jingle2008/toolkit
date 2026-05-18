package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/jingle2008/toolkit/internal/config"
	"github.com/jingle2008/toolkit/internal/infra/k8s"
	production "github.com/jingle2008/toolkit/internal/infra/loader/production"
	"github.com/jingle2008/toolkit/internal/infra/terraform"
	"github.com/jingle2008/toolkit/internal/ui/tui/actions"
	"github.com/jingle2008/toolkit/pkg/infra/logging"
	"github.com/jingle2008/toolkit/pkg/models"
)

// gpuNodeResolverFn is the seam tests use to fake the k8s lookup. In
// production it routes through realResolveGpuNode → loader.LoadGpuNodes.
var gpuNodeResolverFn = realResolveGpuNode

// realResolveGpuNode finds a GpuNode by name in the live cluster.
// Used by mutation subcommands that need the underlying OCI instance
// ID (reboot, terminate). Callers may bypass this entirely by passing
// --ocid; see resolveGpuNode below.
func realResolveGpuNode(ctx context.Context, cfg config.Config, env models.Environment, name string) (*models.GpuNode, error) {
	ld := production.NewLoader(ctx, cfg.MetadataFile)
	grouped, err := ld.LoadGpuNodes(ctx, cfg.KubeConfig, env)
	if err != nil {
		return nil, fmt.Errorf("load gpu nodes: %w", err)
	}
	for _, nodes := range grouped {
		for i := range nodes {
			if nodes[i].Name == name {
				return &nodes[i], nil
			}
		}
	}
	return nil, fmt.Errorf("gpu node %q not found in any pool", name)
}

// resolveGpuNode produces a *GpuNode suitable for handing to the
// OCI compute actions. If ocid is set, a stub node is synthesized
// (no cluster call); otherwise the live cluster is consulted via
// gpuNodeResolverFn. name is always carried for audit / log
// readability.
func resolveGpuNode(ctx context.Context, cfg config.Config, env models.Environment, name, ocid string) (*models.GpuNode, error) {
	if ocid != "" {
		return &models.GpuNode{Name: name, ID: ocid}, nil
	}
	return gpuNodeResolverFn(ctx, cfg, env, name)
}

// gpuPoolResolverFn is the seam tests use to fake gpu-pool resolution.
var gpuPoolResolverFn = realResolveGpuPool

// realResolveGpuPool loads gpu pools from the Terraform repo, finds
// one by name, and enriches it with the live OCI ID + ActualSize.
// Terraform is the source of truth for Size; OCI fills in the ID
// (needed by UpdateInstancePool). Partial-load on the Terraform
// pass is tolerated as long as the named pool is among the rows
// that did load.
func realResolveGpuPool(ctx context.Context, cfg config.Config, env models.Environment, name string) (*models.GpuPool, error) {
	ld := production.NewLoader(ctx, cfg.MetadataFile)
	pools, err := ld.LoadGpuPools(ctx, cfg.RepoPath, env)
	if err != nil {
		if _, ok := errors.AsType[*terraform.PartialLoadError](err); !ok {
			return nil, fmt.Errorf("load gpu pools: %w", err)
		}
		logging.FromContext(ctx).Infow("gpu pools loaded with partial failures", "error", err)
	}

	idx := -1
	for i := range pools {
		if pools[i].Name == name {
			idx = i
			break
		}
	}
	if idx < 0 {
		return nil, fmt.Errorf("gpu pool %q not found in repo", name)
	}

	compartmentID, err := resolveCompartmentID(ctx, cfg, env)
	if err != nil {
		return nil, fmt.Errorf("resolve compartment ID: %w", err)
	}
	enriched := []models.GpuPool{pools[idx]}
	if err := actions.PopulateGpuPools(ctx, enriched, env, compartmentID); err != nil {
		return nil, fmt.Errorf("populate gpu pool: %w", err)
	}
	if enriched[0].ID == "" {
		return nil, fmt.Errorf("gpu pool %q has no OCID after OCI lookup; may not be applied yet", name)
	}
	return &enriched[0], nil
}

// resolveCompartmentID queries the cluster for any GPU node and
// returns its CompartmentID. Mirrors the TUI's getCompartmentID
// fallback path (the TUI prefers the dataset cache; CLI doesn't keep
// one between invocations).
func resolveCompartmentID(ctx context.Context, cfg config.Config, env models.Environment) (string, error) {
	clientset, err := k8s.NewClientsetFromKubeConfig(cfg.KubeConfig, env.GetKubeContext())
	if err != nil {
		return "", err
	}
	nodes, err := k8s.ListGpuNodes(ctx, clientset, 1)
	if err != nil {
		return "", err
	}
	if len(nodes) == 0 {
		return "", fmt.Errorf("no GPU nodes in cluster (cannot resolve compartment ID)")
	}
	return nodes[0].CompartmentID, nil
}

// validateScaleConfig is the strictest mutation validator: scale needs
// the Terraform repo (Size source of truth), kubeconfig (compartment
// lookup), and the env triple (OCI client + kube context).
func validateScaleConfig(cfg config.Config) error {
	var missing []string
	if cfg.RepoPath == "" {
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
	if cfg.KubeConfig == "" {
		missing = append(missing, "--kubeconfig")
	}
	if len(missing) > 0 {
		return fmt.Errorf(
			"missing required setting(s) for scale: %s\n"+
				"  set them via flags, environment (TOOLKIT_*), or `toolkit init`",
			strings.Join(missing, ", "),
		)
	}
	if _, err := os.Stat(cfg.KubeConfig); err != nil {
		return fmt.Errorf("kubeconfig %q not readable: %w", cfg.KubeConfig, err)
	}
	return nil
}

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
