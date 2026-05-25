package columns

import (
	"strings"

	"github.com/jingle2008/toolkit/pkg/models"
)

// ServiceTenancyColumns is the canonical column set for domain.ServiceTenancy.
// The Environment field is displayed as "Type" — the title kept from the
// pre-canonical TUI because the SortType binding (registry.go) and operator
// mental model both expect "Type". The struct field stays Environment.
var ServiceTenancyColumns = Set[models.ServiceTenancy]{Columns: []Column[models.ServiceTenancy]{
	{
		Title: "Name", Key: "name", Ratio: 0.15,
		Render: func(s models.ServiceTenancy) string { return s.Name },
	},
	{
		Title: "Realm", Key: "realm", Ratio: 0.10,
		Render: func(s models.ServiceTenancy) string { return s.Realm },
	},
	{
		Title: "Type", Key: "environment", Ratio: 0.10,
		Render: func(s models.ServiceTenancy) string { return s.Environment },
	},
	{
		Title: "Home Region", Key: "home-region", Ratio: 0.15,
		Render: func(s models.ServiceTenancy) string { return s.HomeRegion },
	},
	{
		Title: "Regions", Key: "regions", Ratio: 0.50,
		Render: func(s models.ServiceTenancy) string { return strings.Join(s.Regions, ", ") },
	},
}}
