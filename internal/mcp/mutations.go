package mcp

import (
	"context"
	"fmt"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jingle2008/toolkit/internal/infra/k8s"
	"github.com/jingle2008/toolkit/internal/resolve"
	"github.com/jingle2008/toolkit/internal/ui/tui/actions"
	"github.com/jingle2008/toolkit/pkg/infra/logging"
	"github.com/jingle2008/toolkit/pkg/models"
)

// Seam variables — overrideable in tests so handlers don't reach a
// live cluster or OCI tenancy. Production callers go through the
// upstream packages directly. The resolver seams (mcpResolveGpuNodeFn,
// mcpResolveGpuPoolFn) cover the lookup-and-enrich chain so handler
// tests don't have to stub a fake k8s+OCI pipeline.
var (
	mcpSetCordonFn        = k8s.SetCordon
	mcpDrainNodeFn        = k8s.DrainNode
	mcpSoftResetFn        = actions.SoftResetInstance
	mcpTerminateFn        = actions.TerminateInstance
	mcpIncreasePoolSizeFn = actions.IncreasePoolSize
	mcpDeleteDACFn        = actions.DeleteDedicatedAICluster
	mcpResolveGpuNodeFn   = func(ctx context.Context, s *Server, env models.Environment, name, ocid string) (*models.GpuNode, error) {
		return resolve.GpuNode(ctx, s.loader, s.cfg.KubeConfig, env, name, ocid)
	}
	mcpResolveGpuPoolFn = func(ctx context.Context, s *Server, env models.Environment, name string) (*models.GpuPool, error) {
		return resolve.GpuPool(ctx, s.loader, s.cfg.RepoPath, s.cfg.KubeConfig, env, name)
	}
)

// confirmGate is embedded in every mutating tool's input. The field
// is OPTIONAL at the JSON-Schema level (omitempty) so the SDK passes
// the call to the handler even when confirm is missing — that way we
// can audit-log the attempt and emit a notifications/message
// explaining the contract. Only confirm=true triggers execution.
type confirmGate struct {
	Confirm bool `json:"confirm,omitempty" jsonschema:"set true to execute; otherwise the tool refuses without acting"`
}

