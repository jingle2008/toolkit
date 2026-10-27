package models

import "fmt"

// Filterable represents an item that can be filtered by fields.
type Filterable interface {
	FilterableFields() []string
}

// NamedItem represents an item with a name.
type NamedItem interface {
	GetName() string
}

// QualifiedItem is a row of a flat category whose name alone can repeat
// (service tenancies share names across realms). QualifiedName is
// <qualifier>/<name>, which names one row where the name doesn't.
type QualifiedItem interface {
	NamedItem
	QualifiedName() string
}

// VariantItem is a row of a grouped category that can share both its
// group and its name with another row (one model artifact built for
// several GPU configs). Variant tells those rows apart:
// <group>/<name>@<variant> names one.
type VariantItem interface {
	Variant() string
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

// NamedTenancyOverride is a TenancyOverride that knows its tenant's name
// — the three tenancy override kinds.
type NamedTenancyOverride interface {
	TenancyOverride
	// GetTenantName is the tenant's metadata name, or its OCID when it
	// has none.
	GetTenantName() string
}

// RealmedID is implemented by OCI resources whose full OCID is
// constructed from a realm and a region plus a stored name suffix
// — DedicatedAICluster and ImportedModel today. Distinct from
// NamedItem.GetName (which returns just the suffix) and used by the
// TUI's CopyItemName action to produce the full OCID.
type RealmedID interface {
	OCID(realm, region string) string
}

// RealmedTenancyID is implemented by OCI resources whose owning
// tenancy OCID is constructed from a realm plus a stored tenancy-id
// suffix — DedicatedAICluster and ImportedModel today. Distinct
// from TenancyOverride.GetTenantID (which takes no realm and is
// implemented by file-backed override types whose stored TenantID
// is already the full identifier).
type RealmedTenancyID interface {
	TenancyOCID(realm string) string
}

// TenantOwned is a resource a tenancy owns, named by its tenancy-id
// label — DedicatedAICluster, ImportedModel, GPUWorkload and
// InferenceService.
type TenantOwned interface {
	RealmedTenancyID
	// RawTenantID is the tenancy-id as labelled ("" or UnknownTenancy
	// when there is none).
	RawTenantID() string
	// OwnerTenant is the tenant the tenancy-id resolves to; nil when it
	// doesn't.
	OwnerTenant() *Tenant
}

// UnknownTenancy is the tenancy-id of a DAC or imported model whose CR
// has no tenancy-id label: an orphan, not a tenancy.
const UnknownTenancy = "UNKNOWN_TENANCY"

// tenancyOCID is the tenancy OCID for a stored tenancy-id suffix, or ""
// when there is none: no label (an on-demand or non-serving pod), or the
// orphan placeholder, which no real OCID ends in.
func tenancyOCID(realm, tenantID string) string {
	if tenantID == "" || tenantID == UnknownTenancy {
		return ""
	}
	return fmt.Sprintf("ocid1.tenancy.%s..%s", realm, tenantID)
}

// DefinitionOverride represents a definition override with regions and value.
type DefinitionOverride interface {
	NamedFilterable
	GetRegions() []string
	GetValue() string
}

// Faulty marks an item that can be checked for faults.
type Faulty interface {
	IsFaulty() bool
}

// The tenant-owned resources and the tenancy overrides.
var (
	_ TenantOwned          = DedicatedAICluster{}
	_ TenantOwned          = ImportedModel{}
	_ TenantOwned          = GPUWorkload{}
	_ TenantOwned          = InferenceService{}
	_ NamedTenancyOverride = LimitTenancyOverride{}
	_ NamedTenancyOverride = ConsolePropertyTenancyOverride{}
	_ NamedTenancyOverride = PropertyTenancyOverride{}
)
