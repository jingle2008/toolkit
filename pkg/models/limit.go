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

// GetFilterableFields returns filterable fields for the limit definition.
func (c LimitDefinition) GetFilterableFields() []string {
	return []string{c.Name, c.Description}
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

// GetFilterableFields returns filterable fields for the limit regional override.
func (o LimitRegionalOverride) GetFilterableFields() []string {
	return append(o.Regions, o.Name)
}

// LimitTenancyOverride represents a tenancy override for a limit.
type LimitTenancyOverride struct {
	LimitRegionalOverride
	TenantID string `json:"tenant_id"`
}

// GetTenantID returns the tenant ID of the limit tenancy override.
func (o LimitTenancyOverride) GetTenantID() string {
	return o.TenantID
}

// GetFilterableFields returns filterable fields for the limit tenancy override.
func (o LimitTenancyOverride) GetFilterableFields() []string {
	return append(o.Regions, o.Name, o.TenantID)
}
