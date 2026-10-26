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
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jingle2008/toolkit/internal/config"
	"github.com/jingle2008/toolkit/internal/infra/loader"
	"github.com/jingle2008/toolkit/internal/infra/terraform"
	"github.com/jingle2008/toolkit/internal/logging"
	"github.com/jingle2008/toolkit/internal/mutation"
	"github.com/jingle2008/toolkit/pkg/models"
)

// Server is the toolkit's MCP server. Build with NewServer, then call Run.
type Server struct {
	cfg    config.Config
	loader loader.Composite
	logger *slog.Logger
	server *sdk.Server
	// bgCtx scopes work that outlives a tool call (drain drivers) to the
	// server's lifetime; Run replaces it with its own ctx.
	bgCtx context.Context //nolint:containedctx // lifetime of background drain drivers, which outlive tool calls
	// drainDrivers holds the "kubeContext/node" keys of drains this
	// process is driving, so a repeated drain_node adds no second driver.
	drainDrivers sync.Map
	// envCache is the repo's environments, which envGuard checks env
	// overrides against.
	envCache envCache
}

// NewServer constructs a server that exposes read-only category tools.
// cfg supplies the startup env defaults; each tool call may override
// env_type / env_region / env_realm per-call.
func NewServer(cfg config.Config, ld loader.Composite, logger *slog.Logger, version string) *Server {
	s := &Server{
		cfg:    cfg,
		loader: ld,
		logger: logger,
		bgCtx:  logging.WithContext(context.Background(), logger),
	}
	s.server = sdk.NewServer(&sdk.Implementation{
		Name:    "toolkit",
		Version: version,
	}, nil)
	s.server.AddReceivingMiddleware(s.envGuard)
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
// A line that isn't a JSON-RPC message is answered with a JSON-RPC error
// and the session goes on (see newGuardedTransport).
//
// Single-shot: a second Run on the same *Server reuses the SDK's
// session list rather than reinitializing cleanly. Construct a fresh
// Server (via NewServer) if you need to restart.
func (s *Server) Run(ctx context.Context) error {
	return s.runIO(ctx, os.Stdin, os.Stdout)
}

// runIO is Run over in and out instead of stdin and stdout.
func (s *Server) runIO(ctx context.Context, in io.Reader, out io.Writer) error {
	s.bgCtx = ctx
	return s.server.Run(ctx, newGuardedTransport(in, out))
}

// envOverride is the shared input shape for tools that touch realm /
// region scoped data. All three fields are optional; empty means
// "use the value supplied at server startup".
type envOverride struct {
	EnvType   string `json:"env_type,omitempty" jsonschema:"override startup env_type (dev/preprod/prod/...); ppe is accepted as an alias for preprod"`
	EnvRegion string `json:"env_region,omitempty" jsonschema:"override startup env_region: either a full identifier (us-ashburn-1) or a 3-letter code (iad)"`
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
		env.Type = models.ResolveEnvType(in.EnvType)
	}
	if in.EnvRegion != "" {
		env.Region = in.EnvRegion
		// envFor has no error return (25 call sites), so an unresolvable
		// code is passed through here; envGuard has already refused the
		// call before any handler gets this far (see checkEnvOverride).
		if r, err := models.ResolveRegion(in.EnvRegion); err != nil {
			s.logger.Warn("unresolvable env_region override; passing through",
				"env_region", in.EnvRegion, "error", err)
		} else {
			env.Region = string(r)
		}
	}
	if in.EnvRealm != "" {
		env.Realm = models.ResolveRealm(in.EnvRealm)
	}
	return env
}

// listInput is the common input for category list tools.
type listInput struct {
	Filter string `json:"filter,omitempty" jsonschema:"fuzzy substring match across the model's filterable fields (case-insensitive)"`
	Limit  int    `json:"limit,omitempty"  jsonschema:"max items to return after filter; 0 (default) means unlimited. The cap is on the whole result (where rows come in groups, across all of them)."`
	envOverride
}

// kindInput extends listInput with a "kind" discriminator used by the
// bundled definition / override tools.
type kindInput struct {
	Kind string `json:"kind" jsonschema:"one of: limit, console_property, property (case-insensitive)"`
	listInput
}

// kind is Kind lowercased and trimmed, as category names are read, so
// "Limit" or " limit" select the limit kind.
func (k kindInput) kind() string { return strings.ToLower(strings.TrimSpace(k.Kind)) }

// listResult is the uniform envelope returned by every list tool. It's
// generic over T so the SDK can derive a proper OutputSchema from the
// item shape via reflection (replacing the previous json.RawMessage
// items field, which was opaque to the schema generator and shipped as
// empty `{}` to clients that read StructuredContent).
//
//	items     — the matching rows (array of category-shaped objects)
//	count     — len(items), provided for quick parsing
//	warnings  — non-fatal loader warnings (e.g. partial GPUPool sources)
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
	// Resolved is what a dry run looked up about the target (e.g. the
	// node's instance, the InferenceService's namespace); dry runs only.
	Resolved mutation.Resolved `json:"resolved,omitempty"`
	// Note is what the call wants the agent to know: why a NOOP did
	// nothing, or a dry run's warning.
	Note string `json:"note,omitempty"`
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

// mutationOK returns the standard "OK" envelope every mutation emits on
// the happy path. Mirrors jsonResult's role but for the mutationResult
// shape.
func mutationOK(action, kind, target string) mutationResult {
	return mutationResult{
		Status: "OK",
		Action: action,
		Kind:   kind,
		Target: target,
	}
}

// failTool wraps a handler's fatal error path. `what` is the human
// label (e.g. "load gpu pools"); err is the underlying cause. The
// wrapped error is what the SDK turns into CallToolResult.IsError plus
// a message, which is the client's channel for the failure.
//
// This used to also emit a notifications/message at "error" level. That
// was redundant with the returned error and rode on the MCP logging
// feature deprecated by SEP-2577, so it was dropped.
//
// Generic over Out so the same helper covers list and mutation handlers
// without each caller having to spell out a typed zero. Callers supply
// the Out type at the call site, e.g. `failTool[listResult[Tenant]](...)`.
//
//nolint:unparam // signature pinned by ToolHandlerFor[In, Out] — *CallToolResult is always nil on the failure path but must be in the tuple
func failTool[Out any](what string, err error) (*sdk.CallToolResult, Out, error) {
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
