package mcp

import (
	"context"
	"errors"
	"fmt"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jingle2008/toolkit/internal/collections"
	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/infra/telemetry"
	"github.com/jingle2008/toolkit/internal/resolve"
)

// Seams tests use to fake the row and catalog load, and the clock.
var (
	mcpMetricsTargetFn = resolve.MetricsTarget
	mcpNow             = time.Now
)

type metricsInput struct {
	Category string `json:"category" jsonschema:"dac, importedmodel, gpuworkload or inferenceservice (names or aliases, see list_aliases)"`
	Name     string `json:"name" jsonschema:"the row's name, as get_item takes it: exact, or <group>/<name> when groups share it"`
	DAC      string `json:"dac,omitempty" jsonschema:"for an imported model several DACs serve: the DAC namespace whose dashboard to return"`
	envOverride
}

type metricsResult struct {
	URL        string   `json:"url"`
	Capability string   `json:"capability" jsonschema:"the metric set the served model's capability calls for, e.g. chat, text_rerank"`
	FilterKey  string   `json:"filterKey" jsonschema:"DacId (dedicated: one DAC) or ResourceId (on-demand: one model)"`
	FilterVal  string   `json:"filterValue"`
	Warnings   []string `json:"warnings,omitempty"`
}

const metricsDescription = "Get the OCI Telemetry MQL dashboard URL for one row — what `toolkit metrics <category> <name>` prints and" +
	" the TUI's metrics key opens: the last 7 days of the metrics the served model's capability calls for, scoped to the" +
	" DAC (dedicated) or the model (on-demand). For dedicated AI clusters, imported models, GPU workloads and inference" +
	" services. An imported model several DACs serve needs `dac`. A row with no dashboard (no model, an unsupported" +
	" capability) is an error saying why. Accepts env_type/env_region/env_realm overrides."

func (s *Server) handleMetricsURL(ctx context.Context, _ *sdk.CallToolRequest, in metricsInput) (*sdk.CallToolResult, metricsResult, error) {
	cat, err := domain.ParseCategory(in.Category)
	if err != nil {
		return failTool[metricsResult]("metrics_url", fmt.Errorf("category: %w", err))
	}
	env := s.envFor(in.envOverride)
	item, catalog, warnings, err := mcpMetricsTargetFn(ctx, s.loader, s.cfg.RepoPath, s.cfg.KubeConfig, env, cat, in.Name)
	if _, ok := errors.AsType[*collections.NoSuchNameError](err); ok {
		err = fmt.Errorf("%w (the name must match exactly; list tools' filter matches loosely)", err)
	}
	if err != nil {
		return failTool[metricsResult]("metrics_url", err)
	}
	plan, err := telemetry.PlanOn(item, env, catalog, in.DAC)
	if _, ok := errors.AsType[*telemetry.SeveralDACsError](err); ok {
		err = fmt.Errorf("%w; pass dac", err)
	}
	if errors.Is(err, telemetry.ErrNoMetrics) {
		err = fmt.Errorf("no metrics dashboard for %s %s", cat, in.Name)
	}
	if err != nil {
		return failTool[metricsResult]("metrics_url", err)
	}
	return &sdk.CallToolResult{}, metricsResult{
		URL:        telemetry.DashboardURL(env, plan, mcpNow()),
		Capability: plan.Capability.String(),
		FilterKey:  plan.Filter.Key,
		FilterVal:  plan.Filter.Value,
		Warnings:   warnings,
	}, nil
}
