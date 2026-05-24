package columns

import "github.com/jingle2008/toolkit/pkg/models"

// DefinitionColumns is parameterized by the concrete Definition
// type (ConsolePropertyDefinition or PropertyDefinition); both
// satisfy models.Definition.
func DefinitionColumns[T models.Definition]() Set[T] {
	return Set[T]{Columns: []Column[T]{
		{Title: "Name", Key: "name", Ratio: 0.38,
			Render: func(d T) string { return d.GetName() }},
		{Title: "Description", Key: "description", Ratio: 0.50,
			Render: func(d T) string { return d.GetDescription() }},
		{Title: "Value", Key: "value", Ratio: 0.12,
			Render: func(d T) string { return d.GetValue() }},
	}}
}

// Pre-instantiated typed sets — the registry switch uses these directly
// so each call doesn't reconstruct the closures.
var (
	ConsolePropertyDefinitionColumns = DefinitionColumns[models.ConsolePropertyDefinition]()
	PropertyDefinitionColumns        = DefinitionColumns[models.PropertyDefinition]()
)
