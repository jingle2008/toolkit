// Package resolve maps a user-facing name (and optional OCID bypass)
// into the full *models.GPUNode / *models.GPUPool struct that the OCI
// compute actions require. Used by both `toolkit get`-derived
// mutation subcommands (internal/cli) and the MCP server's mutating
// tools (internal/mcp), so the find-by-name + OCI-enrichment chain
// lives in one place.
package resolve

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/jingle2008/toolkit/internal/infra/k8s"
	"github.com/jingle2008/toolkit/internal/infra/loader"
	"github.com/jingle2008/toolkit/internal/infra/terraform"
	"github.com/jingle2008/toolkit/internal/ui/tui/actions"
	"github.com/jingle2008/toolkit/pkg/infra/logging"
	"github.com/jingle2008/toolkit/pkg/models"
)

// Seam variables — tests in this package swap them to avoid touching
// a live cluster or OCI tenancy. Each defaults to the real upstream.
var (
	populateGPUPoolsFn          = actions.PopulateGPUPools
	newClientsetFromKubeFn      = k8s.NewClientsetFromKubeConfig
	listGPUNodesByCompartmentFn = k8s.ListGPUNodes
)

// GPUNode finds a *models.GPUNode for the OCI compute actions. With
// ocid set, no cluster call is made — a stub {Name, ID:ocid} is
// returned. With ocid empty, the loader is consulted and the named
// node is returned by walking every pool.
func GPUNode(ctx context.Context, ld loader.Composite, kubeConfig string, env models.Environment, name, ocid string) (*models.GPUNode, error) {
	if ocid != "" {
		return &models.GPUNode{Name: name, ID: ocid}, nil
	}
	grouped, err := ld.LoadGPUNodesByPool(ctx, kubeConfig, env)
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

// GPUPool loads GPU pools from the Terraform repo, finds the named
// one, then enriches with the live OCI ID + ActualSize via
// PopulateGPUPools. Partial-load on the Terraform pass is tolerated
// as long as the named pool is among the rows that did load — that
// matches the behavior of `toolkit get gpupool`.
func GPUPool(ctx context.Context, ld loader.Composite, repoPath, kubeConfig string, env models.Environment, name string) (*models.GPUPool, error) {
	pools, err := ld.LoadGPUPools(ctx, repoPath, env)
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

	compartmentID, err := CompartmentID(ctx, kubeConfig, env)
	if err != nil {
		return nil, fmt.Errorf("resolve compartment ID: %w", err)
	}

	enriched := []models.GPUPool{pools[idx]}
	if err := populateGPUPoolsFn(ctx, enriched, env, compartmentID); err != nil {
		return nil, fmt.Errorf("populate gpu pool: %w", err)
	}
	if enriched[0].ID == "" {
		return nil, fmt.Errorf("gpu pool %q has no OCID after OCI lookup; may not be applied yet", name)
	}
	return &enriched[0], nil
}

// EnrichGPUPools fills ActualSize and Status on every pool by
// resolving the compartment ID via the live cluster and then calling
// PopulateGPUPools.
//
// Best-effort: a non-nil error means enrichment couldn't complete
// (compartment lookup or OCI populate failed), but the pools slice
// is still safe to use — the loader's Status="..." placeholder
// remains and ActualSize stays at zero. Callers should surface the
// error as a warning, not abort.
//
// Used by `toolkit get gpupool` and MCP `list_gpu_pools` to match
// the TUI's enriched view. Mutation paths (resolve.GPUPool) keep
// their per-pool enrichment for the single-pool ID lookup they
// actually need.
func EnrichGPUPools(ctx context.Context, pools []models.GPUPool, kubeConfig string, env models.Environment) error {
	if len(pools) == 0 {
		return nil
	}
	logger := logging.FromContext(ctx)
	compartmentID, err := CompartmentID(ctx, kubeConfig, env)
	if err != nil {
		logger.Infow("gpu pool enrichment failed", "step", "compartment_id", "error", err)
		return fmt.Errorf("compartment lookup failed: %w", err)
	}
	if err := populateGPUPoolsFn(ctx, pools, env, compartmentID); err != nil {
		logger.Infow("gpu pool enrichment failed", "step", "populate", "error", err)
		return fmt.Errorf("OCI populate failed: %w", err)
	}
	return nil
}

// compartmentCache memoizes successful CompartmentID lookups for the
// life of the process. The MCP server makes the call on every
// list_gpu_pools / scale_gpu_pool tool invocation; without caching,
// every call burns one K8s lookup before the OCI step. Compartment
// identity for a given cluster is stable for the cluster's life, so
// the cache has no semantic risk.
//
// Key is env.KubeContext() — a string like "dp-dev-iad" that
// uniquely identifies the target cluster. kubeConfig is intentionally
// NOT part of the key: it's set once at NewServer (MCP) or once per
// CLI invocation and never mutated within a process, so it can't
// vary across cache lookups.
//
// CLI invocations are one-shot processes, so the cache neither helps
// nor hurts them — but keeping the cache at the package level avoids
// threading a pre-resolved compartment ID through 4 call paths.
//
// Errors are NOT cached: a transient cluster failure self-heals on
// the next request. Cleared in tests via clearCompartmentCache.
var compartmentCache sync.Map // map[string]string — kubeContext → compartmentID

// clearCompartmentCache resets the package-level cache. Test-only;
// production callers cannot reach a state that requires invalidation
// (see comment on compartmentCache).
func clearCompartmentCache() {
	compartmentCache.Range(func(k, _ any) bool {
		compartmentCache.Delete(k)
		return true
	})
}

// CompartmentID queries the cluster for any GPU node and returns its
// CompartmentID. Used to scope OCI ListInstancePools calls during
// pool enrichment. Successful lookups are cached per kubeContext for
// the life of the process.
func CompartmentID(ctx context.Context, kubeConfig string, env models.Environment) (string, error) {
	kubeContext := env.KubeContext()
	if cached, ok := compartmentCache.Load(kubeContext); ok {
		return cached.(string), nil //nolint:forcetypeassert // only Stored values are string
	}
	clientset, err := newClientsetFromKubeFn(kubeConfig, kubeContext)
	if err != nil {
		return "", err
	}
	nodes, err := listGPUNodesByCompartmentFn(ctx, clientset, 1)
	if err != nil {
		return "", err
	}
	if len(nodes) == 0 {
		return "", fmt.Errorf("no GPU nodes in cluster (cannot resolve compartment ID)")
	}
	result := nodes[0].CompartmentID
	compartmentCache.Store(kubeContext, result)
	return result, nil
}
