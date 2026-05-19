/*
Package mcp implements the toolkit MCP server. It exposes the same
read-only category surface as the headless `toolkit get` CLI as a set
of MCP tools that an agent (Claude Code, Claude Desktop, any
MCP-aware client) can call directly over stdio — no shell out, no
output parsing.

The handlers reuse the existing loader composite (internal/infra/loader),
so any improvement to data loading (partial-tolerance, variable
defaults, etc.) is shared between CLI and MCP automatically.
*/
package mcp

import (
	"context"
	"errors"
	"fmt"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jingle2008/toolkit/internal/config"
	"github.com/jingle2008/toolkit/internal/infra/loader"
	"github.com/jingle2008/toolkit/internal/infra/terraform"
	"github.com/jingle2008/toolkit/pkg/infra/logging"
	"github.com/jingle2008/toolkit/pkg/models"
)

// Server is the toolkit's MCP server. Build with NewServer, then call Run.
type Server struct {
	cfg    config.Config
	loader loader.Loader
	logger logging.Logger
	server *sdk.Server
}

// NewServer constructs a server that exposes read-only category tools.
// cfg supplies the startup env defaults; each tool call may override
// env_type / env_region / env_realm per-call.
func NewServer(cfg config.Config, ld loader.Loader, logger logging.Logger, version string) *Server {
	s := &Server{
		cfg:    cfg,
		loader: ld,
		logger: logger,
	}
	s.server = sdk.NewServer(&sdk.Implementation{
		Name:    "toolkit",
		Version: version,
	}, nil)
	registerTools(s)
	return s
}

// Run blocks until any of:
//   - stdin reaches EOF (the MCP client closed the pipe),
//   - ctx is canceled,
//   - the underlying transport returns a fatal error.
//
// Returns nil on a clean client disconnect or ctx cancel, otherwise
// the transport's error. Uses stdio: stdin reads JSON-RPC frames,
// stdout writes them. Callers must keep stdout free of any other
// output (the toolkit logger writes to a file by default — see
// internal/cli/mcp.go).
//
// Single-shot: a second Run on the same *Server reuses the SDK's
// session list rather than reinitializing cleanly. Construct a fresh
// Server (via NewServer) if you need to restart.
func (s *Server) Run(ctx context.Context) error {
	return s.server.Run(ctx, &sdk.StdioTransport{})
}

// envOverride is the shared input shape for tools that touch realm /
// region scoped data. All three fields are optional; empty means
// "use the value supplied at server startup".
type envOverride struct {
	EnvType   string `json:"env_type,omitempty" jsonschema:"override startup env_type (dev/preprod/prod/...)"`
	EnvRegion string `json:"env_region,omitempty" jsonschema:"override startup env_region (e.g. us-ashburn-1)"`
	EnvRealm  string `json:"env_realm,omitempty" jsonschema:"override startup env_realm (e.g. oc1)"`
}

// envFor returns the effective Environment for this call by layering
// any non-empty override fields on top of the startup config.
func (s *Server) envFor(in envOverride) models.Environment {
	env := models.Environment{
		Type:   s.cfg.EnvType,
		Region: s.cfg.EnvRegion,
		Realm:  s.cfg.EnvRealm,
	}
	if in.EnvType != "" {
		env.Type = in.EnvType
	}
	if in.EnvRegion != "" {
		env.Region = in.EnvRegion
	}
	if in.EnvRealm != "" {
		env.Realm = in.EnvRealm
	}
	return env
}

// listInput is the common input for category list tools.
type listInput struct {
	Filter string `json:"filter,omitempty" jsonschema:"fuzzy substring match across the model's filterable fields (case-insensitive)"`
	Limit  int    `json:"limit,omitempty"  jsonschema:"max items to return after filter; 0 (default) means unlimited. For grouped categories the cap is across the whole flattened result, not per group."`
	envOverride
}

// kindInput extends listInput with a "kind" discriminator used by the
// bundled definition / override tools.
type kindInput struct {
	Kind string `json:"kind" jsonschema:"one of: limit, console_property, property"`
	listInput
}

