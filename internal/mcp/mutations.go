package mcp

import (
	"context"
	"errors"
	"fmt"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jingle2008/toolkit/internal/config"
	"github.com/jingle2008/toolkit/internal/configloader"
	"github.com/jingle2008/toolkit/internal/infra/k8s"
	"github.com/jingle2008/toolkit/internal/infra/loader"
	"github.com/jingle2008/toolkit/internal/infra/oci"
	"github.com/jingle2008/toolkit/internal/mutation"
	"github.com/jingle2008/toolkit/internal/resolve"
	"github.com/jingle2008/toolkit/pkg/infra/logging"
	"github.com/jingle2008/toolkit/pkg/models"
)

// Seam variables — overrideable in tests so handlers don't reach a
// live cluster or OCI tenancy. Production callers go through the
// upstream packages directly. The resolver seams (mcpResolveGPUNodeFn,
// mcpResolveGPUPoolFn) cover the lookup-and-enrich chain so handler
// tests don't have to stub a fake k8s+OCI pipeline.
var (
	mcpSetCordonFn        = k8s.SetCordon
	mcpNodeCordonedFn     = k8s.NodeCordoned
	mcpStartDrainFn       = k8s.StartDrain
	mcpAwaitDrainFn       = k8s.AwaitDrain
	mcpDrainStatusFn      = k8s.GetDrainStatus
	mcpSoftResetFn        = oci.SoftResetInstance
	mcpTerminateFn        = oci.TerminateInstance
	mcpIncreasePoolSizeFn = oci.IncreasePoolSize
	mcpDeleteDACFn        = oci.DeleteDedicatedAICluster
	mcpGetDACFn           = oci.GetDedicatedAICluster
	mcpResolveISVCFn      = k8s.ResolveInferenceService
	mcpRestartISVCFn      = k8s.RestartInferenceService
	mcpPlanOrphanISVCFn   = k8s.PlanOrphanInferenceServiceDelete
	mcpDeleteOrphanISVCFn = k8s.DeleteOrphanInferenceService
	mcpResolveGPUNodeFn   = func(ctx context.Context, s *Server, env models.Environment, name, ocid string) (*models.GPUNode, error) {
		return resolve.GPUNode(ctx, s.loader, s.cfg.KubeConfig, env, name, ocid)
	}
	mcpResolveGPUPoolFn = func(ctx context.Context, s *Server, env models.Environment, name string) (*models.GPUPool, error) {
		return resolve.GPUPool(ctx, s.loader, s.cfg.RepoPath, s.cfg.KubeConfig, env, name)
	}
	mcpCheckTenantFn = func(s *Server, entry models.TenantMetadata) (bool, error) {
		if s.cfg.MetadataFile == "" {
			return false, errors.New("no metadata file configured")
		}
		return configloader.CheckTenantUpsert(s.cfg.MetadataFile, entry)
	}
	mcpUpsertTenantFn = func(s *Server, entry models.TenantMetadata) error {
		writer, ok := s.loader.(loader.TenantMetadataWriter)
		if !ok {
			return errors.New("loader does not support writing metadata")
		}
		return writer.UpsertTenantMetadata(entry)
	}
)

// confirmGate is embedded in every mutating tool's input. The fields
// are OPTIONAL at the JSON-Schema level (omitempty) so the SDK passes
// the call to the handler even when confirm is missing — that way we
// can audit-log the attempt and return an error explaining the
// contract. Only confirm=true triggers execution; dry_run=true reports
// what would run instead, and wins over confirm.
type confirmGate struct {
	Confirm bool `json:"confirm,omitempty" jsonschema:"set true to execute; otherwise the tool refuses without acting"`
	DryRun  bool `json:"dry_run,omitempty" jsonschema:"set true to report what the call would do without acting: the target is looked up as for a real call and the result is status DRY_RUN with what was resolved; needs no confirm"`
}

// irreversibleGate is embedded, beside confirmGate, in the input of
// every tool whose action is mutation.Irreversible. confirm=true alone
// doesn't run it: the agent must also name the target it means — the
// MCP counterpart of the TUI's capital Y and the CLI's mandatory --yes.
type irreversibleGate struct {
	ConfirmTarget string `json:"confirm_target,omitempty" jsonschema:"required to execute this irreversible action: the target repeated exactly, as you named it or as dry_run reports it"`
}

