package tui

import (
	"testing"

	"charm.land/bubbles/v2/table"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/columns"
	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/pkg/models"
)

// TestGroupKeyAtRowIndex1 guards the load-bearing invariant that, for grouped
// sub-categories, the grouping key renders at row[1]. itemKeyFrom ("view
// details") reads row[1] to recover the parent's key — see
// internal/ui/tui/table_utils.go. The cases cover categories grouped by
// different parents (Tenant for ImportedModel/DAC, GPUNode for GPUWorkload,
// DedicatedAICluster for InferenceService).
//
// Unlike hand-built rows, this test renders rows from the REAL column sets
// via columns.RenderTable, so reordering a column set (e.g. moving the group
// key off index 1) fails here instead of silently breaking details (and the
// go-to targets built from the same key) at runtime. Sentinel values are chosen distinct
// from every other cell so a misordering can never accidentally still match.
func TestGroupKeyAtRowIndex1(t *testing.T) {
	t.Parallel()

	const (
		groupKey = "owning-parent-XYZ"
		name     = "row0-name-ABC"
		other    = "distinct-other-cell"
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
				groupKey: {{
					BaseModel: models.BaseModel{Name: name, DisplayName: other, Vendor: other},
					Namespace: other,
				}},
			},
		},
		{
			name:     "DedicatedAICluster",
			category: domain.DedicatedAICluster,
			items: map[string][]models.DedicatedAICluster{
				groupKey: {{Name: name, Type: other}},
			},
		},
		{
			name:     "GPUWorkload",
			category: domain.GPUWorkload,
			items: map[string][]models.GPUWorkload{
				groupKey: {{Name: name, Namespace: other, Model: other, Runtime: other, Mode: other}},
			},
		},
		{
			name:     "InferenceService",
			category: domain.InferenceService,
			items: map[string][]models.InferenceService{
				groupKey: {{Name: name, Namespace: other, Type: other, Model: other, Runtime: other}},
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

			// "view details" must build a scoped key of {groupKey, name}.
			key, ok := itemKeyFrom(tc.category, row).(models.ScopedItemKey)
			require.True(t, ok, "grouped category should yield a ScopedItemKey")
			require.Equal(t, groupKey, key.Scope,
				"itemKeyFrom must read the scope (group key) from row[1]; did a column move off index 1?")
			require.Equal(t, name, key.Name)
		})
	}
}
