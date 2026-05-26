package models

// Filterable represents an item that can be filtered by fields.
type Filterable interface {
	FilterableFields() []string
}

// NamedItem represents an item with a name.
type NamedItem interface {
	GetName() string
}

// NamedFilterable represents an item that is both named and filterable.
type NamedFilterable interface {
	NamedItem
	Filterable
	Faulty
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

// RealmedID is implemented by OCI resources whose full OCID is
// constructed from a realm and a region plus a stored name suffix
// — DedicatedAICluster and ImportedModel today. Distinct from
// NamedItem.GetName (which returns just the suffix) and used by the
// TUI's CopyItemName action to produce the full OCID.
type RealmedID interface {
	GetID(realm, region string) string
}

// RealmedTenancyID is implemented by OCI resources whose owning
// tenancy OCID is constructed from a realm plus a stored tenancy-id
// suffix — DedicatedAICluster and ImportedModel today. Distinct
// from TenancyOverride.GetTenantID (which takes no realm and is
// implemented by file-backed override types whose stored TenantID
// is already the full identifier).
type RealmedTenancyID interface {
	GetTenantID(realm string) string
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