// listResult is the uniform envelope returned by every list tool. It's
// generic over T so the SDK can derive a proper OutputSchema from the
// item shape via reflection (replacing the previous json.RawMessage
// items field, which was opaque to the schema generator and shipped as
// empty `{}` to clients that read StructuredContent).
//
//	items     — the matching rows (array of category-shaped objects)
//	count     — len(items), provided for quick parsing
//	warnings  — non-fatal loader warnings (e.g. partial GpuPool sources)
type listResult[T any] struct {
	Items    []T      `json:"items"`
	Count    int      `json:"count"`
	Warnings []string `json:"warnings,omitempty"`
}

// mutationResult is the success envelope every mutating tool returns.
// Distinct from listResult so the OutputSchema reflects the actual
// payload shape (no `count: 0` / `items: []` noise).
type mutationResult struct {
	Status string `json:"status"`
	Action string `json:"action"`
	Kind   string `json:"kind"`
	Target string `json:"target"`
}

// jsonResult wraps items in the standard listResult envelope. Callers
// return the value directly as the typed Out of the SDK handler; the
// SDK marshals it into CallToolResult.StructuredContent and — when
// res.Content is empty — auto-emits a TextContent block carrying the
// same JSON for backward-compat with older clients (see
// go-sdk/mcp/server.go: "If the Content field isn't being used,
// return the serialized JSON in a TextContent block").
//
// We pass &sdk.CallToolResult{} so the SDK fills Content; we never
// hand-build the TextContent.
//
//nolint:unparam // signature pinned by ToolHandlerFor[In, Out] — error is always nil but must be in the tuple
func jsonResult[T any](items []T, warnings []string) (*sdk.CallToolResult, listResult[T], error) {
	if items == nil {
		items = []T{}
	}
	return &sdk.CallToolResult{}, listResult[T]{
		Items:    items,
		Count:    len(items),
		Warnings: warnings,
	}, nil
}

// mutationSuccess returns the standard "OK" envelope every mutation
// emits on the happy path. Mirrors jsonResult's role but for the
// mutationResult shape.
func mutationSuccess(action, kind, target string) (*sdk.CallToolResult, mutationResult, error) {
	return &sdk.CallToolResult{}, mutationResult{
		Status: "OK",
		Action: action,
		Kind:   kind,
		Target: target,
	}, nil
}

// notify emits a notifications/message to the connected MCP client.
// Best-effort: errors from Log are intentionally swallowed because a
// notification failure must not mask the tool's primary response (or
// error). A nil session — possible if a handler is invoked outside a
// live transport — silently no-ops.
func notify(ctx context.Context, sess *sdk.ServerSession, level sdk.LoggingLevel, msg string) {
	if sess == nil {
		return
	}
	_ = sess.Log(ctx, &sdk.LoggingMessageParams{
		Level:  level,
		Logger: "toolkit",
		Data:   msg,
	})
}

// failTool wraps a handler's fatal error path: it emits a
// notifications/message at "error" level so MCP clients can show the
// failure live (the tool error itself is also returned and surfaces as
// a tool-call failure in the response). `what` is the human label
// (e.g. "load gpu pools"); err is the underlying cause.
//
// Generic over Out so the same helper covers list and mutation handlers
// without each caller having to spell out a typed zero. Callers supply
// the Out type at the call site, e.g. `failTool[listResult[Tenant]](...)`.
//
//nolint:unparam // signature pinned by ToolHandlerFor[In, Out] — *CallToolResult is always nil on the failure path but must be in the tuple
func failTool[Out any](ctx context.Context, req *sdk.CallToolRequest, what string, err error) (*sdk.CallToolResult, Out, error) {
	notify(ctx, req.Session, "error", fmt.Sprintf("%s: %v", what, err))
	var zero Out
	return nil, zero, fmt.Errorf("%s: %w", what, err)
}

// warningsFromPartial pulls the per-source error strings off a
// *terraform.PartialLoadError so MCP callers can see what loaded
// partially without the err being treated as fatal.
func warningsFromPartial(err error) []string {
	partial, ok := errors.AsType[*terraform.PartialLoadError](err)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(partial.Errs))
	for _, e := range partial.Errs {
		out = append(out, e.Error())
	}
	return out
}

// normFilter applies the same fuzzy substring matching the CLI uses.
func normFilter(s string) string { return strings.ToLower(strings.TrimSpace(s)) }
