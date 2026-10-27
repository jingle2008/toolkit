package mcp

import (
	"context"
	"reflect"

	"github.com/jingle2008/toolkit/internal/infra/k8s"
	"github.com/jingle2008/toolkit/internal/infra/oci"
	"github.com/jingle2008/toolkit/internal/mutation/ops"
	"github.com/jingle2008/toolkit/pkg/models"
)

// testLookups is the read-only target lookups that cordon_node,
// uncordon_node, drain_node, delete_dac and set_tenant make before acting
// or reporting a dry run, so every lookup finds its target and no test
// reaches a cluster, OCI or the user's metadata file. A test about a
// lookup sets its own.
func testLookups() ops.Deps {
	return ops.Deps{
		NodeCordoned: func(context.Context, string, string, string) (bool, error) { return false, nil },
		DrainStatus: func(_ context.Context, _, _, node string) (k8s.DrainStatus, error) {
			return k8s.DrainStatus{Node: node, DrainState: k8s.DrainState{Phase: k8s.DrainNone}}, nil
		},
		GetDAC: func(_ context.Context, d *models.DedicatedAICluster, env models.Environment) (oci.DACState, error) {
			return oci.DACState{ID: d.OCID(env.Realm, env.Region), State: "ACTIVE"}, nil
		},
		CheckTenant: func(models.TenantMetadata) (bool, error) { return false, nil },
	}
}

// fillDeps sets every call d leaves unset to from's.
func fillDeps(d *ops.Deps, from ops.Deps) {
	dv, fv := reflect.ValueOf(d).Elem(), reflect.ValueOf(from)
	for i := range dv.NumField() {
		if dv.Field(i).IsNil() {
			dv.Field(i).Set(fv.Field(i))
		}
	}
}
