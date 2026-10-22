package mcp

import (
	"context"
	"os"
	"testing"

	"github.com/jingle2008/toolkit/internal/infra/k8s"
	"github.com/jingle2008/toolkit/internal/infra/oci"
	"github.com/jingle2008/toolkit/pkg/models"
)

// TestMain stubs the read-only target lookups that cordon_node,
// uncordon_node, drain_node, delete_dac and set_tenant make before acting
// or reporting a dry run, so every lookup finds its target and no test
// reaches a cluster, OCI or the user's metadata file. A test about a
// lookup swaps its seam again.
func TestMain(m *testing.M) {
	mcpNodeCordonedFn = func(context.Context, string, string, string) (bool, error) { return false, nil }
	mcpDrainStatusFn = func(_ context.Context, _, _, node string) (k8s.DrainStatus, error) {
		return k8s.DrainStatus{Node: node, DrainState: k8s.DrainState{Phase: k8s.DrainNone}}, nil
	}
	mcpGetDACFn = func(_ context.Context, d *models.DedicatedAICluster, env models.Environment) (oci.DACState, error) {
		return oci.DACState{ID: d.OCID(env.Realm, env.Region), State: "ACTIVE"}, nil
	}
	mcpCheckTenantFn = func(*Server, models.TenantMetadata) (bool, error) { return false, nil }
	os.Exit(m.Run())
}
