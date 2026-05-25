package columns

import (
	"github.com/jingle2008/toolkit/pkg/models"
)

// ImportedModelColumns is the canonical column set for domain.ImportedModel.
// 6 columns, ratios sum to 1.00. Version was dropped (imported models are
// keyed by Name, which already carries the operator-meaningful versioning
// convention); its 0.05 width was released to Vendor so vendor strings have
// room to render without truncation. Version stays reachable via `-o json`.
// Ordering is name-first, tenant-key-second (matches TUI; Decision #4).
var ImportedModelColumns = GroupedSet[models.ImportedModel]{Columns: []GroupedColumn[models.ImportedModel]{
	{
		Title: "Name", Key: "name", Ratio: 0.20, TruncateMiddle: true,
		Render: func(_ string, m models.ImportedModel) string { return m.Name },
		ExportRender: func(realm, region string, _ string, m models.ImportedModel) string {
			return m.GetID(realm, region)
		},
	},
	{
		Title: "Tenant", Key: "tenant", Ratio: 0.22, TruncateMiddle: true,
		Render: func(k string, _ models.ImportedModel) string { return k },
		ExportRender: func(realm, _ string, _ string, m models.ImportedModel) string {
			return m.GetTenantID(realm)
		},
	},
	{
		Title: "Namespace", Key: "namespace", Ratio: 0.15, TruncateMiddle: true,
		Render: func(_ string, m models.ImportedModel) string { return m.Namespace },
	},
	{
		Title: "Display Name", Key: "display-name", Ratio: 0.27,
		Render: func(_ string, m models.ImportedModel) string { return m.DisplayName },
	},
	{
		Title: "Vendor", Key: "vendor", Ratio: 0.10,
		Render: func(_ string, m models.ImportedModel) string { return m.Vendor },
	},
	{
		Title: "Status", Key: "status", Ratio: 0.06,
		Render: func(_ string, m models.ImportedModel) string { return m.Status },
	},
}}
