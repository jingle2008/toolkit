package models

import "slices"

// LimitDefinition represents a limit definition for a service.
type LimitDefinition struct {
	Name                 string `json:"name"`
	Description          string `json:"description"`
	Type                 string `json:"type"`
	Scope                string `json:"scope"`
	IsReleasedToCustomer bool   `json:"is_released_to_customer"`
	DefaultMin           string `json:"default_min"`
	DefaultMax           string `json:"default_max"`
	Service              string `json:"service"`
	PublicName           string `json:"public_name"`
	IsStaged             bool   `json:"is_staged"`
	IsQuota              bool   `json:"is_quota"`
	UsageSource          string `json:"usage_source"`
	// Splat throttle definitions (limits_definitions/splat_limit_definitions.json)
	// carry these instead of scope/min/max: the API operation throttled,
	// the dimension it is throttled on (user/tenant), and a default rate
	// such as "50RPS400RPM".
	OperationID       string `json:"operation_id,omitempty"`
	ThrottleDimension string `json:"throttle_dimension,omitempty"`
	DefaultValue      string `json:"default_value,omitempty"`
}

// GetName returns the name of the limit definition.
func (c LimitDefinition) GetName() string {
	return c.Name
}

// GetDescription returns the description of the limit definition.
func (c LimitDefinition) GetDescription() string {
	return c.Description
}

// FilterableFields returns filterable fields for the limit definition.
func (c LimitDefinition) FilterableFields() []string {
	fields := []string{c.Name, c.Description}
	if c.OperationID != "" {
		fields = append(fields, c.OperationID)
	}
	return fields
}

// IsFaulty returns false by default for LimitDefinition.
func (c LimitDefinition) IsFaulty() bool {
	return false
}

// LimitRange represents a min/max range for a limit override.
type LimitRange struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

// LimitRegionalOverride represents a regional override for a limit.
type LimitRegionalOverride struct {
	Realms  []string     `json:"realms"`
	Group   string       `json:"group"`
	Name    string       `json:"name"`
	Regions []string     `json:"regions"`
	Values  []LimitRange `json:"values"`
}

// GetName returns the name of the limit tenancy override.
func (o LimitRegionalOverride) GetName() string {
	return o.Name
}

// FilterableFields returns filterable fields for the limit regional override.
func (o LimitRegionalOverride) FilterableFields() []string {
	return slices.Concat(o.Regions, []string{o.Name})
}

// IsFaulty returns false by default for LimitRegionalOverride.
func (o LimitRegionalOverride) IsFaulty() bool {
	return false
}

// LimitTenancyOverride represents a tenancy override for a limit.
//
// TenantID is the record's tenancy OCID, which identifies its tenant.
// TenantName is that tenant's name (its metadata name, or the OCID when
// it has none) — the map key in Dataset.LimitTenancyOverrideMap — and is
// stamped by the configloader after unmarshal; the files don't declare
// it. The directory a record sits in plays no part.
type LimitTenancyOverride struct {
	LimitRegionalOverride
	TenantName string `json:"tenant"`
	TenantID   string `json:"tenant_id"`
}

// GetTenantID returns the tenant ID of the limit tenancy override.
func (o LimitTenancyOverride) GetTenantID() string {
	return o.TenantID
}

// SetTenantName stamps the owning tenant's name onto the override.
// Called by the configloader after unmarshal so consumers can read
// the grouping identifier from the struct instead of carrying the
// map key alongside.
func (o *LimitTenancyOverride) SetTenantName(name string) {
	o.TenantName = name
}

// FilterableFields returns filterable fields for the limit tenancy override.
func (o LimitTenancyOverride) FilterableFields() []string {
	return slices.Concat(o.Regions, []string{o.Name, o.TenantName, o.TenantID})
}

// GetTenantName is the tenant's metadata name, or its OCID when it has
// none.
func (o LimitTenancyOverride) GetTenantName() string { return o.TenantName }
