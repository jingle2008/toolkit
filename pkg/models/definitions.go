package models

type Filterable interface {
	GetFilterableFields() []string
}

type KeyedItem interface {
	GetKey() string
}

type NamedItem interface {
	GetName() string
}

type NamedFilterable interface {
	NamedItem
	Filterable
}

type Definition interface {
	NamedFilterable
	GetDescription() string
	GetValue() string
}

type TenancyOverride interface {
	GetTenantId() string
}

type DefinitionOverride interface {
	NamedFilterable
	GetRegions() []string
	GetValue() string
}

type LimitDefinitionGroup struct {
	Name   string            `json:"group"`
	Values []LimitDefinition `json:"values"`
}

type ConsolePropertyDefinitionGroup struct {
	Name   string                      `json:"service"`
	Values []ConsolePropertyDefinition `json:"values"`
}

type PropertyDefinitionGroup struct {
	Name   string               `json:"group"`
	Values []PropertyDefinition `json:"values"`
}