// surfaceMCP is the audit record's surface for every MCP mutation.
const surfaceMCP = "mcp"

// effectiveMutationEnv applies the agent's envOverride only when the
// operator opted into per-call overrides via
// MutationEnvOverrideAllowed. Otherwise the override is ignored and
// the startup env is used unchanged. When the override IS applied and
// changes the effective env, that's audit-logged for SIEM visibility.
func (s *Server) effectiveMutationEnv(a mutation.Action, target string, in envOverride) models.Environment {
	startup := s.envFor(envOverride{})
	if !s.cfg.MutationEnvOverrideAllowed {
		if in.EnvType != "" || in.EnvRegion != "" || in.EnvRealm != "" {
			s.logger.Infow(
				"mutation env_override ignored (server disallows)",
				"action", a.Verb, "kind", a.Kind, "target", target, "surface", surfaceMCP,
				"requested_env_type", in.EnvType,
				"requested_env_region", in.EnvRegion,
				"requested_env_realm", in.EnvRealm,
			)
		}
		return startup
	}
	effective := s.envFor(in)
	if effective != startup {
		s.logger.Infow(
			"mutation env override active (deviation from startup)",
			"level", "warn",
			"action", a.Verb, "kind", a.Kind, "target", target, "surface", surfaceMCP,
			"startup_realm", startup.Realm, "effective_realm", effective.Realm,
			"startup_region", startup.Region, "effective_region", effective.Region,
			"startup_type", startup.Type, "effective_type", effective.Type,
		)
	}
	return effective
}

// planned is what a mutation will do, once its target is looked up.
type planned struct {
	action mutation.Action
	target string
	// resolved is what the lookup found, reported by a dry run.
	resolved mutation.Resolved
	// noop, when the lookup sets it, ends the call as status NOOP with
	// this note: there is nothing to do, so nothing is performed or
	// audited.
	noop string
	// note is a dry run's note: what the action will do, any warning.
	note string
}

func (p planned) String() string { return p.action.Verb + " " + p.action.Kind + "/" + p.target }

// ok is the success envelope for p.
func (p planned) ok() mutationResult { return mutationOK(p.action.Verb, p.action.Kind, p.target) }

// mutationCall is one mutating tool call, as its handler describes it.
type mutationCall[Out any] struct {
	action mutation.Action
	target string // as the agent named it
	gate   confirmGate
	// confirmTarget is the agent's irreversibleGate.ConfirmTarget,
	// checked when the planned action is Irreversible.
	confirmTarget string
	// plan, when set, looks the target up without changing anything. It
	// may refine p's action and target, and records in p.resolved what a
	// dry run reports.
	plan func(p *planned) error
	// perform acts on what plan found.
	perform func(p planned) (Out, error)
	// envelope builds the tool's result from a bare envelope, for the
	// dry-run and NOOP responses.
	envelope func(mutationResult) Out
	// auditFields, when set, adds the action's own fields to every audit
	// record. It is called per record, so fields perform fills in (a
	// scale's actual sizes) reach the done record.
	auditFields func() []any
}

// fields is the call's own audit fields, none when unset.
func (c mutationCall[Out]) fields() []any {
	if c.auditFields == nil {
		return nil
	}
	return c.auditFields()
}

// asResult is the envelope for tools whose result is the bare envelope.
func asResult(r mutationResult) mutationResult { return r }

