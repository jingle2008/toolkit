package columns

import (
	"github.com/jingle2008/toolkit/pkg/models"
)

// EnvironmentColumns is the canonical column set for domain.Environment.
// All Default==true, matching today's TUI column order.
var EnvironmentColumns = Set[models.Environment]{Columns: []Column[models.Environment]{
	{Title: "Name", Key: "name", Default: true, Ratio: 0.20,
		Render: func(e models.Environment) string { return e.GetName() }},
	{Title: "Realm", Key: "realm", Default: true, Ratio: 0.15,
		Render: func(e models.Environment) string { return e.Realm }},
	{Title: "Type", Key: "type", Default: true, Ratio: 0.15,
		Render: func(e models.Environment) string { return e.Type }},
	{Title: "Region", Key: "region", Default: true, Ratio: 0.50,
		Render: func(e models.Environment) string { return e.Region }},
}}
