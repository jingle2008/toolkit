package columns

import (
	"fmt"
	"strings"

	"github.com/jingle2008/toolkit/pkg/models"
)

// TenantColumns is the canonical column set for domain.Tenant. The
// second column's Title is "OCIDs" (plural) — one tenant can have
// multiple OCIDs (Tenant.IDs is a slice, and Tenant.IsFaulty flags
// >1 IDs). The cell joins them with commas. Key stays "ocid" for
// CLI --columns stability with prior releases.
var TenantColumns = Set[models.Tenant]{Columns: []Column[models.Tenant]{
	{Title: "Name", Key: "name", Ratio: 0.20,
		Render: func(t models.Tenant) string { return t.Name }},
	{Title: "OCIDs", Key: "ocid", Ratio: 0.60,
		Render: func(t models.Tenant) string { return strings.Join(t.IDs, ",") }},
	{Title: "Internal", Key: "internal", Ratio: 0.10,
		Render: func(t models.Tenant) string { return fmt.Sprint(t.IsInternal) }},
	{Title: "Note", Key: "note", Ratio: 0.10,
		Render: func(t models.Tenant) string { return t.Note }},
}}
