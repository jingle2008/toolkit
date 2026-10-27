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
	"github.com/jingle2008/toolkit/internal/mutation/ops"
	"github.com/jingle2008/toolkit/internal/resolve"
	"github.com/jingle2008/toolkit/pkg/models"
)

// productionDeps is the mutations' infra calls for s: the live cluster
// and OCI, with lookups through s's loader and metadata file.
func productionDeps(s *Server) *ops.Deps {
	d := ops.Production()
	d.ResolveGPUNode = func(ctx context.Context, env models.Environment, name, ocid string) (*models.GPUNode, error) {
		return resolve.GPUNode(ctx, s.loader, s.cfg.RepoPath, s.cfg.KubeConfig, env, name, ocid)
	}
	d.ResolveGPUPool = func(ctx context.Context, env models.Environment, name string) (*models.GPUPool, error) {
		return resolve.GPUPool(ctx, s.loader, s.cfg.RepoPath, s.cfg.KubeConfig, env, name)
	}
	d.CheckTenant = func(entry models.TenantMetadata) (bool, error) {
		if s.cfg.MetadataFile == "" {
			return false, errors.New("no metadata file configured")
		}
		return configloader.CheckTenantUpsert(s.cfg.MetadataFile, entry)
	}
	d.UpsertTenant = func(_ context.Context, entry models.TenantMetadata) error {
		writer, ok := s.loader.(loader.TenantMetadataWriter)
		if !ok {
			return errors.New("loader does not support writing metadata")
		}
		return writer.UpsertTenantMetadata(entry)
	}
	return &d
}

// opsEnv is where a mutation acts, in env.
func (s *Server) opsEnv(env models.Environment) ops.Env {
	return ops.Env{KubeConfig: s.cfg.KubeConfig, Env: env}
}

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
	ConfirmTarget string `json:"confirm_target,omitempty" jsonschema:"required to execute this irreversible action: the target repeated exactly, as you named it or as dry_run reports it (for a node given by ocid, the OCID)"`
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
			s.logger.Info(
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
		s.logger.Warn(
			"mutation env override active (deviation from startup)",
			"action", a.Verb, "kind", a.Kind, "target", target, "surface", surfaceMCP,
			"startup_realm", startup.Realm, "effective_realm", effective.Realm,
			"startup_region", startup.Region, "effective_region", effective.Region,
			"startup_type", startup.Type, "effective_type", effective.Type,
		)
	}
	return effective
}

// planOK is the success envelope for p.
func planOK(p mutation.Plan) mutationResult {
	return mutationOK(p.Action.Verb, p.Action.Kind, p.Target)
}

// mutationCall is one mutating tool call, as its handler describes it.
type mutationCall[Out any] struct {
	// op is the mutation, its action and target as the agent named them.
	op   mutation.Op
	gate confirmGate
	// confirmTarget is the agent's irreversibleGate.ConfirmTarget,
	// checked when the planned action is Irreversible.
	confirmTarget string
	// envelope builds the tool's result from a bare envelope, for the
	// dry-run and NOOP responses, and for success when result is unset.
	envelope func(mutationResult) Out
	// result, when set, builds the success result from the plan as
	// performed.
	result func(p mutation.Plan) Out
}

// asResult is the envelope for tools whose result is the bare envelope.
func asResult(r mutationResult) mutationResult { return r }

// runMutationCall is the whole MCP mutation flow, shared by every
// mutating tool. A blank target is refused first, by name, rather than
// passed to the cluster or OCI: the schema marks it required but accepts
// "". An InferenceService target is <namespace>/<name>, so the name is
// what follows the last slash. Then runGatedMutation.
func runMutationCall[Out any](ctx context.Context, s *Server, c mutationCall[Out]) (*sdk.CallToolResult, Out, error) {
	requested := c.op.Plan
	if strings.TrimSpace(requested.Target[strings.LastIndex(requested.Target, "/")+1:]) == "" {
		return failTool[Out](requested.Action.Verb, fmt.Errorf("%s target is required", requested.Action.Kind))
	}
	return runGatedMutation(ctx, s, c)
}

