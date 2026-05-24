package columns

import (
	"github.com/jingle2008/toolkit/pkg/models"
)

// ImportedModelColumns is the canonical column set for domain.ImportedModel.
// Unions today's CLI columns (Tenant, Name, Namespace, Vendor, Version, Status)
// with today's TUI columns (Name, Tenant, Namespace, Display Name, Status).
// Vendor and Version are Default==false so they remain reachable via --columns.
// Ordering is name-first, tenant-key-second (matches TUI; Decision #4).
var ImportedModelColumns = GroupedSet[models.ImportedModel]{Columns: []GroupedColumn[models.ImportedModel]{
	{Title: "Name", Key: "name", Default: true, Ratio: 0.20,
		Render: func(_ string, m models.ImportedModel) string { return m.Name }},
	{Title: "Tenant", Key: "tenant", Default: true, Ratio: 0.22,
		Render: func(k string, _ models.ImportedModel) string { return k }},
	{Title: "Namespace", Key: "namespace", Default: true, Ratio: 0.15,
		Render: func(_ string, m models.ImportedModel) string { return m.Namespace }},
	{Title: "Display Name", Key: "display-name", Default: true, Ratio: 0.27,
		Render: func(_ string, m models.ImportedModel) string { return m.DisplayName }},
	{Title: "Vendor", Key: "vendor", Default: true, Ratio: 0.05,
		Render: func(_ string, m models.ImportedModel) string { return m.Vendor }},
	{Title: "Version", Key: "version", Default: true, Ratio: 0.05,
		Render: func(_ string, m models.ImportedModel) string { return m.Version }},
	{Title: "Status", Key: "status", Default: true, Ratio: 0.06,
		Render: func(_ string, m models.ImportedModel) string { return m.Status }},
}}
