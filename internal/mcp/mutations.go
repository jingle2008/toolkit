package mcp

import (
	"context"
	"errors"
	"fmt"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jingle2008/toolkit/internal/config"
	"github.com/jingle2008/toolkit/internal/infra/k8s"
	"github.com/jingle2008/toolkit/internal/infra/loader"
	"github.com/jingle2008/toolkit/internal/resolve"
	"github.com/jingle2008/toolkit/internal/ui/tui/actions"
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
	mcpStartDrainFn       = k8s.StartDrain
	mcpAwaitDrainFn       = k8s.AwaitDrain
	mcpDrainStatusFn      = k8s.GetDrainStatus
	mcpSoftResetFn        = actions.SoftResetInstance
	mcpTerminateFn        = actions.TerminateInstance
	mcpIncreasePoolSizeFn = actions.IncreasePoolSize
	mcpDeleteDACFn        = actions.DeleteDedicatedAICluster
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
	mcpUpsertTenantFn = func(s *Server, entry models.TenantMetadata) error {
		writer, ok := s.loader.(loader.TenantMetadataWriter)
		if !ok {
			return errors.New("loader does not support writing metadata")
		}
		return writer.UpsertTenantMetadata(entry)
	}
)

// confirmGate is embedded in every mutating tool's input. The field
// is OPTIONAL at the JSON-Schema level (omitempty) so the SDK passes
// the call to the handler even when confirm is missing — that way we
// can audit-log the attempt and return an error explaining the
// contract. Only confirm=true triggers execution.
type confirmGate struct {
	Confirm bool `json:"confirm,omitempty" jsonschema:"set true to execute; otherwise the tool refuses without acting"`
}

