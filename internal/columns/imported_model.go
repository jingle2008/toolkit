package columns

import (
	"cmp"
	"strconv"
	"strings"

	"github.com/jingle2008/toolkit/pkg/models"
)

// ImportedModelColumns is the canonical column set for domain.ImportedModel.
// 8 columns, ratios sum to 1.00. Mirrors BaseModel's operator-useful set
// (Name, Display Name, Size, Context, Vendor, Status) plus the imported-only
// Tenant (grouping key) and Namespace columns. The Internal/owner-state
// column was dropped from the table; owner state stays reachable via the
// go-to-related shortcut and `-o json`. Version stays reachable via `-o json`.
//
// Tenant MUST stay at index 1: itemKeyFrom in
// internal/ui/tui/table_utils.go derives the scoped key from row[1] for
// grouped categories ("view details" and "go to related").
var ImportedModelColumns = GroupedSet[models.ImportedModel]{Columns: []GroupedColumn[models.ImportedModel]{
	{
		Title: "Name", Key: "name", Ratio: 0.18,
		Render: func(_ string, m models.ImportedModel) string { return m.Name },
		RenderForExport: func(realm, region string, _ string, m models.ImportedModel) string {
			return m.OCID(realm, region)
		},
	},
	{
		Title: "Tenant", Key: "tenant", Ratio: 0.17,
		Render: func(k string, _ models.ImportedModel) string { return k },
		// An orphan has no tenancy OCID; its key, UNKNOWN_TENANCY, stays.
		RenderForExport: func(realm, _ string, k string, m models.ImportedModel) string {
			return cmp.Or(m.TenancyOCID(realm), k)
		},
	},
	{
		Title: "Display Name", Key: "display-name", Ratio: 0.22,
		Render: func(_ string, m models.ImportedModel) string { return m.DisplayName },
	},
	{
		// Every copy's namespace, primary (usually `default`) first.
		Title: "Namespace", Key: "namespace", Ratio: 0.15,
		Render: func(_ string, m models.ImportedModel) string { return strings.Join(m.Namespaces(), ", ") },
	},
	{
		Title: "Size", Key: "size", Ratio: 0.07,
		Render: func(_ string, m models.ImportedModel) string { return m.ParameterSize },
	},
	{
		Title: "Context", Key: "context", Ratio: 0.08,
		Render: func(_ string, m models.ImportedModel) string { return strconv.Itoa(m.MaxTokens) },
	},
	{
		Title: "Vendor", Key: "vendor", Ratio: 0.08,
		Render: func(_ string, m models.ImportedModel) string { return m.Vendor },
	},
	{
		Title: "Status", Key: "status", Ratio: 0.05,
		Render: func(_ string, m models.ImportedModel) string { return m.Status },
	},
}}