// runMutationCall is the whole MCP mutation flow, shared by every
// mutating tool, with the audit record of each step:
//
//  1. Neither confirm nor dry_run: refused, before touching anything.
//  2. plan looks the target up; a failure ends the call (audited as
//     failed unless a dry run). A noop it sets ends the call as NOOP.
//  3. dry_run: report the plan as status DRY_RUN.
//  4. An Irreversible action whose confirm_target names neither the
//     requested nor the resolved target: refused.
//  5. perform, audited begin, then done or failed.
//
// The refusal and failure paths return an error, which the SDK renders
// as IsError plus text; success is Out, which the SDK marshals into
// StructuredContent and auto-emits as TextContent.
func runMutationCall[Out any](s *Server, c mutationCall[Out]) (*sdk.CallToolResult, Out, error) {
	p := planned{action: c.action, target: c.target}
	audit := func(phase mutation.Phase, err error) {
		mutation.Audit(s.logger, surfaceMCP, p.action, p.target, phase, err, c.fields()...)
	}
	if !c.gate.Confirm && !c.gate.DryRun {
		audit(mutation.PhaseRefused, nil)
		return failTool[Out](p.action.Verb,
			fmt.Errorf("mutating tool requires confirm=true (target %s/%s)", p.action.Kind, p.target))
	}
	if c.plan != nil {
		if err := c.plan(&p); err != nil {
			if !c.gate.DryRun {
				audit(mutation.PhaseFailed, err)
			}
			return failTool[Out](p.String(), err)
		}
	}
	if p.noop != "" {
		r := p.ok()
		r.Status, r.Note = "NOOP", p.noop
		return &sdk.CallToolResult{}, c.envelope(r), nil
	}
	if c.gate.DryRun {
		audit(mutation.PhaseDryRun, nil)
		r := p.ok()
		r.Status, r.Resolved, r.Note = "DRY_RUN", p.resolved, p.note
		return &sdk.CallToolResult{}, c.envelope(r), nil
	}
	if p.action.Tier == mutation.Irreversible &&
		(c.confirmTarget == "" || (c.confirmTarget != c.target && c.confirmTarget != p.target)) {
		audit(mutation.PhaseRefused, nil)
		return failTool[Out](p.String(), fmt.Errorf(
			"irreversible: confirm=true is not enough; also set confirm_target to %q", p.target,
		))
	}
	audit(mutation.PhaseBegin, nil)
	out, err := c.perform(p)
	if err != nil {
		audit(mutation.PhaseFailed, err)
		return failTool[Out](p.String(), err)
	}
	audit(mutation.PhaseDone, nil)
	// Empty Content lets the SDK auto-emit the structured result as text.
	return &sdk.CallToolResult{}, out, nil
}

// --- Input types --------------------------------------------------

type cordonNodeInput struct {
	Node string `json:"node" jsonschema:"the node name as reported by kubectl get nodes"`
	confirmGate
	envOverride
}

type drainNodeInput struct {
	Node string `json:"node" jsonschema:"the node name as reported by kubectl get nodes"`
	confirmGate
	envOverride
}

type drainStatusInput struct {
	Node string `json:"node" jsonschema:"the node name as reported by kubectl get nodes"`
	envOverride
}

// drainNodeResult is the standard envelope plus the drain's status at
// the moment drain_node returned.
type drainNodeResult struct {
	mutationResult
	Drain k8s.DrainStatus `json:"drain"`
}

type restartNodeInput struct {
	Node string `json:"node" jsonschema:"the node name as reported by kubectl get nodes"`
	OCID string `json:"ocid,omitempty" jsonschema:"skip k8s lookup and target this instance OCID directly"`
	confirmGate
	envOverride
}

type terminateNodeInput struct {
	Node string `json:"node" jsonschema:"the node name as reported by kubectl get nodes"`
	OCID string `json:"ocid,omitempty" jsonschema:"skip k8s lookup and target this instance OCID directly"`
	confirmGate
	irreversibleGate
	envOverride
}

type scaleGPUPoolInput struct {
	Name string `json:"name" jsonschema:"the pool name from the Terraform repo (same as toolkit get gpupool)"`
	Add  *int   `json:"add,omitempty" jsonschema:"instances to add (at least 1), counted from OCI's current size; may take the pool past its Terraform size, which the next terraform apply reverts. Omit to fill the pool to its Terraform size (status NOOP when it is already there)"`
	confirmGate
	envOverride
}

// scaleGPUPoolResult is the standard envelope plus, on success, the
// sizes the scale-up went from and to.
type scaleGPUPoolResult struct {
	mutationResult
	Scale *oci.PoolScale `json:"scale,omitempty"`
}

type deleteDACInput struct {
	Name string `json:"name" jsonschema:"the DAC name (same identifier as toolkit get dac shows)"`
	confirmGate
	irreversibleGate
	envOverride
}

type restartInferenceServiceInput struct {
	Name      string `json:"name" jsonschema:"the InferenceService name (same as list_inference_services shows)"`
	Namespace string `json:"namespace,omitempty" jsonschema:"the InferenceService namespace; omit to search all namespaces (the name must then be unique)"`
	confirmGate
	envOverride
}

