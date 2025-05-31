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

// LimitTenancyOverride represents a tenancy override for a limit.
type LimitTenancyOverride struct {
	Realms   []string `json:"realms"`
	Name     string   `json:"name"`
	Regions  []string `json:"regions"`
	Group    string   `json:"group"`
	TenantID string   `json:"tenant_id"`
	Values   []struct {
		Min int `json:"min"`
		Max int `json:"max"`
	} `json:"values"`
}

// GetName returns the name of the limit tenancy override.
func (o LimitTenancyOverride) GetName() string {
	return o.Name
}

// GetTenantId returns the tenant ID of the limit tenancy override.
func (o LimitTenancyOverride) GetTenantID() string {
	return o.TenantID
}

// GetFilterableFields returns filterable fields for the limit tenancy override.
func (o LimitTenancyOverride) GetFilterableFields() []string {
	results := o.Regions[:]
	return append(results, o.Name)
}
