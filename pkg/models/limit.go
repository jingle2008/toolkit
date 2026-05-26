package models

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
	return []string{c.Name, c.Description}
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
	return append(o.Regions, o.Name)
}

// IsFaulty returns false by default for LimitRegionalOverride.
func (o LimitRegionalOverride) IsFaulty() bool {
	return false
}

// LimitTenancyOverride represents a tenancy override for a limit.
//
// TenantName carries the originating tenant directory name (the short
// human-readable identifier used as the map key in
// Dataset.LimitTenancyOverrideMap) and is populated by the
// configloader after yaml unmarshal — yaml files don't usually
// declare it because the path-grouping is conventional. TenantID is
// the per-record OCID from yaml. The two are distinct identifiers
// for the same tenant: name groups records together; id is the OCI
// identifier on this specific record.
type LimitTenancyOverride struct {
	LimitRegionalOverride
	TenantName string `json:"tenant"`
	TenantID   string `json:"tenant_id"`
}

// GetTenantID returns the tenant ID of the limit tenancy override.
func (o LimitTenancyOverride) GetTenantID() string {
	return o.TenantID
}

// SetTenantName stamps the tenant short name onto the override.
// Called by the configloader after unmarshal so consumers can read
// the grouping identifier from the struct instead of carrying the
// map key alongside.
func (o *LimitTenancyOverride) SetTenantName(name string) {
	o.TenantName = name
}

// FilterableFields returns filterable fields for the limit tenancy override.
func (o LimitTenancyOverride) FilterableFields() []string {
	return append(o.Regions, o.Name, o.TenantName, o.TenantID)
}