type deleteInferenceServiceInput struct {
	Name      string `json:"name" jsonschema:"the InferenceService name (same as list_inference_services shows)"`
	Namespace string `json:"namespace,omitempty" jsonschema:"the InferenceService namespace; omit to search all namespaces (the name must then be unique)"`
	confirmGate
	irreversibleGate
	envOverride
}

// deleteInferenceServiceResult is the mutation envelope plus the
// one-line outcome, which says when a follow-up force-delete applies.
type deleteInferenceServiceResult struct {
	mutationResult
	Summary string `json:"summary"`
}

type setTenantInput struct {
	OCID       string `json:"ocid" jsonschema:"the full tenancy OCID (the metadata entry key); must start with ocid1.tenancy."`
	Name       string `json:"name" jsonschema:"friendly tenant name (required)"`
	IsInternal *bool  `json:"is_internal,omitempty" jsonschema:"mark tenant internal; when omitted, keeps the recorded flag (a new tenant defaults to internal)"`
	Note       string `json:"note,omitempty" jsonschema:"optional free-form note"`
	confirmGate
}

// --- Handlers -----------------------------------------------------

func (s *Server) handleCordonNode(ctx context.Context, req *sdk.CallToolRequest, in cordonNodeInput) (*sdk.CallToolResult, mutationResult, error) {
	return s.setCordon(ctx, mutation.CordonNode, in, true)
}

func (s *Server) handleUncordonNode(ctx context.Context, req *sdk.CallToolRequest, in cordonNodeInput) (*sdk.CallToolResult, mutationResult, error) {
	return s.setCordon(ctx, mutation.UncordonNode, in, false)
}

func (s *Server) setCordon(ctx context.Context, a mutation.Action, in cordonNodeInput, unschedulable bool) (*sdk.CallToolResult, mutationResult, error) {
	env := s.effectiveMutationEnv(a, in.Node, in.envOverride)
	return runMutationCall(s, mutationCall[mutationResult]{
		action: a, target: in.Node, gate: in.confirmGate, envelope: asResult,
		plan: func(p *planned) error {
			cordoned, err := mcpNodeCordonedFn(ctx, s.cfg.KubeConfig, env.KubeContext(), in.Node)
			p.resolved = mutation.ResolvedCordon(cordoned)
			return err
		},
		perform: func(p planned) (mutationResult, error) {
			_, err := mcpSetCordonFn(ctx, s.cfg.KubeConfig, env.KubeContext(), in.Node, unschedulable)
			return p.ok(), err
		},
	})
}

// handleDrainNode starts (or resumes) the drain and returns at once,
// leaving a background driver to carry it through. The drain lives on
// the Node, so a driver lost to a server restart is replaced by calling
// drain_node again.
func (s *Server) handleDrainNode(ctx context.Context, req *sdk.CallToolRequest, in drainNodeInput) (*sdk.CallToolResult, drainNodeResult, error) {
	a := mutation.DrainNode
	env := s.effectiveMutationEnv(a, in.Node, in.envOverride)
	var failed k8s.DrainStatus
	res, out, err := runMutationCall(s, mutationCall[drainNodeResult]{
		action: a, target: in.Node, gate: in.confirmGate,
		envelope: func(r mutationResult) drainNodeResult { return drainNodeResult{mutationResult: r} },
		plan: func(p *planned) error {
			status, err := mcpDrainStatusFn(ctx, s.cfg.KubeConfig, env.KubeContext(), in.Node)
			p.resolved = mutation.ResolvedDrain(string(status.Phase))
			return err
		},
		perform: func(p planned) (drainNodeResult, error) {
			status, err := mcpStartDrainFn(ctx, s.cfg.KubeConfig, env.KubeContext(), in.Node)
			if err != nil {
				failed = status
				return drainNodeResult{}, err
			}
			if status.Phase.InProgress() {
				s.driveDrain(env.KubeContext(), in.Node)
			}
			return drainNodeResult{mutationResult: p.ok(), Drain: status}, nil
		},
	})
	if err != nil && failed.Phase != "" {
		// A drain that failed still has a status (phase, restarted
		// services, cordon outcome): return it beside the error.
		res = &sdk.CallToolResult{}
		res.SetError(err)
		out = drainNodeResult{
			mutationResult: mutationResult{Status: "FAILED", Action: a.Verb, Kind: a.Kind, Target: in.Node},
			Drain:          failed,
		}
		return res, out, nil
	}
	return res, out, err
}

