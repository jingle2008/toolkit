package columns

import (
	"strconv"

	"github.com/jingle2008/toolkit/pkg/models"
)

func dacUnitShapeOrProfile(d models.DedicatedAICluster) string {
	if d.UnitShape != "" {
		return d.UnitShape
	}
	return d.Profile
}

// DacColumns is the canonical column set for domain.DedicatedAICluster.
//
// Ordering invariant: row[0]=Name, row[1]=Tenant. internal/ui/tui/export_csv.go
// depends on this ordering when substituting the realm/region-qualified ID
// for the Name column in CSV export. If you reorder, fix the substitution
// there too.
//
// Ordering is name-first, tenant-key-second (matches TUI; Decision #4).
var DacColumns = GroupedSet[models.DedicatedAICluster]{Columns: []GroupedColumn[models.DedicatedAICluster]{
	{Title: "Name", Key: "name", Ratio: 0.35,
		Render: func(_ string, d models.DedicatedAICluster) string { return d.Name }},
	{Title: "Tenant", Key: "tenant", Ratio: 0.16,
		Render: func(k string, _ models.DedicatedAICluster) string { return k }},
	{Title: "Internal", Key: "internal", Ratio: 0.05,
		Render: func(_ string, d models.DedicatedAICluster) string { return d.GetOwnerState() }},
	{Title: "Usage", Key: "usage", Ratio: 0.05,
		Render: func(_ string, d models.DedicatedAICluster) string { return d.GetUsage() }},
	{Title: "Type", Key: "type", Ratio: 0.06,
		Render: func(_ string, d models.DedicatedAICluster) string { return d.Type }},
	{Title: "Model", Key: "model", Ratio: 0.09,
		Render: func(_ string, d models.DedicatedAICluster) string { return d.ModelName }},
	{Title: "Shape/Profile", Key: "shape-profile", Ratio: 0.12,
		Render: func(_ string, d models.DedicatedAICluster) string { return dacUnitShapeOrProfile(d) }},
	{Title: "Size", Key: "size", Ratio: 0.04,
		Render: func(_ string, d models.DedicatedAICluster) string { return strconv.Itoa(d.Size) }},
	{Title: "Age", Key: "age", Ratio: 0.04,
		Render: func(_ string, d models.DedicatedAICluster) string { return d.Age }},
	{Title: "Status", Key: "status", Ratio: 0.04,
		Render: func(_ string, d models.DedicatedAICluster) string { return d.Status }},
}}
