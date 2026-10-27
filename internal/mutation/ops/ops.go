// Package ops builds the mutations the CLI and MCP share: each
// constructor returns a mutation.Op whose Resolve looks the target up and
// whose Perform acts on it, through the infra calls in Deps. A front-end
// runs the Op through mutation.Run with its own gate, and renders what
// the constructor hands back beside it.
package ops

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/jingle2008/toolkit/internal/infra/k8s"
	"github.com/jingle2008/toolkit/internal/infra/oci"
	"github.com/jingle2008/toolkit/internal/logging"
	"github.com/jingle2008/toolkit/internal/mutation"
	"github.com/jingle2008/toolkit/pkg/models"
)

// Deps are the infra calls the mutations make; tests fake them.
type Deps struct {
	// ResolveGPUNode looks a GPU node up in the cluster by name, or,
	// given an OCID, checks the instance is in one of env's GPU pools.
	ResolveGPUNode func(ctx context.Context, env models.Environment, name, ocid string) (*models.GPUNode, error)
	// ResolveGPUPool looks a GPU pool up in the Terraform repo and OCI.
	ResolveGPUPool func(ctx context.Context, env models.Environment, name string) (*models.GPUPool, error)

	NodeCordoned func(ctx context.Context, kubeConfig, kubeContext, node string) (bool, error)
	SetCordon    func(ctx context.Context, kubeConfig, kubeContext, node string, unschedulable bool) (bool, error)
	DrainStatus  func(ctx context.Context, kubeConfig, kubeContext, node string) (k8s.DrainStatus, error)

	SoftReset        func(ctx context.Context, node *models.GPUNode, env models.Environment, logger *slog.Logger) error
	Terminate        func(ctx context.Context, node *models.GPUNode, env models.Environment, logger *slog.Logger) error
	IncreasePoolSize func(ctx context.Context, pool *models.GPUPool, add int, env models.Environment, logger *slog.Logger) (oci.PoolScale, error)

	GetDAC    func(ctx context.Context, dac *models.DedicatedAICluster, env models.Environment) (oci.DACState, error)
	DeleteDAC func(ctx context.Context, dac *models.DedicatedAICluster, env models.Environment, logger *slog.Logger) error

	ResolveISVC      func(ctx context.Context, kubeConfig, kubeContext, namespace, name string) (string, error)
	RestartISVC      func(ctx context.Context, kubeConfig, kubeContext, namespace, name string) error
	PlanOrphanISVC   func(ctx context.Context, kubeConfig, kubeContext, namespace, name string) (k8s.OrphanDeleteOutcome, error)
	DeleteOrphanISVC func(ctx context.Context, kubeConfig, kubeContext, namespace, name string) (k8s.OrphanDeleteOutcome, error)

	// CheckTenant makes the metadata write's checks without writing,
	// reporting whether the entry exists.
	CheckTenant  func(entry models.TenantMetadata) (bool, error)
	UpsertTenant func(ctx context.Context, entry models.TenantMetadata) error
}

// Env is where a mutation acts.
type Env struct {
	KubeConfig string
	Env        models.Environment
}

func (e Env) kubeContext() string { return e.Env.KubeContext() }

// Cordon cordons node, or uncordons it when unschedulable is false.
// Changed reports, after Perform, whether the node's state changed.
func Cordon(d Deps, e Env, node string, unschedulable bool) (op mutation.Op, changed *bool) {
	a := mutation.CordonNode
	if !unschedulable {
		a = mutation.UncordonNode
	}
	changed = new(bool)
	return mutation.Op{
		Plan: mutation.Plan{Action: a, Target: node},
		Resolve: func(ctx context.Context, p *mutation.Plan) error {
			cordoned, err := d.NodeCordoned(ctx, e.KubeConfig, e.kubeContext(), node)
			p.Resolved = mutation.ResolvedCordon(cordoned)
			return err
		},
		Perform: func(ctx context.Context, _ mutation.Plan) error {
			var err error
			*changed, err = d.SetCordon(ctx, e.KubeConfig, e.kubeContext(), node, unschedulable)
			return err
		},
	}, changed
}

// Drain looks node's drain up. How it runs is the front-end's: the CLI
// drives it to the end, MCP starts it and drives it in the background,
// so the caller sets Perform.
func Drain(d Deps, e Env, node string) mutation.Op {
	return mutation.Op{
		Plan: mutation.Plan{Action: mutation.DrainNode, Target: node},
		Resolve: func(ctx context.Context, p *mutation.Plan) error {
			status, err := d.DrainStatus(ctx, e.KubeConfig, e.kubeContext(), node)
			p.Resolved = mutation.ResolvedDrain(string(status.Phase))
			return err
		},
	}
}

// RestartNode soft-resets the instance backing GPU node name, or the
// ocid instance; see nodeAction.
func RestartNode(d Deps, e Env, name, ocid string) mutation.Op {
	return nodeAction(d, e, mutation.RestartNode, name, ocid, d.SoftReset)
}

