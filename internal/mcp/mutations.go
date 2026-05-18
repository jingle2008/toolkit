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
// upstream packages directly.
var (
	mcpSetCordonFn        = k8s.SetCordon
	mcpDrainNodeFn        = k8s.DrainNode
	mcpSoftResetFn        = actions.SoftResetInstance
	mcpTerminateFn        = actions.TerminateInstance
	mcpIncreasePoolSizeFn = actions.IncreasePoolSize
	mcpDeleteDACFn        = actions.DeleteDedicatedAICluster
)

// confirmGate is embedded in every mutating tool's input. The field
// is OPTIONAL at the JSON-Schema level (omitempty) so the SDK passes
// the call to the handler even when confirm is missing — that way we
// can audit-log the attempt and emit a notifications/message
// explaining the contract. Only confirm=true triggers execution.
type confirmGate struct {
	Confirm bool `json:"confirm,omitempty" jsonschema:"set true to execute; otherwise the tool refuses without acting"`
}

// requireConfirm short-circuits with a uniform tool error when the
// caller forgot to set confirm: true. Audit log captures the intent
// so the refusal is visible alongside successful mutations.
//
// Returns (response, _, err, gated). When gated is true the caller
// must return the first three values immediately.
func (s *Server) requireConfirm(ctx context.Context, req *sdk.CallToolRequest, action, kind, target string, confirm bool) (*sdk.CallToolResult, struct{}, error, bool) {
	if confirm {
		return nil, struct{}{}, nil, false
	}
	s.logger.Infow("mutation",
		"action", action, "kind", kind, "target", target, "surface", "mcp",
		"phase", "refused",
	)
	notify(ctx, req.Session, "info",
		fmt.Sprintf("%s %s/%s refused: set confirm=true to execute", action, kind, target))
	res, _, err := failTool(ctx, req, action,
		fmt.Errorf("mutating tool requires confirm=true (target %s/%s)", kind, target))
	return res, struct{}{}, err, true
}

