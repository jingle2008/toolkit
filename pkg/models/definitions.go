package models

// Filterable represents an item that can be filtered by fields.
type Filterable interface {
	GetFilterableFields() []string
}

// KeyedItem represents an item with a unique key.
type KeyedItem interface {
	GetKey() string
}

// NamedItem represents an item with a name.
type NamedItem interface {
	GetName() string
}

// NamedFilterable represents an item that is both named and filterable.
type NamedFilterable interface {
	NamedItem
	Filterable
}

// Definition represents a definition item with description and value.
type Definition interface {
	NamedFilterable
	GetDescription() string
	GetValue() string
}

// TenancyOverride represents an override with a tenant ID.
type TenancyOverride interface {
	GetTenantID() string
}

// DefinitionOverride represents a definition override with regions and value.
type DefinitionOverride interface {
	NamedFilterable
	GetRegions() []string
	GetValue() string
}

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
