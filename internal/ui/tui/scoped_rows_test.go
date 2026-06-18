package tui

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/pkg/models"
)

// TestComputeTableRows_RegionalOverrideScopeFilter guards the context
// (scope) filter for the flat regional-override categories: when scoped to
// a definition, only the overrides for that definition are shown — both on
// the display path and the CSV export path. Covers all three variants so a
// future copy-paste mismatch in the rowSources entries is caught.
func TestComputeTableRows_RegionalOverrideScopeFilter(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		cat     domain.Category
		owner   domain.Category
		dataset func() *models.Dataset
	}{
		{
			"limit", domain.LimitRegionalOverride, domain.LimitDefinition,
			func() *models.Dataset {
				return &models.Dataset{LimitRegionalOverrides: []models.LimitRegionalOverride{
					{Name: "item-a", Regions: []string{"r1"}},
					{Name: "item-b", Regions: []string{"r2"}},
				}}
			},
		},
		{
			"console property", domain.ConsolePropertyRegionalOverride, domain.ConsolePropertyDefinition,
			func() *models.Dataset {
				return &models.Dataset{ConsolePropertyRegionalOverrides: []models.ConsolePropertyRegionalOverride{
					{Name: "item-a", Regions: []string{"r1"}},
					{Name: "item-b", Regions: []string{"r2"}},
				}}
			},
		},
		{
			"property", domain.PropertyRegionalOverride, domain.PropertyDefinition,
			func() *models.Dataset {
				return &models.Dataset{PropertyRegionalOverrides: []models.PropertyRegionalOverride{
					{Name: "item-a", Regions: []string{"r1"}},
					{Name: "item-b", Regions: []string{"r2"}},
				}}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ds := tc.dataset()
			scope := &domain.Scope{Category: tc.owner, Name: "item-a"}

			// Display path, scoped to item-a's definition: only item-a.
			rows, _ := computeTableRows(ds, tc.cat, scope, "", "", true, false)
			require.Len(t, rows, 1)
			require.Equal(t, "item-a", rows[0][0])

			// No scope: the full flat list is shown.
			rows, _ = computeTableRows(ds, tc.cat, nil, "", "", true, false)
			require.Len(t, rows, 2)

			// Export path, correctly scoped: narrows the same way.
			exp := rowSources[tc.cat].rows(rowCtx{dataset: ds, scope: scope, export: true})
			require.Len(t, exp, 1)
			require.Equal(t, "item-a", exp[0][0])
		})
	}
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