// runMutationTool wraps the audit+notify+execute flow that every
// mutating handler shares. Mirrors cli.runMutation but adapted to the
// MCP response shape — no stdout/prompt; success becomes a structured
// jsonResult.
func (s *Server) runMutationTool(ctx context.Context, req *sdk.CallToolRequest, action, kind, target string, perform func() error) (*sdk.CallToolResult, struct{}, error) {
	s.logger.Infow("mutation",
		"action", action, "kind", kind, "target", target, "surface", "mcp",
		"phase", "begin",
	)
	if err := perform(); err != nil {
		s.logger.Errorw("mutation failed",
			"action", action, "kind", kind, "target", target, "surface", "mcp",
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
}

type drainNodeInput struct {
	Node string `json:"node" jsonschema:"the node name as reported by kubectl get nodes"`
	confirmGate
}

type rebootNodeInput struct {
	Node string `json:"node" jsonschema:"the node name as reported by kubectl get nodes"`
	OCID string `json:"ocid,omitempty" jsonschema:"skip k8s lookup and target this instance OCID directly"`
	confirmGate
}

type terminateNodeInput struct {
	Node string `json:"node" jsonschema:"the node name as reported by kubectl get nodes"`
	OCID string `json:"ocid,omitempty" jsonschema:"skip k8s lookup and target this instance OCID directly"`
	confirmGate
}

type scaleGpuPoolInput struct {
	Name string `json:"name" jsonschema:"the pool name from the Terraform repo (same as toolkit get gpupool)"`
	confirmGate
}

type deleteDACInput struct {
	Name string `json:"name" jsonschema:"the DAC name (same identifier as toolkit get dac shows)"`
	confirmGate
}

// --- Handlers -----------------------------------------------------

func (s *Server) handleCordonNode(ctx context.Context, req *sdk.CallToolRequest, in cordonNodeInput) (*sdk.CallToolResult, struct{}, error) {
	if res, _, err, gated := s.requireConfirm(ctx, req, "cordon", "node", in.Node, in.Confirm); gated {
		return res, struct{}{}, err
	}
	env := s.envFor(envOverride{})
	return s.runMutationTool(ctx, req, "cordon", "node", in.Node, func() error {
		_, err := mcpSetCordonFn(ctx, s.cfg.KubeConfig, env.GetKubeContext(), in.Node, true)
		return err
	})
}

func (s *Server) handleUncordonNode(ctx context.Context, req *sdk.CallToolRequest, in cordonNodeInput) (*sdk.CallToolResult, struct{}, error) {
	if res, _, err, gated := s.requireConfirm(ctx, req, "uncordon", "node", in.Node, in.Confirm); gated {
		return res, struct{}{}, err
	}
	env := s.envFor(envOverride{})
	return s.runMutationTool(ctx, req, "uncordon", "node", in.Node, func() error {
		_, err := mcpSetCordonFn(ctx, s.cfg.KubeConfig, env.GetKubeContext(), in.Node, false)
		return err
	})
}

func (s *Server) handleDrainNode(ctx context.Context, req *sdk.CallToolRequest, in drainNodeInput) (*sdk.CallToolResult, struct{}, error) {
	if res, _, err, gated := s.requireConfirm(ctx, req, "drain", "node", in.Node, in.Confirm); gated {
		return res, struct{}{}, err
	}
	env := s.envFor(envOverride{})
	return s.runMutationTool(ctx, req, "drain", "node", in.Node, func() error {
		return mcpDrainNodeFn(ctx, s.cfg.KubeConfig, env.GetKubeContext(), in.Node)
	})
}

func (s *Server) handleRebootNode(ctx context.Context, req *sdk.CallToolRequest, in rebootNodeInput) (*sdk.CallToolResult, struct{}, error) {
	if res, _, err, gated := s.requireConfirm(ctx, req, "reboot", "node", in.Node, in.Confirm); gated {
		return res, struct{}{}, err
	}
	env := s.envFor(envOverride{})
	return s.runMutationTool(ctx, req, "reboot", "node", in.Node, func() error {
		node, err := s.resolveNodeForOCIAction(ctx, env, in.Node, in.OCID)
		if err != nil {
			return err
		}
		return mcpSoftResetFn(ctx, node, env, logging.FromContext(ctx))
	})
}

func (s *Server) handleTerminateNode(ctx context.Context, req *sdk.CallToolRequest, in terminateNodeInput) (*sdk.CallToolResult, struct{}, error) {
	if res, _, err, gated := s.requireConfirm(ctx, req, "terminate", "node", in.Node, in.Confirm); gated {
		return res, struct{}{}, err
	}
	env := s.envFor(envOverride{})
	return s.runMutationTool(ctx, req, "terminate", "node", in.Node, func() error {
		node, err := s.resolveNodeForOCIAction(ctx, env, in.Node, in.OCID)
		if err != nil {
			return err
		}
		return mcpTerminateFn(ctx, node, env, logging.FromContext(ctx))
	})
}

func (s *Server) handleScaleGpuPool(ctx context.Context, req *sdk.CallToolRequest, in scaleGpuPoolInput) (*sdk.CallToolResult, struct{}, error) {
	if res, _, err, gated := s.requireConfirm(ctx, req, "scale", "gpu_pool", in.Name, in.Confirm); gated {
		return res, struct{}{}, err
	}
	env := s.envFor(envOverride{})
	return s.runMutationTool(ctx, req, "scale", "gpu_pool", in.Name, func() error {
		pool, err := s.resolveGpuPoolForOCIAction(ctx, env, in.Name)
		if err != nil {
			return err
		}
		return mcpIncreasePoolSizeFn(ctx, pool, env, logging.FromContext(ctx))
	})
}

func (s *Server) handleDeleteDAC(ctx context.Context, req *sdk.CallToolRequest, in deleteDACInput) (*sdk.CallToolResult, struct{}, error) {
	if res, _, err, gated := s.requireConfirm(ctx, req, "delete", "dac", in.Name, in.Confirm); gated {
		return res, struct{}{}, err
	}
	env := s.envFor(envOverride{})
	return s.runMutationTool(ctx, req, "delete", "dac", in.Name, func() error {
		dac := &models.DedicatedAICluster{Name: in.Name}
		return mcpDeleteDACFn(ctx, dac, env, logging.FromContext(ctx))
	})
}

// --- Resolvers ----------------------------------------------------
//
// Thin delegations to internal/resolve. The shared package centralizes
// the find-by-name + OCI-enrichment chain so CLI and MCP agree on
// partial-load tolerance, "pool has no OCID yet" guards, and the
// compartment-ID fallback.

func (s *Server) resolveNodeForOCIAction(ctx context.Context, env models.Environment, name, ocid string) (*models.GpuNode, error) {
	return resolve.GpuNode(ctx, s.loader, s.cfg.KubeConfig, env, name, ocid)
}

func (s *Server) resolveGpuPoolForOCIAction(ctx context.Context, env models.Environment, name string) (*models.GpuPool, error) {
	return resolve.GpuPool(ctx, s.loader, s.cfg.RepoPath, s.cfg.KubeConfig, env, name)
}

// --- Registration -------------------------------------------------

// registerMutationTools adds the seven mutating tools. Each requires
// confirm=true at the input level; the tool description tells the
// agent explicitly so the contract is discoverable without running
// the tool to see the refusal.
func registerMutationTools(s *Server) {
	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "cordon_node",
		Description: "Cordon (mark unschedulable) a Kubernetes node. Idempotent. Mutating: requires confirm=true to execute, otherwise refuses without acting.",
	}, s.handleCordonNode)

	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "uncordon_node",
		Description: "Uncordon (mark schedulable) a Kubernetes node. Idempotent. Mutating: requires confirm=true.",
	}, s.handleUncordonNode)

	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "drain_node",
		Description: "Drain pods from a node (cordon + evict). Use before terminate. Mutating: requires confirm=true.",
	}, s.handleDrainNode)

	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "reboot_node",
		Description: "Soft-reset the OCI instance backing a GPU node. Fire-and-forget. Mutating: requires confirm=true.",
	}, s.handleRebootNode)

	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "terminate_node",
		Description: "Terminate the OCI instance backing a GPU node (boot volume destroyed). DESTRUCTIVE. Mutating: requires confirm=true.",
	}, s.handleTerminateNode)

	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "scale_gpu_pool",
		Description: "Push the Terraform-declared pool.Size to OCI for the named GPU pool. No size override: Terraform is the source of truth. Mutating: requires confirm=true.",
	}, s.handleScaleGpuPool)

	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "delete_dac",
		Description: "Delete a dedicated AI cluster and its endpoints (synchronous, polls the work request). DESTRUCTIVE. Mutating: requires confirm=true.",
	}, s.handleDeleteDAC)
}
