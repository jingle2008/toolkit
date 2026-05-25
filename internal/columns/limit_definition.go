package columns

import (
	"github.com/jingle2008/toolkit/pkg/models"
)

// LimitDefinitionColumns is the canonical column set for domain.LimitDefinition.
// Matches today's CLI and TUI tables.
var LimitDefinitionColumns = Set[models.LimitDefinition]{Columns: []Column[models.LimitDefinition]{
	{
		Title: "Name", Key: "name", Ratio: 0.32,
		Render: func(d models.LimitDefinition) string { return d.Name },
	},
	{
		Title: "Description", Key: "description", Ratio: 0.48,
		Render: func(d models.LimitDefinition) string { return d.Description },
	},
	{
		Title: "Scope", Key: "scope", Ratio: 0.08,
		Render: func(d models.LimitDefinition) string { return d.Scope },
	},
	{
		Title: "Min", Key: "min", Ratio: 0.06,
		Render: func(d models.LimitDefinition) string { return d.DefaultMin },
	},
	{
		Title: "Max", Key: "max", Ratio: 0.06,
		Render: func(d models.LimitDefinition) string { return d.DefaultMax },
	},
}}
