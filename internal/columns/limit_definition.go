package columns

import (
	"github.com/jingle2008/toolkit/pkg/models"
)

// LimitDefinitionColumns is the canonical column set for domain.LimitDefinition.
// All Default==true, matching today's CLI and TUI tables.
var LimitDefinitionColumns = Set[models.LimitDefinition]{Columns: []Column[models.LimitDefinition]{
	{Title: "Name", Key: "name", Default: true, Ratio: 0.32,
		Render: func(d models.LimitDefinition) string { return d.Name }},
	{Title: "Description", Key: "description", Default: true, Ratio: 0.48,
		Render: func(d models.LimitDefinition) string { return d.Description }},
	{Title: "Scope", Key: "scope", Default: true, Ratio: 0.08,
		Render: func(d models.LimitDefinition) string { return d.Scope }},
	{Title: "Min", Key: "min", Default: true, Ratio: 0.06,
		Render: func(d models.LimitDefinition) string { return d.DefaultMin }},
	{Title: "Max", Key: "max", Default: true, Ratio: 0.06,
		Render: func(d models.LimitDefinition) string { return d.DefaultMax }},
}}
