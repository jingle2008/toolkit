package columns

import (
	"strings"

	"github.com/jingle2008/toolkit/pkg/models"
)

// RegionalOverrideColumns is parameterized by the concrete DefinitionOverride
// type (ConsolePropertyRegionalOverride or PropertyRegionalOverride); both
// satisfy models.DefinitionOverride.
func RegionalOverrideColumns[T models.DefinitionOverride]() Set[T] {
	return Set[T]{Columns: []Column[T]{
		{Title: "Name", Key: "name", Default: true, Ratio: 0.40,
			Render: func(o T) string { return o.GetName() }},
		{Title: "Regions", Key: "regions", Default: true, Ratio: 0.40,
			Render: func(o T) string { return strings.Join(o.GetRegions(), ", ") }},
		{Title: "Value", Key: "value", Default: true, Ratio: 0.20,
			Render: func(o T) string { return o.GetValue() }},
	}}
}

var (
	ConsolePropertyRegionalOverrideColumns = RegionalOverrideColumns[models.ConsolePropertyRegionalOverride]()
	PropertyRegionalOverrideColumns        = RegionalOverrideColumns[models.PropertyRegionalOverride]()
)
