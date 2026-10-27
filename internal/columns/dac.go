package columns

import (
	"cmp"
	"strconv"

	"github.com/jingle2008/toolkit/pkg/models"
)

func dacShapeOrProfile(d models.DedicatedAICluster) string {
	if d.UnitShape != "" {
		return d.UnitShape
	}
	return d.Profile
}

// DACColumns is the canonical column set for domain.DedicatedAICluster.
//
// Ordering is name-first, tenant-key-second (matches TUI; Decision #4).
// The Name and Tenant columns carry an RenderForExport closure that
// produces fully-qualified OCIDs (vs the suffix-only display form);
// substitution happens per-column inside the column registry, so
// reordering the columns here doesn't require companion edits in
// the CSV export path.
//
// Name was rebalanced from 0.35 to 0.20 to match ImportedModel and free
// width for the narrow sortable columns (Internal/Usage/Size/Age); they
// previously had no headroom for the ↕ sortable indicator (or even the
// full title in Internal's case). The freed 0.15 is redistributed to
// the columns that needed breathing room, including Status — its 6-char
// title and ACTIVE/FAILED/READY values would truncate at ratio 0.04.
var DACColumns = GroupedSet[models.DedicatedAICluster]{Columns: []GroupedColumn[models.DedicatedAICluster]{
	{
		Title: "Name", Key: "name", Ratio: 0.20,
		Render: func(_ string, d models.DedicatedAICluster) string { return d.Name },
		RenderForExport: func(realm, region string, _ string, d models.DedicatedAICluster) string {
			return d.OCID(realm, region)
		},
	},
	{
		Title: "Tenant", Key: "tenant", Ratio: 0.17,
		Render: func(k string, _ models.DedicatedAICluster) string { return k },
		// An orphan has no tenancy OCID; its key, UNKNOWN_TENANCY, stays.
		RenderForExport: func(realm, _ string, k string, d models.DedicatedAICluster) string {
			return cmp.Or(d.TenancyOCID(realm), k)
		},
	},
	{
		Title: "Internal", Key: "internal", Ratio: 0.09,
		Render: func(_ string, d models.DedicatedAICluster) string { return d.OwnerState() },
	},
	{
		Title: "Usage", Key: "usage", Ratio: 0.07,
		Render: func(_ string, d models.DedicatedAICluster) string { return d.Usage() },
	},
	{
		Title: "Type", Key: "type", Ratio: 0.07,
		Render: func(_ string, d models.DedicatedAICluster) string { return d.Type },
	},
	{
		Title: "Model", Key: "model", Ratio: 0.10,
		Render: func(_ string, d models.DedicatedAICluster) string { return d.ModelName },
	},
	{
		Title: "Shape/Profile", Key: "shape-profile", Ratio: 0.12,
		Render: func(_ string, d models.DedicatedAICluster) string { return dacShapeOrProfile(d) },
	},
	{
		Title: "Size", Key: "size", Ratio: 0.06,
		Render: func(_ string, d models.DedicatedAICluster) string { return strconv.Itoa(d.Size) },
	},
	{
		Title: "Age", Key: "age", Ratio: 0.06,
		Render: func(_ string, d models.DedicatedAICluster) string { return models.AgeOf(d.CreatedAt) },
	},
	{
		Title: "Status", Key: "status", Ratio: 0.06,
		Render: func(_ string, d models.DedicatedAICluster) string { return d.Status },
	},
}}
