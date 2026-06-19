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
// sub-categories, the grouping key renders at row[1]. itemKeyFrom ("view
// details") and parentScope ("jump to parent") both read row[1] to recover the
// parent's key — see internal/ui/tui/table_utils.go. Each case names the
// parent category so categories grouped by different parents (Tenant for
// ImportedModel/DAC, GPUNode for GPUWorkload) share one guard.
//
// Unlike the hand-built rows in parent_nav_test.go, this test renders rows from
// the REAL column sets via columns.RenderTable, so reordering a column set
// (e.g. moving the group key off index 1) fails here instead of silently
// breaking the two features at runtime. Sentinel values are chosen distinct
// from every other cell so a misordering can never accidentally still match.
func TestGroupKeyAtRowIndex1(t *testing.T) {
	t.Parallel()

	const (
		groupKey = "owning-parent-XYZ"
		name     = "row0-name-ABC"
		other    = "distinct-other-cell"
	)

	cases := []struct {
		name      string
		category  domain.Category
		parentCat domain.Category
		items     any
	}{
		{
			name:      "ImportedModel",
			category:  domain.ImportedModel,
			parentCat: domain.Tenant,
			items: map[string][]models.ImportedModel{
				groupKey: {{
					BaseModel: models.BaseModel{Name: name, DisplayName: other, Vendor: other},
					Namespace: other,
				}},
			},
		},
		{
			name:      "DedicatedAICluster",
			category:  domain.DedicatedAICluster,
			parentCat: domain.Tenant,
			items: map[string][]models.DedicatedAICluster{
				groupKey: {{Name: name, Type: other}},
			},
		},
		{
			name:      "GPUWorkload",
			category:  domain.GPUWorkload,
			parentCat: domain.GPUNode,
			items: map[string][]models.GPUWorkload{
				groupKey: {{Name: name, Namespace: other, Model: other, Runtime: other, Mode: other}},
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

			// "jump to parent" must recover the parent's key from row[1].
			parent, ok := parentScope(tc.category, row)
			require.True(t, ok, "category should have a parent")
			require.Equal(t, tc.parentCat, parent.Category)
			require.Equal(t, groupKey, parent.Name,
				"parentScope must read the group key from row[1]; did a column move off index 1?")

			// "view details" must build a scoped key of {groupKey, name}.
			key, ok := itemKeyFrom(tc.category, row).(models.ScopedItemKey)
			require.True(t, ok, "grouped category should yield a ScopedItemKey")
			require.Equal(t, groupKey, key.Scope,
				"itemKeyFrom must read the scope (group key) from row[1]; did a column move off index 1?")
			require.Equal(t, name, key.Name)
		})
	}
}