// TerminateNode terminates the instance backing GPU node name, or the
// ocid instance; see nodeAction.
func TerminateNode(d Deps, e Env, name, ocid string) mutation.Op {
	return nodeAction(d, e, mutation.TerminateNode, name, ocid, d.Terminate)
}

// NodeTarget is a GPU node action's target: the node as named, or, given
// an OCID, the OCID.
func NodeTarget(name, ocid string) string {
	if ocid != "" {
		return ocid
	}
	return name
}

// nodeAction is act, an OCI action, on the instance backing GPU node
// name. Given an OCID, the node isn't looked up but the instance must be
// in one of the env's GPU pools; the OCID is then the target — so an
// irreversible action's confirmation repeats it, not the free-form node
// label, which the audit records carry beside it.
func nodeAction(
	d Deps, e Env, a mutation.Action, name, ocid string,
	act func(context.Context, *models.GPUNode, models.Environment, *slog.Logger) error,
) mutation.Op {
	op := mutation.Op{Plan: mutation.Plan{Action: a, Target: NodeTarget(name, ocid)}}
	if ocid != "" {
		op.AuditFields = func() []any { return []any{"node", name} }
	}
	var node *models.GPUNode
	op.Resolve = func(ctx context.Context, p *mutation.Plan) error {
		n, err := d.ResolveGPUNode(ctx, e.Env, name, ocid)
		if err != nil {
			return err
		}
		node, p.Resolved = n, mutation.ResolvedNode(n.ID)
		if ocid != "" {
			p.Resolved = mutation.ResolvedPoolInstance(n.ID, n.NodePool)
		}
		return nil
	}
	op.Perform = func(ctx context.Context, _ mutation.Plan) error {
		return act(ctx, node, e.Env, logging.FromContext(ctx))
	}
	return op
}

// Scale is a pool scale-up, as ScalePool resolves and performs it.
type Scale struct {
	// Pool is the pool, once resolved.
	Pool *models.GPUPool
	// Add is the instances to add, counted from the lookup.
	Add int
	// Result is the scale-up's sizes, once performed.
	Result *oci.PoolScale
}

// Sizes is the sizes the scale-up went from and to, "" before Perform.
func (s *Scale) Sizes() string {
	if s.Result == nil {
		return ""
	}
	return s.Result.String()
}

// Summary is Sizes, with a warning when the scale-up took the pool past
// its Terraform size — for a front-end that showed no notice before it.
func (s *Scale) Summary() string {
	out := s.Sizes()
	if s.Result == nil {
		return out
	}
	if w := mutation.PoolOverSizeWarning(s.Pool.Size, s.Result.After); w != "" {
		out += ". " + w
	}
	return out
}

// ScalePool grows GPU pool name by add instances or, when !addGiven, to
// its Terraform size (a Noop when it is there). addFlag names the add
// option in the Noop note: the CLI's --add, MCP's add.
func ScalePool(d Deps, e Env, name string, add int, addGiven bool, addFlag string) (mutation.Op, *Scale) {
	s := &Scale{}
	return mutation.Op{
		Plan: mutation.Plan{Action: mutation.ScaleGPUPool, Target: name},
		Resolve: func(ctx context.Context, p *mutation.Plan) error {
			found, err := d.ResolveGPUPool(ctx, e.Env, name)
			if err != nil {
				return err
			}
			if found.ID == "" {
				return fmt.Errorf("pool %s: %w", found.Name, oci.ErrNoInstancePool)
			}
			if s.Add, err = mutation.PoolScaleAdd(*found, add, addGiven); err != nil {
				return err
			}
			if s.Add == 0 {
				p.Noop = mutation.PoolAtSizeNote(*found, addFlag)
				return nil
			}
			s.Pool = found
			p.Resolved = mutation.ResolvedPool(found.ID, found.Size, found.ActualSize, s.Add)
			p.Notice = mutation.PoolScaleNotice(*found, s.Add)
			return nil
		},
		AuditFields: func() []any {
			if s.Result == nil {
				return []any{"add", s.Add}
			}
			return []any{"add", s.Add, "from", s.Result.Before, "to", s.Result.After}
		},
		Perform: func(ctx context.Context, _ mutation.Plan) error {
			count := s.Add
			if !addGiven {
				count = oci.FillPool // Add was counted from the lookup; OCI's size may have moved since
			}
			result, err := d.IncreasePoolSize(ctx, s.Pool, count, e.Env, logging.FromContext(ctx))
			if err != nil {
				return err
			}
			s.Result = &result
			return nil
		},
	}, s
}