// runGatedMutation runs a mutation call through mutation.Run, the flow
// the CLI's runMutation shares, with MCP's gate:
//
//   - Neither confirm nor dry_run: refused, before touching anything.
//   - An Irreversible action whose confirm_target names neither the
//     requested nor the resolved target: refused, after a dry run would
//     have reported.
//
// The refusal and failure paths return an error, which the SDK renders
// as IsError plus text; success is Out, which the SDK marshals into
// StructuredContent and auto-emits as TextContent.
func runGatedMutation[Out any](ctx context.Context, s *Server, c mutationCall[Out]) (*sdk.CallToolResult, Out, error) {
	var unadmitted bool
	requested := c.op.Plan.Target
	gate := mutation.Gate{
		DryRun: c.gate.DryRun,
		Admit: func(p mutation.Plan) error {
			if c.gate.Confirm || c.gate.DryRun {
				return nil
			}
			unadmitted = true
			return mutation.Refuse(fmt.Errorf("mutating tool requires confirm=true (target %s/%s)", p.Action.Kind, p.Target))
		},
		Confirm: func(_ context.Context, p mutation.Plan) error {
			if p.Action.Tier == mutation.Irreversible &&
				(c.confirmTarget == "" || (c.confirmTarget != requested && c.confirmTarget != p.Target)) {
				return mutation.Refuse(fmt.Errorf(
					"irreversible: confirm=true is not enough; also set confirm_target to %q", p.Target,
				))
			}
			return nil
		},
	}
	p, outcome, err := mutation.Run(ctx, s.logger, surfaceMCP, gate, c.op)
	if err != nil {
		if unadmitted {
			return failTool[Out](p.Action.Verb, err)
		}
		return failTool[Out](p.String(), err)
	}
	switch outcome {
	case mutation.Noop:
		r := planOK(p)
		r.Status, r.Note = "NOOP", p.Noop
		return &sdk.CallToolResult{}, c.envelope(r), nil
	case mutation.DryRun:
		r := planOK(p)
		r.Status, r.Resolved, r.Note = "DRY_RUN", p.Resolved, p.Notice
		return &sdk.CallToolResult{}, c.envelope(r), nil
	case mutation.Performed:
	}
	out := c.envelope(planOK(p))
	if c.result != nil {
		out = c.result(p)
	}
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
	OCID string `json:"ocid,omitempty" jsonschema:"skip the node lookup (for a node that left the cluster) and target this instance OCID; it must be an instance of one of the environment's GPU instance pools, in its GPU compartment, else the call is refused. The OCID, not node, is then the target: the result, the audit record and confirm_target name it"`
	confirmGate
	envOverride
}

type terminateNodeInput struct {
	Node string `json:"node" jsonschema:"the node name as reported by kubectl get nodes"`
	OCID string `json:"ocid,omitempty" jsonschema:"skip the node lookup (for a node that left the cluster) and target this instance OCID; it must be an instance of one of the environment's GPU instance pools, in its GPU compartment, else the call is refused. The OCID, not node, is then the target: the result, the audit record and confirm_target name it"`
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
	op, _ := ops.Cordon(*s.deps, s.opsEnv(env), in.Node, unschedulable)
	return runMutationCall(ctx, s, mutationCall[mutationResult]{op: op, gate: in.confirmGate, envelope: asResult})
}