// driveDrain carries a node's drain to completion in the background,
// unless this process already drives it.
func (s *Server) driveDrain(kubeContext, node string) {
	key := kubeContext + "/" + node
	if _, driving := s.drainDrivers.LoadOrStore(key, struct{}{}); driving {
		return
	}
	go func() {
		defer s.drainDrivers.Delete(key)
		if err := mcpAwaitDrainFn(s.bgCtx, s.cfg.KubeConfig, kubeContext, node); err != nil {
			s.logger.Errorw("mutation", "action", "drain", "kind", "node", "target", node, "surface", surfaceMCP,
				"phase", "background-failed", "error", err)
			return
		}
		s.logger.Infow("mutation", "action", "drain", "kind", "node", "target", node, "surface", surfaceMCP,
			"phase", "background-done")
	}()
}

func (s *Server) handleDrainStatus(ctx context.Context, req *sdk.CallToolRequest, in drainStatusInput) (*sdk.CallToolResult, k8s.DrainStatus, error) {
	status, err := mcpDrainStatusFn(ctx, s.cfg.KubeConfig, s.envFor(in.envOverride).KubeContext(), in.Node)
	if err != nil {
		return failTool[k8s.DrainStatus]("get drain status of node/"+in.Node, err)
	}
	return &sdk.CallToolResult{}, status, nil
}

// planGPUNode looks the node up (or, given an OCID, takes it as is) into
// *node, reporting the instance a dry run would act on.
func (s *Server) planGPUNode(ctx context.Context, env models.Environment, name, ocid string, node **models.GPUNode) func(*planned) error {
	return func(p *planned) error {
		n, err := mcpResolveGPUNodeFn(ctx, s, env, name, ocid)
		if err != nil {
			return err
		}
		*node = n
		p.resolved = mutation.ResolvedNode(n.ID)
		return nil
	}
}

func (s *Server) handleRestartNode(ctx context.Context, req *sdk.CallToolRequest, in restartNodeInput) (*sdk.CallToolResult, mutationResult, error) {
	a := mutation.RestartNode
	env := s.effectiveMutationEnv(a, in.Node, in.envOverride)
	var node *models.GPUNode
	return runMutationCall(s, mutationCall[mutationResult]{
		action: a, target: in.Node, gate: in.confirmGate, envelope: asResult,
		plan: s.planGPUNode(ctx, env, in.Node, in.OCID, &node),
		perform: func(p planned) (mutationResult, error) {
			return p.ok(), mcpSoftResetFn(ctx, node, env, logging.FromContext(ctx))
		},
	})
}

func (s *Server) handleTerminateNode(ctx context.Context, req *sdk.CallToolRequest, in terminateNodeInput) (*sdk.CallToolResult, mutationResult, error) {
	a := mutation.TerminateNode
	env := s.effectiveMutationEnv(a, in.Node, in.envOverride)
	var node *models.GPUNode
	return runMutationCall(s, mutationCall[mutationResult]{
		action: a, target: in.Node, gate: in.confirmGate, confirmTarget: in.ConfirmTarget, envelope: asResult,
		plan: s.planGPUNode(ctx, env, in.Node, in.OCID, &node),
		perform: func(p planned) (mutationResult, error) {
			return p.ok(), mcpTerminateFn(ctx, node, env, logging.FromContext(ctx))
		},
	})
}

