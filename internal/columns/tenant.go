package columns

import (
	"fmt"
	"strings"

	"github.com/jingle2008/toolkit/pkg/models"
)

// TenantColumns is the canonical column set for domain.Tenant.
// All Default==true (preserves today's CLI table NAME|IDS|INTERNAL|NOTE).
var TenantColumns = Set[models.Tenant]{Columns: []Column[models.Tenant]{
	{Title: "Name", Key: "name", Default: true, Ratio: 0.20,
		Render: func(t models.Tenant) string { return t.Name }},
	{Title: "OCID", Key: "ids", Default: true, Ratio: 0.60,
		Render: func(t models.Tenant) string { return strings.Join(t.IDs, ",") }},
	{Title: "Internal", Key: "internal", Default: true, Ratio: 0.10,
		Render: func(t models.Tenant) string { return fmt.Sprint(t.IsInternal) }},
	{Title: "Note", Key: "note", Default: true, Ratio: 0.10,
		Render: func(t models.Tenant) string { return t.Note }},
}}