// effectiveMutationEnv applies the agent's envOverride only when the
// operator opted into per-call overrides via
// MutationEnvOverrideAllowed. Otherwise the override is ignored and
// the startup env is used unchanged. When the override IS applied and
// changes the effective env, that's audit-logged for SIEM visibility.
func (s *Server) effectiveMutationEnv(action, kind, target string, in envOverride) models.Environment {
	startup := s.envFor(envOverride{})
	if !s.cfg.MutationEnvOverrideAllowed {
		if in.EnvType != "" || in.EnvRegion != "" || in.EnvRealm != "" {
			s.logger.Infow(
				"mutation env_override ignored (server disallows)",
				"action", action, "kind", kind, "target", target, "surface", "mcp",
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
			"action", action, "kind", kind, "target", target, "surface", "mcp",
			"startup_realm", startup.Realm, "effective_realm", effective.Realm,
			"startup_region", startup.Region, "effective_region", effective.Region,
			"startup_type", startup.Type, "effective_type", effective.Type,
		)
	}
	return effective
}

// runMutationTool wraps the entire MCP mutation flow: refuse if
// confirm is false, audit-log begin/refused/failed/done, and return the
// standard envelope on success.
//
// Mirrors cli.runMutation but adapted to the MCP response shape —
// no stdout/prompt; success becomes a mutationResult that the SDK
// marshals into StructuredContent + auto-emits as TextContent. The
// refusal and failure paths return an error, which the SDK renders as
// IsError plus text.
//
// Takes no ctx/req: `perform` closes over whatever it needs from the
// call site, and nothing here talks to the session any more now that
// the notifications/message emission is gone.
func (s *Server) runMutationTool(action, kind, target string, confirm bool, perform func() error) (*sdk.CallToolResult, mutationResult, error) {
	return runMutationToolWith(s, action, kind, target, confirm, func() (mutationResult, error) {
		if err := perform(); err != nil {
			return mutationResult{}, err
		}
		return mutationOK(action, kind, target), nil
	})
}

// runMutationToolWith is runMutationTool for tools whose success result
// carries more than the standard envelope.
func runMutationToolWith[Out any](s *Server, action, kind, target string, confirm bool, perform func() (Out, error)) (*sdk.CallToolResult, Out, error) {
	if !confirm {
		s.logger.Infow(
			"mutation",
			"action", action, "kind", kind, "target", target, "surface", "mcp",
			"phase", "refused",
		)
		return failTool[Out](action,
			fmt.Errorf("mutating tool requires confirm=true (target %s/%s)", kind, target))
	}

	s.logger.Infow(
		"mutation",
		"action", action, "kind", kind, "target", target, "surface", "mcp",
		"phase", "begin",
	)
	out, err := perform()
	if err != nil {
		s.logger.Errorw(
			"mutation",
			"action", action, "kind", kind, "target", target, "surface", "mcp",
			"phase", "failed",
			"error", err,
		)
		return failTool[Out](action+" "+kind+"/"+target, err)
	}
	s.logger.Infow(
		"mutation",
		"action", action, "kind", kind, "target", target, "surface", "mcp",
		"phase", "done",
	)
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
	envOverride
}

type scaleGPUPoolInput struct {
	Name string `json:"name" jsonschema:"the pool name from the Terraform repo (same as toolkit get gpupool)"`
	confirmGate
	envOverride
}

type deleteDACInput struct {
	Name string `json:"name" jsonschema:"the DAC name (same identifier as toolkit get dac shows)"`
	confirmGate
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
	IsInternal *bool  `json:"is_internal,omitempty" jsonschema:"mark tenant internal; defaults to true when omitted"`
	Note       string `json:"note,omitempty" jsonschema:"optional free-form note"`
	confirmGate
}

// --- Handlers -----------------------------------------------------

// handleMutation is the shared entry point for every mutating tool:
// derive the effective env (audit-logging any override), then dispatch
// through runMutationTool which enforces the confirm gate and emits the
// standard audit-log plus response envelope. Handlers supply only the
// action/kind/target labels plus the env-scoped perform closure, which
// closes over the handler's own ctx.
func (s *Server) handleMutation(
	action, kind, target string,
	confirm bool,
	override envOverride,
	perform func(env models.Environment) error,
) (*sdk.CallToolResult, mutationResult, error) {
	env := s.effectiveMutationEnv(action, kind, target, override)
	return s.runMutationTool(action, kind, target, confirm, func() error {
		return perform(env)
	})
}

func (s *Server) handleCordonNode(ctx context.Context, req *sdk.CallToolRequest, in cordonNodeInput) (*sdk.CallToolResult, mutationResult, error) {
	return s.handleMutation("cordon", "node", in.Node, in.Confirm, in.envOverride, func(env models.Environment) error {
		_, err := mcpSetCordonFn(ctx, s.cfg.KubeConfig, env.KubeContext(), in.Node, true)
		return err
	})
}

func (s *Server) handleUncordonNode(ctx context.Context, req *sdk.CallToolRequest, in cordonNodeInput) (*sdk.CallToolResult, mutationResult, error) {
	return s.handleMutation("uncordon", "node", in.Node, in.Confirm, in.envOverride, func(env models.Environment) error {
		_, err := mcpSetCordonFn(ctx, s.cfg.KubeConfig, env.KubeContext(), in.Node, false)
		return err
	})
}

// handleDrainNode starts (or resumes) the drain and returns at once,
// leaving a background driver to carry it through. The drain lives on
// the Node, so a driver lost to a server restart is replaced by calling
// drain_node again.
func (s *Server) handleDrainNode(ctx context.Context, req *sdk.CallToolRequest, in drainNodeInput) (*sdk.CallToolResult, drainNodeResult, error) {
	const action, kind = "drain", "node"
	env := s.effectiveMutationEnv(action, kind, in.Node, in.envOverride)
	var failed k8s.DrainStatus
	res, out, err := runMutationToolWith(s, action, kind, in.Node, in.Confirm, func() (drainNodeResult, error) {
		status, err := mcpStartDrainFn(ctx, s.cfg.KubeConfig, env.KubeContext(), in.Node)
		if err != nil {
			failed = status
			return drainNodeResult{}, err
		}
		if status.Phase.InProgress() {
			s.driveDrain(env.KubeContext(), in.Node)
		}
		return drainNodeResult{mutationResult: mutationOK(action, kind, in.Node), Drain: status}, nil
	})
	if err != nil && failed.Phase != "" {
		// A drain that failed still has a status (phase, restarted
		// services, cordon outcome): return it beside the error.
		res = &sdk.CallToolResult{}
		res.SetError(err)
		out = drainNodeResult{
			mutationResult: mutationResult{Status: "FAILED", Action: action, Kind: kind, Target: in.Node},
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
			s.logger.Errorw("mutation", "action", "drain", "kind", "node", "target", node, "surface", "mcp",
				"phase", "background-failed", "error", err)
			return
		}
		s.logger.Infow("mutation", "action", "drain", "kind", "node", "target", node, "surface", "mcp",
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

func (s *Server) handleRestartNode(ctx context.Context, req *sdk.CallToolRequest, in restartNodeInput) (*sdk.CallToolResult, mutationResult, error) {
	return s.handleMutation("restart", "node", in.Node, in.Confirm, in.envOverride, func(env models.Environment) error {
		node, err := mcpResolveGPUNodeFn(ctx, s, env, in.Node, in.OCID)
		if err != nil {
			return err
		}
		return mcpSoftResetFn(ctx, node, env, logging.FromContext(ctx))
	})
}

func (s *Server) handleTerminateNode(ctx context.Context, req *sdk.CallToolRequest, in terminateNodeInput) (*sdk.CallToolResult, mutationResult, error) {
	return s.handleMutation("terminate", "node", in.Node, in.Confirm, in.envOverride, func(env models.Environment) error {
		node, err := mcpResolveGPUNodeFn(ctx, s, env, in.Node, in.OCID)
		if err != nil {
			return err
		}
		return mcpTerminateFn(ctx, node, env, logging.FromContext(ctx))
	})
}

func (s *Server) handleScaleGPUPool(ctx context.Context, req *sdk.CallToolRequest, in scaleGPUPoolInput) (*sdk.CallToolResult, mutationResult, error) {
	return s.handleMutation("scale", "gpu_pool", in.Name, in.Confirm, in.envOverride, func(env models.Environment) error {
		pool, err := mcpResolveGPUPoolFn(ctx, s, env, in.Name)
		if err != nil {
			return err
		}
		return mcpIncreasePoolSizeFn(ctx, pool, env, logging.FromContext(ctx))
	})
}

func (s *Server) handleDeleteDAC(ctx context.Context, req *sdk.CallToolRequest, in deleteDACInput) (*sdk.CallToolResult, mutationResult, error) {
	return s.handleMutation("delete", "dac", in.Name, in.Confirm, in.envOverride, func(env models.Environment) error {
		dac := &models.DedicatedAICluster{Name: in.Name}
		return mcpDeleteDACFn(ctx, dac, env, logging.FromContext(ctx))
	})
}

// handleRestartInferenceService resolves the namespace before the audited
// mutation, as the CLI does, so the audit log and result always carry
// <namespace>/<name>. A refused call (no confirm) is not resolved: refusal
// must not touch the cluster.
func (s *Server) handleRestartInferenceService(ctx context.Context, req *sdk.CallToolRequest, in restartInferenceServiceInput) (*sdk.CallToolResult, mutationResult, error) {
	const action, kind = "restart", "inference_service"
	target := in.Name
	if in.Namespace != "" {
		target = in.Namespace + "/" + in.Name
	}
	env := s.effectiveMutationEnv(action, kind, target, in.envOverride)
	ns := in.Namespace
	if in.Confirm {
		resolved, err := mcpResolveISVCFn(ctx, s.cfg.KubeConfig, env.KubeContext(), in.Namespace, in.Name)
		if err != nil {
			return failTool[mutationResult](action+" "+kind+"/"+target, err)
		}
		ns, target = resolved, resolved+"/"+in.Name
	}
	return s.runMutationTool(action, kind, target, in.Confirm, func() error {
		return mcpRestartISVCFn(ctx, s.cfg.KubeConfig, env.KubeContext(), ns, in.Name)
	})
}

// handleDeleteInferenceService deletes an orphaned endpoint (its DAC is
// gone). As for restart, the target is resolved — and here also checked
// and planned — only once confirmed, so a refused call never touches the
// cluster; the planned action (delete or force-delete) names the audited
// mutation.
func (s *Server) handleDeleteInferenceService(ctx context.Context, req *sdk.CallToolRequest, in deleteInferenceServiceInput) (*sdk.CallToolResult, deleteInferenceServiceResult, error) {
	const kind = "inference_service"
	action, target := "delete", in.Name
	if in.Namespace != "" {
		target = in.Namespace + "/" + in.Name
	}
	env := s.effectiveMutationEnv(action, kind, target, in.envOverride)
	ns := in.Namespace
	if in.Confirm {
		resolved, err := mcpResolveISVCFn(ctx, s.cfg.KubeConfig, env.KubeContext(), in.Namespace, in.Name)
		if err != nil {
			return failTool[deleteInferenceServiceResult](action+" "+kind+"/"+target, err)
		}
		ns, target = resolved, resolved+"/"+in.Name
		outcome, err := mcpPlanOrphanISVCFn(ctx, s.cfg.KubeConfig, env.KubeContext(), ns, in.Name)
		if err != nil {
			return failTool[deleteInferenceServiceResult](action+" "+kind+"/"+target, err)
		}
		action = outcome.Action()
	}
	return runMutationToolWith(s, action, kind, target, in.Confirm, func() (deleteInferenceServiceResult, error) {
		done, err := mcpDeleteOrphanISVCFn(ctx, s.cfg.KubeConfig, env.KubeContext(), ns, in.Name)
		if err != nil {
			return deleteInferenceServiceResult{}, err
		}
		return deleteInferenceServiceResult{mutationResult: mutationOK(done.Action(), kind, target), Summary: done.Summary(target)}, nil
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
	internal := true
	if in.IsInternal != nil {
		internal = *in.IsInternal
	}
	name := in.Name
	entry := models.TenantMetadata{ID: in.OCID, Name: &name, IsInternal: &internal}
	if in.Note != "" {
		note := in.Note
		entry.Note = &note
	}
	return s.runMutationTool("set", "tenant", in.OCID, in.Confirm, func() error {
		return mcpUpsertTenantFn(s, entry)
	})
}

// --- Registration -------------------------------------------------

// mutationToolFooter is appended to every mutation tool's description.
// Centralized so the env-override semantics stay in sync across all
// env-scoped tools whenever the policy is revisited.
const mutationToolFooter = " Mutating: requires confirm=true to execute, otherwise refuses without acting." +
	" Accepts env_type/env_region/env_realm, but per-call overrides take effect ONLY if the server" +
	" was started with --" + config.FlagMutationEnvOverrideAllowed + ". By default override fields are ignored" +
	" (audit-logged at info) and the startup env is used."

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
		Annotations: idempotent(),
		Description: "Safely drain a node: cordon; refuse if it runs fine-tuning, training, or replicationJob pods;" +
			" restart InferenceServices with minReplicas<=1 and wait (up to 20m) for them to reschedule elsewhere;" +
			" then evict the rest (up to 10m). On failure before eviction, uncordons the node unless it was already cordoned; once eviction starts it stays cordoned." +
			" Asynchronous: returns at once with the drain's status (phase starting/waiting/evicting/done/failed) while the server drives it in the background;" +
			" poll get_drain_status for progress. The drain is recorded on the Node, so calling drain_node again resumes it (e.g. after a server restart)" +
			" without repeating finished steps. Use before terminate." + mutationToolFooter,
	}, s.handleDrainNode)

	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "get_drain_status",
		Annotations: readOnly(),
		Description: "Read-only: report a node's drain as recorded on the Node by drain_node (or the CLI/TUI): phase" +
			" (none/starting/waiting/evicting/done/failed), startedAt, the restarted InferenceServices, the current phase's deadline," +
			" what it still waits on (remaining), and the failure message. Does not advance the drain: if the phase stays" +
			" in progress past its deadline, no process is driving it, so call drain_node again to resume." +
			" Accepts env_type/env_region/env_realm overrides.",
	}, s.handleDrainStatus)

	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "restart_node",
		Annotations: recoverable(),
		Description: "Soft-reset the OCI instance backing a GPU node. Fire-and-forget." + mutationToolFooter,
	}, s.handleRestartNode)

	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "terminate_node",
		Annotations: destructive(),
		Description: "Terminate the OCI instance backing a GPU node (boot volume destroyed). DESTRUCTIVE." + mutationToolFooter,
	}, s.handleTerminateNode)

	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "scale_gpu_pool",
		Annotations: idempotent(),
		Description: "Push the Terraform-declared pool.Size to OCI for the named GPU pool. No size override: Terraform is the source of truth." + mutationToolFooter,
	}, s.handleScaleGPUPool)

	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "delete_dac",
		Annotations: destructive(),
		Description: "Delete a dedicated AI cluster and its endpoints (synchronous, polls the work request). DESTRUCTIVE." + mutationToolFooter,
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
			" Omit namespace to search all namespaces; an ambiguous name is refused. DESTRUCTIVE." + mutationToolFooter,
	}, s.handleDeleteInferenceService)

	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "set_tenant",
		Annotations: idempotent(),
		Description: "Create or update a tenancy's metadata (name / internal flag / note), keyed by full tenancy OCID in the global metadata file." +
			" Mutating: requires confirm=true to execute, otherwise refuses without acting." +
			" Not env-scoped: env_type/env_region/env_realm do not apply.",
	}, s.handleSetTenant)
}