func (s *Server) handleScaleGPUPool(ctx context.Context, req *sdk.CallToolRequest, in scaleGPUPoolInput) (*sdk.CallToolResult, scaleGPUPoolResult, error) {
	a := mutation.ScaleGPUPool
	env := s.effectiveMutationEnv(a, in.Name, in.envOverride)
	var (
		pool   *models.GPUPool
		n      int
		result *oci.PoolScale
	)
	return runMutationCall(s, mutationCall[scaleGPUPoolResult]{
		action: a, target: in.Name, gate: in.confirmGate,
		envelope: func(r mutationResult) scaleGPUPoolResult { return scaleGPUPoolResult{mutationResult: r} },
		plan: func(p *planned) error {
			found, err := mcpResolveGPUPoolFn(ctx, s, env, in.Name)
			if err != nil {
				return err
			}
			if found.ID == "" {
				return fmt.Errorf("pool %s: %w", found.Name, oci.ErrNoInstancePool)
			}
			requested := 0
			if in.Add != nil {
				requested = *in.Add
			}
			if n, err = mutation.PoolScaleAdd(*found, requested, in.Add != nil); err != nil {
				return err
			}
			if n == 0 {
				p.noop = mutation.PoolAtSizeNote(*found, "add")
				return nil
			}
			pool = found
			p.resolved = mutation.ResolvedPool(found.ID, found.Size, found.ActualSize, n)
			p.note = mutation.PoolScaleNotice(*found, n)
			return nil
		},
		auditFields: func() []any {
			if result == nil {
				return []any{"add", n}
			}
			return []any{"add", n, "from", result.Before, "to", result.After}
		},
		perform: func(p planned) (scaleGPUPoolResult, error) {
			scale, err := mcpIncreasePoolSizeFn(ctx, pool, n, env, logging.FromContext(ctx))
			if err != nil {
				return scaleGPUPoolResult{}, err
			}
			result = &scale
			r := scaleGPUPoolResult{mutationResult: p.ok(), Scale: result}
			r.Note = scale.String()
			if w := mutation.PoolOverSizeWarning(pool.Size, scale.After); w != "" {
				r.Note += ". " + w
			}
			return r, nil
		},
	})
}

func (s *Server) handleDeleteDAC(ctx context.Context, req *sdk.CallToolRequest, in deleteDACInput) (*sdk.CallToolResult, mutationResult, error) {
	a := mutation.DeleteDAC
	env := s.effectiveMutationEnv(a, in.Name, in.envOverride)
	dac := &models.DedicatedAICluster{Name: in.Name}
	return runMutationCall(s, mutationCall[mutationResult]{
		action: a, target: in.Name, gate: in.confirmGate, confirmTarget: in.ConfirmTarget, envelope: asResult,
		plan: func(p *planned) error {
			found, err := mcpGetDACFn(ctx, dac, env)
			p.resolved = mutation.ResolvedDAC(found.ID, found.State)
			return err
		},
		perform: func(p planned) (mutationResult, error) {
			return p.ok(), mcpDeleteDACFn(ctx, dac, env, logging.FromContext(ctx))
		},
	})
}

// isvcTarget is the InferenceService target as the agent named it.
func isvcTarget(namespace, name string) string {
	if namespace != "" {
		return namespace + "/" + name
	}
	return name
}

// planISVC resolves the InferenceService's namespace into *ns, so the
// audit log and result always carry <namespace>/<name>, as the CLI's do.
func (s *Server) planISVC(ctx context.Context, env models.Environment, namespace, name string, ns *string) func(*planned) error {
	return func(p *planned) error {
		resolved, err := mcpResolveISVCFn(ctx, s.cfg.KubeConfig, env.KubeContext(), namespace, name)
		if err != nil {
			return err
		}
		*ns, p.target = resolved, resolved+"/"+name
		p.resolved = mutation.ResolvedNamespace(resolved)
		return nil
	}
}

func (s *Server) handleRestartInferenceService(ctx context.Context, req *sdk.CallToolRequest, in restartInferenceServiceInput) (*sdk.CallToolResult, mutationResult, error) {
	a := mutation.RestartInferenceService
	target := isvcTarget(in.Namespace, in.Name)
	env := s.effectiveMutationEnv(a, target, in.envOverride)
	var ns string
	return runMutationCall(s, mutationCall[mutationResult]{
		action: a, target: target, gate: in.confirmGate, envelope: asResult,
		plan: s.planISVC(ctx, env, in.Namespace, in.Name, &ns),
		perform: func(p planned) (mutationResult, error) {
			return p.ok(), mcpRestartISVCFn(ctx, s.cfg.KubeConfig, env.KubeContext(), ns, in.Name)
		},
	})
}

