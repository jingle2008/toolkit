// Package resolve maps a user-facing name (and optional OCID bypass)
// into the full *models.GpuNode / *models.GpuPool struct that the OCI
// compute actions require. Used by both `toolkit get`-derived
// mutation subcommands (internal/cli) and the MCP server's mutating
// tools (internal/mcp), so the find-by-name + OCI-enrichment chain
// lives in one place.
package resolve

import (
	"context"
	"errors"
	"fmt"

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
	populateGpuPoolsFn       = actions.PopulateGpuPools
	newClientsetFromKubeFn   = k8s.NewClientsetFromKubeConfig
	listGpuNodesForCompartFn = k8s.ListGpuNodes
)

// GpuNode finds a *models.GpuNode for the OCI compute actions. With
// ocid set, no cluster call is made — a stub {Name, ID:ocid} is
// returned. With ocid empty, the loader is consulted and the named
// node is returned by walking every pool.
func GpuNode(ctx context.Context, ld loader.Loader, kubeConfig string, env models.Environment, name, ocid string) (*models.GpuNode, error) {
	if ocid != "" {
		return &models.GpuNode{Name: name, ID: ocid}, nil
	}
	grouped, err := ld.LoadGpuNodes(ctx, kubeConfig, env)
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

// GpuPool loads GPU pools from the Terraform repo, finds the named
// one, then enriches with the live OCI ID + ActualSize via
// PopulateGpuPools. Partial-load on the Terraform pass is tolerated
// as long as the named pool is among the rows that did load — that
// matches the behavior of `toolkit get gpupool`.
func GpuPool(ctx context.Context, ld loader.Loader, repoPath, kubeConfig string, env models.Environment, name string) (*models.GpuPool, error) {
	pools, err := ld.LoadGpuPools(ctx, repoPath, env)
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

	enriched := []models.GpuPool{pools[idx]}
	if err := populateGpuPoolsFn(ctx, enriched, env, compartmentID); err != nil {
		return nil, fmt.Errorf("populate gpu pool: %w", err)
	}
	if enriched[0].ID == "" {
		return nil, fmt.Errorf("gpu pool %q has no OCID after OCI lookup; may not be applied yet", name)
	}
	return &enriched[0], nil
}

// CompartmentID queries the cluster for any GPU node and returns its
// CompartmentID. Used to scope OCI ListInstancePools calls during
// pool enrichment.
func CompartmentID(ctx context.Context, kubeConfig string, env models.Environment) (string, error) {
	clientset, err := newClientsetFromKubeFn(kubeConfig, env.GetKubeContext())
	if err != nil {
		return "", err
	}
	nodes, err := listGpuNodesForCompartFn(ctx, clientset, 1)
	if err != nil {
		return "", err
	}
	if len(nodes) == 0 {
		return "", fmt.Errorf("no GPU nodes in cluster (cannot resolve compartment ID)")
	}
	return nodes[0].CompartmentID, nil
}
