package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/table"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/columns"
	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/pkg/models"
)

// TestGroupKeyAtRowIndex1 guards the load-bearing invariant that, for grouped
// tenant-owned categories, the grouping key (tenant) renders at row[1].
// itemKeyFrom ("view details") and ownerScope ("jump to owner") both read
// row[1] to recover the owning tenant — see internal/ui/tui/table_utils.go.
//
// Unlike the hand-built rows in owner_nav_test.go, this test renders rows from
// the REAL column sets via columns.RenderTable, so reordering a column set
// (e.g. moving Tenant off index 1) fails here instead of silently breaking the
// two features at runtime. Sentinel values are chosen distinct from every
// other cell so a misordering can never accidentally still match.
func TestGroupKeyAtRowIndex1(t *testing.T) {
	t.Parallel()

	const (
		tenant = "owning-tenant-XYZ"
		name   = "row0-name-ABC"
		other  = "distinct-other-cell"
	)

	cases := []struct {
		name     string
		category domain.Category
		items    any
	}{
		{
			name:     "ImportedModel",
			category: domain.ImportedModel,
			items: map[string][]models.ImportedModel{
				tenant: {{
					BaseModel: models.BaseModel{Name: name, DisplayName: other, Vendor: other},
					Namespace: other,
				}},
			},
		},
		{
			name:     "DedicatedAICluster",
			category: domain.DedicatedAICluster,
			items: map[string][]models.DedicatedAICluster{
				tenant: {{Name: name, Type: other}},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, rows, err := columns.RenderTable(tc.category, tc.items, nil)
			require.NoError(t, err)
			require.Len(t, rows, 1)
			row := table.Row(rows[0])

			// "jump to owner" must recover the owning tenant from row[1].
			owner, ok := ownerScope(tc.category, row)
			require.True(t, ok, "category should have an owner")
			require.Equal(t, domain.Tenant, owner.Category)
			require.Equal(t, tenant, owner.Name,
				"ownerScope must read the tenant from row[1]; did a column move off index 1?")

			// "view details" must build a scoped key of {tenant, name}.
			key, ok := itemKeyFrom(tc.category, row).(models.ScopedItemKey)
			require.True(t, ok, "grouped category should yield a ScopedItemKey")
			require.Equal(t, tenant, key.Scope,
				"itemKeyFrom must read the scope (tenant) from row[1]; did a column move off index 1?")
			require.Equal(t, name, key.Name)
		})
	}
}