// handleDeleteInferenceService deletes an orphaned endpoint (its DAC is
// gone). Planning also checks it is an orphan and decides between delete
// and force-delete, which names the audited mutation.
func (s *Server) handleDeleteInferenceService(ctx context.Context, req *sdk.CallToolRequest, in deleteInferenceServiceInput) (*sdk.CallToolResult, deleteInferenceServiceResult, error) {
	a := mutation.DeleteInferenceService
	target := isvcTarget(in.Namespace, in.Name)
	env := s.effectiveMutationEnv(a, target, in.envOverride)
	var ns string
	resolveNS := s.planISVC(ctx, env, in.Namespace, in.Name, &ns)
	return runMutationCall(s, mutationCall[deleteInferenceServiceResult]{
		action: a, target: target, gate: in.confirmGate, confirmTarget: in.ConfirmTarget,
		envelope: func(r mutationResult) deleteInferenceServiceResult {
			return deleteInferenceServiceResult{mutationResult: r}
		},
		plan: func(p *planned) error {
			if err := resolveNS(p); err != nil {
				return err
			}
			outcome, err := mcpPlanOrphanISVCFn(ctx, s.cfg.KubeConfig, env.KubeContext(), ns, in.Name)
			if err != nil {
				return err
			}
			p.action = mutation.InferenceServiceDelete(outcome == k8s.OrphanFinalized)
			return nil
		},
		perform: func(p planned) (deleteInferenceServiceResult, error) {
			done, err := mcpDeleteOrphanISVCFn(ctx, s.cfg.KubeConfig, env.KubeContext(), ns, in.Name)
			if err != nil {
				return deleteInferenceServiceResult{}, err
			}
			return deleteInferenceServiceResult{
				mutationResult: mutationOK(done.Action(), p.action.Kind, p.target),
				Summary:        done.Summary(p.target),
			}, nil
		},
	})
}

func (s *Server) handleSetTenant(ctx context.Context, req *sdk.CallToolRequest, in setTenantInput) (*sdk.CallToolResult, mutationResult, error) {
	if strings.TrimSpace(in.Name) == "" {
		return failTool[mutationResult]("set tenant", errors.New("name is required"))
	}
	if !strings.HasPrefix(in.OCID, "ocid1.tenancy.") {
		return failTool[mutationResult]("set tenant",
			fmt.Errorf("invalid tenancy OCID %q: must start with \"ocid1.tenancy.\"", in.OCID))
	}
	name := in.Name
	// A nil is_internal keeps the recorded flag; a new tenant then defaults
	// to internal.
	entry := models.TenantMetadata{ID: in.OCID, Name: &name, IsInternal: in.IsInternal}
	if in.Note != "" {
		note := in.Note
		entry.Note = &note
	}
	return runMutationCall(s, mutationCall[mutationResult]{
		action: mutation.SetTenant, target: in.OCID, gate: in.confirmGate, envelope: asResult,
		// The write's own checks (a name another tenancy has, a file that
		// doesn't load), so a dry run catches them too.
		plan: func(p *planned) error {
			exists, err := mcpCheckTenantFn(s, entry)
			p.resolved = mutation.ResolvedTenant(exists)
			return err
		},
		perform: func(p planned) (mutationResult, error) {
			return p.ok(), mcpUpsertTenantFn(s, entry)
		},
	})
}

// --- Registration -------------------------------------------------

// mutationToolFooter is appended to every mutation tool's description.
// Centralized so the env-override semantics stay in sync across all
// env-scoped tools whenever the policy is revisited.
const mutationToolFooter = " Mutating: requires confirm=true to execute, otherwise refuses without acting;" +
	" dry_run=true instead reports what it would do (status DRY_RUN, with the looked-up target under resolved)." +
	" Accepts env_type/env_region/env_realm, but per-call overrides take effect ONLY if the server" +
	" was started with --" + config.FlagMutationEnvOverrideAllowed + ". By default override fields are ignored" +
	" (audit-logged at info) and the startup env is used."

// irreversibleFooter is appended to the description of every tool whose
// action is mutation.Irreversible, after mutationToolFooter.
const irreversibleFooter = " IRREVERSIBLE: confirm=true is not enough; also set confirm_target to the target exactly," +
	" as you named it or as dry_run reports it, else the call is refused."