// DeleteDAC deletes DAC name and its endpoints, looked up in OCI first.
func DeleteDAC(d Deps, e Env, name string) mutation.Op {
	dac := &models.DedicatedAICluster{Name: name}
	return mutation.Op{
		Plan: mutation.Plan{Action: mutation.DeleteDAC, Target: name},
		Resolve: func(ctx context.Context, p *mutation.Plan) error {
			found, err := d.GetDAC(ctx, dac, e.Env)
			p.Resolved = mutation.ResolvedDAC(found.ID, found.State)
			return err
		},
		Perform: func(ctx context.Context, _ mutation.Plan) error {
			return d.DeleteDAC(ctx, dac, e.Env, logging.FromContext(ctx))
		},
	}
}

// ISVCTarget is an InferenceService target as requested: <namespace>/<name>,
// or the bare name when the namespace is to be searched for.
func ISVCTarget(namespace, name string) string {
	if namespace != "" {
		return namespace + "/" + name
	}
	return name
}

// resolveISVC finds an InferenceService's namespace (searching all when
// namespace is empty) into *ns, so the target names <namespace>/<name>.
func resolveISVC(d Deps, e Env, namespace, name string, ns *string) func(context.Context, *mutation.Plan) error {
	return func(ctx context.Context, p *mutation.Plan) error {
		found, err := d.ResolveISVC(ctx, e.KubeConfig, e.kubeContext(), namespace, name)
		if err != nil {
			return err
		}
		*ns, p.Target, p.Resolved = found, found+"/"+name, mutation.ResolvedNamespace(found)
		return nil
	}
}

// RestartISVC rolling-restarts InferenceService name.
func RestartISVC(d Deps, e Env, namespace, name string) mutation.Op {
	var ns string
	return mutation.Op{
		Plan:    mutation.Plan{Action: mutation.RestartInferenceService, Target: ISVCTarget(namespace, name)},
		Resolve: resolveISVC(d, e, namespace, name, &ns),
		Perform: func(ctx context.Context, _ mutation.Plan) error {
			return d.RestartISVC(ctx, e.KubeConfig, e.kubeContext(), ns, name)
		},
	}
}

// DeleteOrphanISVC deletes InferenceService name, an orphan (its DAC is
// gone). Resolving also checks it is one, and picks delete or
// force-delete, which names the planned action. Done reports, after
// Perform, what the delete did.
func DeleteOrphanISVC(d Deps, e Env, namespace, name string) (op mutation.Op, done *k8s.OrphanDeleteOutcome) {
	var ns string
	resolveNS := resolveISVC(d, e, namespace, name, &ns)
	done = new(k8s.OrphanDeleteOutcome)
	return mutation.Op{
		Plan: mutation.Plan{Action: mutation.DeleteInferenceService, Target: ISVCTarget(namespace, name)},
		Resolve: func(ctx context.Context, p *mutation.Plan) error {
			if err := resolveNS(ctx, p); err != nil {
				return err
			}
			outcome, err := d.PlanOrphanISVC(ctx, e.KubeConfig, e.kubeContext(), ns, name)
			if err != nil {
				return err
			}
			p.Action = mutation.InferenceServiceDelete(outcome == k8s.OrphanFinalized)
			return nil
		},
		Perform: func(ctx context.Context, _ mutation.Plan) error {
			var err error
			*done, err = d.DeleteOrphanISVC(ctx, e.KubeConfig, e.kubeContext(), ns, name)
			return err
		},
	}, done
}

// TenancyOCIDPrefix guards against entries that would never resolve:
// Metadata.GetTenants keys off the OCID's realm segment.
const TenancyOCIDPrefix = "ocid1.tenancy."

// TenantEntry is the metadata entry for tenancy ocid. A nil internal
// keeps the recorded flag (a new tenant then defaults to internal); an
// empty note sets none. name must not be blank — the caller says so in
// its own terms (a flag, a parameter).
func TenantEntry(ocid, name string, internal *bool, note string) (models.TenantMetadata, error) {
	if !strings.HasPrefix(ocid, TenancyOCIDPrefix) {
		return models.TenantMetadata{}, fmt.Errorf("invalid tenancy OCID %q: must start with %q", ocid, TenancyOCIDPrefix)
	}
	if strings.TrimSpace(name) == "" {
		return models.TenantMetadata{}, errors.New("tenant name is required")
	}
	entry := models.TenantMetadata{ID: ocid, Name: &name, IsInternal: internal}
	if note != "" {
		entry.Note = &note
	}
	return entry, nil
}

// SetTenant writes entry, making the write's own checks first (a name
// another tenancy has, a file that doesn't load) so a dry run catches
// them too.
func SetTenant(d Deps, entry models.TenantMetadata) mutation.Op {
	return mutation.Op{
		Plan: mutation.Plan{Action: mutation.SetTenant, Target: entry.ID},
		Resolve: func(_ context.Context, p *mutation.Plan) error {
			exists, err := d.CheckTenant(entry)
			p.Resolved = mutation.ResolvedTenant(exists)
			return err
		},
		Perform: func(ctx context.Context, _ mutation.Plan) error {
			return d.UpsertTenant(ctx, entry)
		},
	}
}
