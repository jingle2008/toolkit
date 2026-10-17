package mcp

import (
	"context"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/relations"
	"github.com/jingle2008/toolkit/internal/resolve"
	"github.com/jingle2008/toolkit/pkg/models"
)

// Seams tests use to fake the full load faulty rows and integrity checks
// need.
var (
	mcpFaultyDatasetFn    = resolve.FaultyDataset
	mcpIntegrityDatasetFn = resolve.IntegrityDataset
)

// faultyListInput is listInput for the tools whose rows can be faulty
// (relations.FaultyCategories); only they take `faulty`.
type faultyListInput struct {
	listInput
	Faulty bool `json:"faulty,omitempty" jsonschema:"only faulty rows, as the TUI's Ctrl+Z shows them: rows in a bad state, or naming a parent row that doesn't exist (what check_integrity reports as FAIL). Loads every category, so it is slower."`
}

// faultyFlat answers a flat list tool's faulty=true: cat's faulty rows,
// then the usual filter and limit.
func faultyFlat[T models.NamedFilterable](
	ctx context.Context, s *Server, cat domain.Category, in faultyListInput, pick func(*models.Dataset) []T,
) (*sdk.CallToolResult, listResult[T], error) {
	ds, warnings, err := mcpFaultyDatasetFn(ctx, s.loader, s.cfg.RepoPath, s.cfg.KubeConfig, s.envFor(in.envOverride), cat)
	if err != nil {
		return failTool[listResult[T]]("list faulty "+strings.ToLower(cat.String())+" rows", err)
	}
	return listFlatResult(relations.FaultySlice(ds, cat, pick(ds)), in.Filter, in.Limit, warnings)
}

// faultyGrouped is faultyFlat for a grouped category.
func faultyGrouped[T models.NamedFilterable](
	ctx context.Context, s *Server, cat domain.Category, in faultyListInput, pick func(*models.Dataset) map[string][]T,
) (*sdk.CallToolResult, listResult[T], error) {
	ds, warnings, err := mcpFaultyDatasetFn(ctx, s.loader, s.cfg.RepoPath, s.cfg.KubeConfig, s.envFor(in.envOverride), cat)
	if err != nil {
		return failTool[listResult[T]]("list faulty "+strings.ToLower(cat.String())+" rows", err)
	}
	return jsonResult(flattenGrouped(relations.FaultyMap(ds, cat, pick(ds)), in.Filter, in.Limit), warnings)
}

type integrityInput struct {
	envOverride
}

// integrityCheck is one check_integrity row: a relation, or the tenancy
// rule, as `toolkit doctor --relations` reports it.
type integrityCheck struct {
	Name     string               `json:"name"`
	Child    string               `json:"child,omitempty"`
	Parent   string               `json:"parent,omitempty"`
	Status   string               `json:"status" jsonschema:"PASS, or the check's severity when a row dangles: FAIL (a missing parent row) or WARN (a tenancy missing from the tenant metadata)"`
	Checked  int                  `json:"checked"`
	Dangling []relations.Dangling `json:"dangling,omitempty"`
	Note     string               `json:"note,omitempty"`
}

func (s *Server) handleCheckIntegrity(ctx context.Context, _ *sdk.CallToolRequest, in integrityInput) (*sdk.CallToolResult, listResult[integrityCheck], error) {
	ds, poolNote, err := mcpIntegrityDatasetFn(ctx, s.loader, s.cfg.RepoPath, s.cfg.KubeConfig, s.envFor(in.envOverride))
	if err != nil {
		return failTool[listResult[integrityCheck]]("check integrity", err)
	}
	var warnings []string
	if poolNote != "" {
		warnings = append(warnings, poolNote)
	}
	checks := relations.CheckIntegrity(ds)
	out := make([]integrityCheck, 0, len(checks))
	for _, c := range checks {
		r := integrityCheck{
			Name: c.Name, Parent: c.Parent.String(), Status: "PASS",
			Checked: c.Checked, Dangling: c.Dangling, Note: c.Note,
		}
		if c.Child != domain.CategoryUnknown { // the tenancy rule spans categories
			r.Child = c.Child.String()
		}
		if len(c.Dangling) > 0 {
			r.Status = string(c.Severity)
		}
		out = append(out, r)
	}
	return jsonResult(out, warnings)
}

const checkIntegrityDescription = "Check every relation between rows, as `toolkit doctor --relations` does: one item per relation" +
	" (e.g. InferenceService → DedicatedAICluster) with how many rows were checked and every row naming a parent that doesn't exist" +
	" (`dangling`, each {row, missing}). `status` is PASS, FAIL (missing parent rows: orphans, such as an endpoint whose DAC was deleted," +
	" which delete_inference_service removes) or WARN (a tenancy missing from the tenant metadata; fix with set_tenant)." +
	" Loads every category. Accepts env_type/env_region/env_realm overrides."