// registerMutationTools adds the mutating tools. Each requires
// confirm=true at the input level; the tool description tells the
// agent explicitly so the contract is discoverable without running
// the tool to see the refusal.
func registerMutationTools(s *Server) {
	// Every mutation tool's input schema includes env_type/env_region/
	// env_realm for parity with the read-only list_* tools, but the
	// override only takes effect when the operator started this server
	// with --mutation-env-override-allowed. By default the agent's
	// override fields are ignored (and audit-logged at info), so the
	// safety story remains: the operator's startup-env choice caps
	// blast radius, and confirm=true gates each individual mutation.
	// Operators who want to give the agent multi-realm authority can
	// flip the flag — the risk is documented on the flag itself.
	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "cordon_node",
		Annotations: idempotent(),
		Description: "Cordon (mark unschedulable) a Kubernetes node. Idempotent." + mutationToolFooter,
	}, s.handleCordonNode)

	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "uncordon_node",
		Annotations: idempotent(),
		Description: "Uncordon (mark schedulable) a Kubernetes node. Idempotent." + mutationToolFooter,
	}, s.handleUncordonNode)

	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "drain_node",
		Annotations: disruptive(),
		Description: "Safely drain a node: cordon; refuse if it runs fine-tuning, training, or replicationJob pods;" +
			" restart InferenceServices with minReplicas<=1 and wait (up to 20m) for them to reschedule elsewhere;" +
			" then evict the rest (up to 10m). On failure before eviction, uncordons the node unless it was already cordoned; once eviction starts it stays cordoned." +
			" Asynchronous: returns at once with the drain's status (phase starting/waiting/evicting/done/failed) while the server drives it in the background;" +
			" poll get_drain_status for progress. The drain is recorded on the Node, so calling drain_node again resumes it (e.g. after a server restart)" +
			" without repeating finished steps. Use before terminate." + mutationToolFooter,
	}, s.handleDrainNode)

	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "restart_node",
		Annotations: recoverable(),
		Description: "Soft-reset the OCI instance backing a GPU node. Fire-and-forget." + mutationToolFooter,
	}, s.handleRestartNode)

	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "terminate_node",
		Annotations: destructive(),
		Description: "Terminate the OCI instance backing a GPU node (boot volume destroyed). DESTRUCTIVE." + mutationToolFooter + irreversibleFooter,
	}, s.handleTerminateNode)

	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "scale_gpu_pool",
		Annotations: recoverable(),
		Description: "Grow the named GPU pool's OCI instance pool. Without add, fills it to its Terraform-declared size (status NOOP when already there)." +
			" With add=N, adds N instances to OCI's current size, possibly past the Terraform size: the next terraform apply shrinks it back and terminates the extra instances." +
			" Each call with add grows the pool again. The result's scale reports the sizes it went from and to." + mutationToolFooter,
	}, s.handleScaleGPUPool)

	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "delete_dac",
		Annotations: destructive(),
		Description: "Delete a dedicated AI cluster and its endpoints (synchronous, polls the work request). DESTRUCTIVE." + mutationToolFooter + irreversibleFooter,
	}, s.handleDeleteDAC)

	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "restart_inference_service",
		Annotations: recoverable(),
		Description: "Trigger a rolling restart of an OME InferenceService by bumping its restartedAt annotation. Fire-and-forget." +
			" Omit namespace to search all namespaces; an ambiguous name is refused." + mutationToolFooter,
	}, s.handleRestartInferenceService)

	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "delete_inference_service",
		Annotations: destructive(),
		Description: "Delete an orphaned OME InferenceService: a dedicated endpoint whose DAC no longer exists (doctor --relations reports it dangling)." +
			" Refused for shared endpoints and for endpoints whose DAC exists (delete_dac owns those)." +
			" An endpoint stuck deleting for over 10 minutes is force-deleted by removing its finalizer; summary says when a second call is needed." +
			" Find candidates with list_inference_services faulty=true or check_integrity." +
			" Omit namespace to search all namespaces; an ambiguous name is refused. DESTRUCTIVE." + mutationToolFooter + irreversibleFooter,
	}, s.handleDeleteInferenceService)

	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "set_tenant",
		Annotations: disruptive(),
		Description: "Create or update a tenancy's metadata (name / internal flag / note), keyed by full tenancy OCID in the global metadata file." +
			" Mutating: requires confirm=true to execute, otherwise refuses without acting; dry_run=true instead makes the write's checks and reports what it would do (status DRY_RUN, resolved.exists says whether the entry is replaced)." +
			" Not env-scoped: env_type/env_region/env_realm do not apply.",
	}, s.handleSetTenant)
}
