package columns

import (
	"fmt"

	"github.com/jingle2008/toolkit/pkg/models"
)

// TenantColumns is the canonical column set for domain.Tenant: one tenant
// is one OCID.
var TenantColumns = Set[models.Tenant]{Columns: []Column[models.Tenant]{
	{
		Title: "Name", Key: "name", Ratio: 0.20,
		Render: func(t models.Tenant) string { return t.Name },
	},
	{
		Title: "OCID", Key: "ocid", Ratio: 0.50,
		Render: func(t models.Tenant) string { return t.ID },
	},
	{
		Title: "Internal", Key: "internal", Ratio: 0.10,
		Render: func(t models.Tenant) string { return fmt.Sprint(t.IsInternal) },
	},
	{
		Title: "Status", Key: "status", Ratio: 0.10,
		Render: func(t models.Tenant) string { return t.Status },
	},
	{
		Title: "Note", Key: "note", Ratio: 0.10,
		Render: func(t models.Tenant) string { return t.Note },
	},
}}
