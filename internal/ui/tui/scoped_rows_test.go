package tui

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/pkg/models"
)

// TestComputeTableRows_RegionalOverrideScopeFilter guards the context
// (scope) filter for the flat regional-override categories: when scoped to
// a definition, only the overrides for that definition are shown.
func TestComputeTableRows_RegionalOverrideScopeFilter(t *testing.T) {
	t.Parallel()
	ds := &models.Dataset{
		LimitRegionalOverrides: []models.LimitRegionalOverride{
			{Name: "limit-a", Regions: []string{"r1"}},
			{Name: "limit-b", Regions: []string{"r2"}},
		},
	}

	// Scoped to limit-a's definition: only limit-a's override is shown.
	scope := &domain.Scope{Category: domain.LimitDefinition, Name: "limit-a"}
	rows, _ := computeTableRows(ds, domain.LimitRegionalOverride, scope, "", "", true, false)
	require.Len(t, rows, 1)
	require.Equal(t, "limit-a", rows[0][0])

	// No scope: the full flat list is shown.
	rows, _ = computeTableRows(ds, domain.LimitRegionalOverride, nil, "", "", true, false)
	require.Len(t, rows, 2)
}

// TestRegionalOverrideSource_UnrelatedScopeNotFiltered protects the export
// path, which passes m.scope unguarded: a scope that does not own the
// category (e.g. a lingering Tenant scope) must not narrow the rows.
func TestRegionalOverrideSource_UnrelatedScopeNotFiltered(t *testing.T) {
	t.Parallel()
	ds := &models.Dataset{
		LimitRegionalOverrides: []models.LimitRegionalOverride{
			{Name: "limit-a"},
			{Name: "limit-b"},
		},
	}
	src := rowSources[domain.LimitRegionalOverride]
	rows := src.rows(rowCtx{
		dataset: ds,
		scope:   &domain.Scope{Category: domain.Tenant, Name: "limit-a"},
		export:  true,
	})
	require.Len(t, rows, 2, "a non-owning scope must not filter the rows")
}
