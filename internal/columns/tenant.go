package columns

import (
	"fmt"
	"strings"

	"github.com/jingle2008/toolkit/pkg/models"
)

// TenantColumns is the canonical column set for domain.Tenant. All
// columns Default==true. The second column's Title is "OCID" (matches
// TUI; the legacy CLI header was "IDS"); the cell content is the
// comma-joined IDs slice (matches legacy CLI).
var TenantColumns = Set[models.Tenant]{Columns: []Column[models.Tenant]{
	{Title: "Name", Key: "name", Default: true, Ratio: 0.20,
		Render: func(t models.Tenant) string { return t.Name }},
	{Title: "OCID", Key: "ocid", Default: true, Ratio: 0.60,
		Render: func(t models.Tenant) string { return strings.Join(t.IDs, ",") }},
	{Title: "Internal", Key: "internal", Default: true, Ratio: 0.10,
		Render: func(t models.Tenant) string { return fmt.Sprint(t.IsInternal) }},
	{Title: "Note", Key: "note", Default: true, Ratio: 0.10,
		Render: func(t models.Tenant) string { return t.Note }},
}}
