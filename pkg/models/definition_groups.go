package models

// LimitDefinitionGroup groups limit definitions by name.
type LimitDefinitionGroup struct {
	Name   string            `json:"group"`
	Values []LimitDefinition `json:"values"`
}

// ConsolePropertyDefinitionGroup groups console property definitions by service name.
type ConsolePropertyDefinitionGroup struct {
	Name   string                      `json:"service"`
	Values []ConsolePropertyDefinition `json:"values"`
}

// PropertyDefinitionGroup groups property definitions by name.
type PropertyDefinitionGroup struct {
	Name   string               `json:"group"`
	Values []PropertyDefinition `json:"values"`
}