// handleDrainNode starts (or resumes) the drain and returns at once,
// leaving a background driver to carry it through. The drain lives on
// the Node, so a driver lost to a server restart is replaced by calling
// drain_node again.
func (s *Server) handleDrainNode(ctx context.Context, req *sdk.CallToolRequest, in drainNodeInput) (*sdk.CallToolResult, drainNodeResult, error) {
	a := mutation.DrainNode
	env := s.effectiveMutationEnv(a, in.Node, in.envOverride)
	var status, failed k8s.DrainStatus
	op := ops.Drain(*s.deps, s.opsEnv(env), in.Node)
	op.Perform = func(ctx context.Context, _ mutation.Plan) error {
		var err error
		status, err = s.deps.StartDrain(ctx, s.cfg.KubeConfig, env.KubeContext(), in.Node)
		if err != nil {
			failed = status
			return err
		}
		if status.Phase.InProgress() {
			s.driveDrain(env.KubeContext(), in.Node)
		}
		return nil
	}
	res, out, err := runMutationCall(ctx, s, mutationCall[drainNodeResult]{
		op: op, gate: in.confirmGate,
		envelope: func(r mutationResult) drainNodeResult { return drainNodeResult{mutationResult: r} },
		result: func(p mutation.Plan) drainNodeResult {
			return drainNodeResult{mutationResult: planOK(p), Drain: status}
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
		if err := s.deps.AwaitDrain(s.bgCtx, s.cfg.KubeConfig, kubeContext, node); err != nil {
			mutation.Audit(s.logger, surfaceMCP, mutation.DrainNode, node, mutation.PhaseBackgroundFailed, err)
			return
		}
		mutation.Audit(s.logger, surfaceMCP, mutation.DrainNode, node, mutation.PhaseBackgroundDone, nil)
	}()
}

func (s *Server) handleDrainStatus(ctx context.Context, req *sdk.CallToolRequest, in drainStatusInput) (*sdk.CallToolResult, k8s.DrainStatus, error) {
	if strings.TrimSpace(in.Node) == "" {
		return failTool[k8s.DrainStatus]("get drain status", errors.New("node is required"))
	}
	status, err := s.deps.DrainStatus(ctx, s.cfg.KubeConfig, s.envFor(in.envOverride).KubeContext(), in.Node)
	if err != nil {
		return failTool[k8s.DrainStatus]("get drain status of node/"+in.Node, err)
	}
	return &sdk.CallToolResult{}, status, nil
}

func (s *Server) handleRestartNode(ctx context.Context, req *sdk.CallToolRequest, in restartNodeInput) (*sdk.CallToolResult, mutationResult, error) {
	env := s.effectiveMutationEnv(mutation.RestartNode, ops.NodeTarget(in.Node, in.OCID), in.envOverride)
	op := ops.RestartNode(*s.deps, s.opsEnv(env), in.Node, in.OCID)
	return runMutationCall(ctx, s, mutationCall[mutationResult]{op: op, gate: in.confirmGate, envelope: asResult})
}

func (s *Server) handleTerminateNode(ctx context.Context, req *sdk.CallToolRequest, in terminateNodeInput) (*sdk.CallToolResult, mutationResult, error) {
	env := s.effectiveMutationEnv(mutation.TerminateNode, ops.NodeTarget(in.Node, in.OCID), in.envOverride)
	op := ops.TerminateNode(*s.deps, s.opsEnv(env), in.Node, in.OCID)
	return runMutationCall(ctx, s, mutationCall[mutationResult]{
		op: op, gate: in.confirmGate, confirmTarget: in.ConfirmTarget, envelope: asResult,
	})
}

func (s *Server) handleScaleGPUPool(ctx context.Context, req *sdk.CallToolRequest, in scaleGPUPoolInput) (*sdk.CallToolResult, scaleGPUPoolResult, error) {
	env := s.effectiveMutationEnv(mutation.ScaleGPUPool, in.Name, in.envOverride)
	add := 0
	if in.Add != nil {
		add = *in.Add
	}
	op, scale := ops.ScalePool(*s.deps, s.opsEnv(env), in.Name, add, in.Add != nil, "add")
	return runMutationCall(ctx, s, mutationCall[scaleGPUPoolResult]{
		op: op, gate: in.confirmGate,
		envelope: func(r mutationResult) scaleGPUPoolResult { return scaleGPUPoolResult{mutationResult: r} },
		result: func(p mutation.Plan) scaleGPUPoolResult {
			r := scaleGPUPoolResult{mutationResult: planOK(p), Scale: scale.Result}
			r.Note = scale.Summary()
			return r
		},
	})
}

func (s *Server) handleDeleteDAC(ctx context.Context, req *sdk.CallToolRequest, in deleteDACInput) (*sdk.CallToolResult, mutationResult, error) {
	env := s.effectiveMutationEnv(mutation.DeleteDAC, in.Name, in.envOverride)
	return runMutationCall(ctx, s, mutationCall[mutationResult]{
		op:   ops.DeleteDAC(*s.deps, s.opsEnv(env), in.Name),
		gate: in.confirmGate, confirmTarget: in.ConfirmTarget, envelope: asResult,
	})
}

func (s *Server) handleRestartInferenceService(ctx context.Context, req *sdk.CallToolRequest, in restartInferenceServiceInput) (*sdk.CallToolResult, mutationResult, error) {
	env := s.effectiveMutationEnv(mutation.RestartInferenceService, ops.ISVCTarget(in.Namespace, in.Name), in.envOverride)
	return runMutationCall(ctx, s, mutationCall[mutationResult]{
		op:   ops.RestartISVC(*s.deps, s.opsEnv(env), in.Namespace, in.Name),
		gate: in.confirmGate, envelope: asResult,
	})
}

// handleDeleteInferenceService deletes an orphaned endpoint (its DAC is
// gone). Planning also checks it is an orphan and decides between delete
// and force-delete, which names the audited mutation.
func (s *Server) handleDeleteInferenceService(ctx context.Context, req *sdk.CallToolRequest, in deleteInferenceServiceInput) (*sdk.CallToolResult, deleteInferenceServiceResult, error) {
	env := s.effectiveMutationEnv(mutation.DeleteInferenceService, ops.ISVCTarget(in.Namespace, in.Name), in.envOverride)
	op, done := ops.DeleteOrphanISVC(*s.deps, s.opsEnv(env), in.Namespace, in.Name)
	return runMutationCall(ctx, s, mutationCall[deleteInferenceServiceResult]{
		op: op, gate: in.confirmGate, confirmTarget: in.ConfirmTarget,
		envelope: func(r mutationResult) deleteInferenceServiceResult {
			return deleteInferenceServiceResult{mutationResult: r}
		},
		result: func(p mutation.Plan) deleteInferenceServiceResult {
			return deleteInferenceServiceResult{
				mutationResult: mutationOK(done.Action(), p.Action.Kind, p.Target),
				Summary:        done.Summary(p.Target),
			}
		},
	})
}

func (s *Server) handleSetTenant(ctx context.Context, req *sdk.CallToolRequest, in setTenantInput) (*sdk.CallToolResult, mutationResult, error) {
	if strings.TrimSpace(in.Name) == "" {
		return failTool[mutationResult]("set tenant", errors.New("name is required"))
	}
	// A nil is_internal keeps the recorded flag; a new tenant then defaults
	// to internal.
	entry, err := ops.TenantEntry(in.OCID, in.Name, in.IsInternal, in.Note)
	if err != nil {
		return failTool[mutationResult]("set tenant", err)
	}
	return runMutationCall(ctx, s, mutationCall[mutationResult]{
		op: ops.SetTenant(*s.deps, entry), gate: in.confirmGate, envelope: asResult,
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

// ocidToolNote is in the description of the node tools that take an ocid.
const ocidToolNote = " With ocid, the instance is targeted without a cluster lookup, but only if it is in one of" +
	" the environment's GPU instance pools; the OCID is then the target."

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
		Description: "Soft-reset the OCI instance backing a GPU node. Fire-and-forget." + ocidToolNote + mutationToolFooter,
	}, s.handleRestartNode)

	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "terminate_node",
		Annotations: destructive(),
		Description: "Terminate the OCI instance backing a GPU node (boot volume destroyed). DESTRUCTIVE." + ocidToolNote +
			mutationToolFooter + irreversibleFooter + " With ocid set, confirm_target must be that OCID, not the node name.",
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
