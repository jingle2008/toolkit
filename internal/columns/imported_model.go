package columns

import (
	"strconv"

	"github.com/jingle2008/toolkit/pkg/models"
)

// ImportedModelColumns is the canonical column set for domain.ImportedModel.
// 8 columns, ratios sum to 1.00. Mirrors BaseModel's operator-useful set
// (Name, Display Name, Size, Context, Vendor, Status) plus the imported-only
// Tenant (grouping key) and Namespace columns. The Internal/owner-state
// column was dropped from the table; owner state stays reachable via the
// Owner shortcut and `-o json`. Version stays reachable via `-o json`.
//
// Tenant MUST stay at index 1: itemKeyFrom/ownerScope in
// internal/ui/tui/table_utils.go derive the scoped key and owner from
// row[1] for grouped categories ("view details" and "jump to owner").
var ImportedModelColumns = GroupedSet[models.ImportedModel]{Columns: []GroupedColumn[models.ImportedModel]{
	{
		Title: "Name", Key: "name", Ratio: 0.18, TruncateMiddle: true,
		Render: func(_ string, m models.ImportedModel) string { return m.Name },
		RenderForExport: func(realm, region string, _ string, m models.ImportedModel) string {
			return m.OCID(realm, region)
		},
	},
	{
		Title: "Tenant", Key: "tenant", Ratio: 0.17, TruncateMiddle: true,
		Render: func(k string, _ models.ImportedModel) string { return k },
		RenderForExport: func(realm, _ string, _ string, m models.ImportedModel) string {
			return m.TenancyOCID(realm)
		},
	},
	{
		Title: "Display Name", Key: "display-name", Ratio: 0.19,
		Render: func(_ string, m models.ImportedModel) string { return m.DisplayName },
	},
	{
		Title: "Namespace", Key: "namespace", Ratio: 0.12, TruncateMiddle: true,
		Render: func(_ string, m models.ImportedModel) string { return m.Namespace },
	},
	{
		Title: "Size", Key: "size", Ratio: 0.08,
		Render: func(_ string, m models.ImportedModel) string { return m.ParameterSize },
	},
	{
		Title: "Context", Key: "context", Ratio: 0.08,
		Render: func(_ string, m models.ImportedModel) string { return strconv.Itoa(m.MaxTokens) },
	},
	{
		Title: "Vendor", Key: "vendor", Ratio: 0.10,
		Render: func(_ string, m models.ImportedModel) string { return m.Vendor },
	},
	{
		Title: "Status", Key: "status", Ratio: 0.08,
		Render: func(_ string, m models.ImportedModel) string { return m.Status },
	},
}}