// effectiveMutationEnv applies the agent's envOverride only when the
// operator opted into per-call overrides via
// MutationEnvOverrideAllowed. Otherwise the override is ignored and
// the startup env is used unchanged. When the override IS applied and
// changes the effective env, that's audit-logged for SIEM visibility.
func (s *Server) effectiveMutationEnv(_ context.Context, action, kind, target string, in envOverride) models.Environment {
	startup := s.envFor(envOverride{})
	if !s.cfg.MutationEnvOverrideAllowed {
		if in.EnvType != "" || in.EnvRegion != "" || in.EnvRealm != "" {
			s.logger.Infow("mutation env_override ignored (server disallows)",
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
		s.logger.Infow("mutation env override active (deviation from startup)",
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
// confirm is false, audit-log begin/refused/failed/done, emit a
// notifications/message at the right level, and return the standard
// envelope on success.
//
// Mirrors cli.runMutation but adapted to the MCP response shape —
// no stdout/prompt; success becomes a structured jsonResult.
func (s *Server) runMutationTool(ctx context.Context, req *sdk.CallToolRequest, action, kind, target string, confirm bool, perform func() error) (*sdk.CallToolResult, struct{}, error) {
	if !confirm {
		s.logger.Infow("mutation",
			"action", action, "kind", kind, "target", target, "surface", "mcp",
			"phase", "refused",
		)
		notify(ctx, req.Session, "info",
			fmt.Sprintf("%s %s/%s refused: set confirm=true to execute", action, kind, target))
		return failTool(ctx, req, action,
			fmt.Errorf("mutating tool requires confirm=true (target %s/%s)", kind, target))
	}

	s.logger.Infow("mutation",
		"action", action, "kind", kind, "target", target, "surface", "mcp",
		"phase", "begin",
	)
	if err := perform(); err != nil {
		s.logger.Errorw("mutation",
			"action", action, "kind", kind, "target", target, "surface", "mcp",
			"phase", "failed",
			"error", err,
		)
		return failTool(ctx, req, action+" "+kind+"/"+target, err)
	}
	s.logger.Infow("mutation",
		"action", action, "kind", kind, "target", target, "surface", "mcp",
		"phase", "done",
	)
	notify(ctx, req.Session, "info",
		fmt.Sprintf("%s %s/%s: OK", action, kind, target))
	return jsonResult(map[string]string{
		"status": "OK",
		"action": action,
		"kind":   kind,
		"target": target,
	}, nil)
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

type rebootNodeInput struct {
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

type scaleGpuPoolInput struct {
	Name string `json:"name" jsonschema:"the pool name from the Terraform repo (same as toolkit get gpupool)"`
	confirmGate
	envOverride
}

type deleteDACInput struct {
	Name string `json:"name" jsonschema:"the DAC name (same identifier as toolkit get dac shows)"`
	confirmGate
	envOverride
}

// --- Handlers -----------------------------------------------------

// handleMutation is the shared entry point for every mutating tool:
// derive the effective env (audit-logging any override), then dispatch
// through runMutationTool which enforces the confirm gate and emits the
// standard audit-log / notification / response envelope. Handlers
// supply only the action/kind/target labels plus the env-scoped perform
// closure.
func (s *Server) handleMutation(
	ctx context.Context,
	req *sdk.CallToolRequest,
	action, kind, target string,
	confirm bool,
	override envOverride,
	perform func(env models.Environment) error,
) (*sdk.CallToolResult, struct{}, error) {
	env := s.effectiveMutationEnv(ctx, action, kind, target, override)
	return s.runMutationTool(ctx, req, action, kind, target, confirm, func() error {
		return perform(env)
	})
}

func (s *Server) handleCordonNode(ctx context.Context, req *sdk.CallToolRequest, in cordonNodeInput) (*sdk.CallToolResult, struct{}, error) {
	return s.handleMutation(ctx, req, "cordon", "node", in.Node, in.Confirm, in.envOverride, func(env models.Environment) error {
		_, err := mcpSetCordonFn(ctx, s.cfg.KubeConfig, env.GetKubeContext(), in.Node, true)
		return err
	})
}

func (s *Server) handleUncordonNode(ctx context.Context, req *sdk.CallToolRequest, in cordonNodeInput) (*sdk.CallToolResult, struct{}, error) {
	return s.handleMutation(ctx, req, "uncordon", "node", in.Node, in.Confirm, in.envOverride, func(env models.Environment) error {
		_, err := mcpSetCordonFn(ctx, s.cfg.KubeConfig, env.GetKubeContext(), in.Node, false)
		return err
	})
}

func (s *Server) handleDrainNode(ctx context.Context, req *sdk.CallToolRequest, in drainNodeInput) (*sdk.CallToolResult, struct{}, error) {
	return s.handleMutation(ctx, req, "drain", "node", in.Node, in.Confirm, in.envOverride, func(env models.Environment) error {
		return mcpDrainNodeFn(ctx, s.cfg.KubeConfig, env.GetKubeContext(), in.Node)
	})
}

func (s *Server) handleRebootNode(ctx context.Context, req *sdk.CallToolRequest, in rebootNodeInput) (*sdk.CallToolResult, struct{}, error) {
	return s.handleMutation(ctx, req, "reboot", "node", in.Node, in.Confirm, in.envOverride, func(env models.Environment) error {
		node, err := mcpResolveGpuNodeFn(ctx, s, env, in.Node, in.OCID)
		if err != nil {
			return err
		}
		return mcpSoftResetFn(ctx, node, env, logging.FromContext(ctx))
	})
}

func (s *Server) handleTerminateNode(ctx context.Context, req *sdk.CallToolRequest, in terminateNodeInput) (*sdk.CallToolResult, struct{}, error) {
	return s.handleMutation(ctx, req, "terminate", "node", in.Node, in.Confirm, in.envOverride, func(env models.Environment) error {
		node, err := mcpResolveGpuNodeFn(ctx, s, env, in.Node, in.OCID)
		if err != nil {
			return err
		}
		return mcpTerminateFn(ctx, node, env, logging.FromContext(ctx))
	})
}

func (s *Server) handleScaleGpuPool(ctx context.Context, req *sdk.CallToolRequest, in scaleGpuPoolInput) (*sdk.CallToolResult, struct{}, error) {
	return s.handleMutation(ctx, req, "scale", "gpu_pool", in.Name, in.Confirm, in.envOverride, func(env models.Environment) error {
		pool, err := mcpResolveGpuPoolFn(ctx, s, env, in.Name)
		if err != nil {
			return err
		}
		return mcpIncreasePoolSizeFn(ctx, pool, env, logging.FromContext(ctx))
	})
}

func (s *Server) handleDeleteDAC(ctx context.Context, req *sdk.CallToolRequest, in deleteDACInput) (*sdk.CallToolResult, struct{}, error) {
	return s.handleMutation(ctx, req, "delete", "dac", in.Name, in.Confirm, in.envOverride, func(env models.Environment) error {
		dac := &models.DedicatedAICluster{Name: in.Name}
		return mcpDeleteDACFn(ctx, dac, env, logging.FromContext(ctx))
	})
}

// --- Registration -------------------------------------------------

// mutationToolFooter is appended to every mutation tool's description.
// Centralized so the env-override semantics stay in sync across all
// seven tools whenever the policy is revisited.
const mutationToolFooter = " Mutating: requires confirm=true to execute, otherwise refuses without acting." +
	" Accepts env_type/env_region/env_realm, but per-call overrides take effect ONLY if the server" +
	" was started with --mutation_env_override_allowed. By default override fields are ignored" +
	" (audit-logged at info) and the startup env is used."

// registerMutationTools adds the seven mutating tools. Each requires
// confirm=true at the input level; the tool description tells the
// agent explicitly so the contract is discoverable without running
// the tool to see the refusal.
func registerMutationTools(s *Server) {
	// Every mutation tool's input schema includes env_type/env_region/
	// env_realm for parity with the read-only list_* tools, but the
	// override only takes effect when the operator started this server
	// with --mutation_env_override_allowed. By default the agent's
	// override fields are ignored (and audit-logged at info), so the
	// safety story remains: the operator's startup-env choice caps
	// blast radius, and confirm=true gates each individual mutation.
	// Operators who want to give the agent multi-realm authority can
	// flip the flag — the risk is documented on the flag itself.
	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "cordon_node",
		Description: "Cordon (mark unschedulable) a Kubernetes node. Idempotent." + mutationToolFooter,
	}, s.handleCordonNode)

	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "uncordon_node",
		Description: "Uncordon (mark schedulable) a Kubernetes node. Idempotent." + mutationToolFooter,
	}, s.handleUncordonNode)

	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "drain_node",
		Description: "Drain pods from a node (cordon + evict). Use before terminate." + mutationToolFooter,
	}, s.handleDrainNode)

	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "reboot_node",
		Description: "Soft-reset the OCI instance backing a GPU node. Fire-and-forget." + mutationToolFooter,
	}, s.handleRebootNode)

	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "terminate_node",
		Description: "Terminate the OCI instance backing a GPU node (boot volume destroyed). DESTRUCTIVE." + mutationToolFooter,
	}, s.handleTerminateNode)

	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "scale_gpu_pool",
		Description: "Push the Terraform-declared pool.Size to OCI for the named GPU pool. No size override: Terraform is the source of truth." + mutationToolFooter,
	}, s.handleScaleGpuPool)

	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "delete_dac",
		Description: "Delete a dedicated AI cluster and its endpoints (synchronous, polls the work request). DESTRUCTIVE." + mutationToolFooter,
	}, s.handleDeleteDAC)
}
