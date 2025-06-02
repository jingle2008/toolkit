package models

// ConsolePropertyDefinition represents a console property definition.
type ConsolePropertyDefinition struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Value       string `json:"value"`
}

// GetName returns the name of the console property definition.
func (c ConsolePropertyDefinition) GetName() string {
	return c.Name
}

// GetDescription returns the description of the console property definition.
func (c ConsolePropertyDefinition) GetDescription() string {
	return c.Description
}

// GetValue returns the value of the console property definition.
func (c ConsolePropertyDefinition) GetValue() string {
	return c.Value
}

// GetFilterableFields returns filterable fields for the console property definition.
func (c ConsolePropertyDefinition) GetFilterableFields() []string {
	return []string{c.Name, c.Description}
}

// ConsolePropertyRegionalOverride represents a regional override for a console property.
type ConsolePropertyRegionalOverride struct {
	Realms  []string `json:"realms"`
	Name    string   `json:"name"`
	Regions []string `json:"regions"`
	Service string   `json:"service"`
	Values  []struct {
		Value string `json:"value"`
	} `json:"values"`
}

// ConsolePropertyTenancyOverride represents a tenancy override for a console property.
type ConsolePropertyTenancyOverride struct {
	TenantID string `json:"tenant_id"`
	ConsolePropertyRegionalOverride
}

// GetName returns the name of the console property regional override.
func (o ConsolePropertyRegionalOverride) GetName() string {
	return o.Name
}

// GetRegions returns the regions of the console property regional override.
func (o ConsolePropertyRegionalOverride) GetRegions() []string {
	return o.Regions
}

// GetValue returns the value of the console property regional override.
func (o ConsolePropertyRegionalOverride) GetValue() string {
	return o.Values[0].Value
}

// GetTenantID returns the tenant ID of the console property tenancy override.
func (o ConsolePropertyTenancyOverride) GetTenantID() string {
	return o.TenantID
}

// GetFilterableFields returns filterable fields for the console property regional override.
func (o ConsolePropertyRegionalOverride) GetFilterableFields() []string {
	results := o.Regions[:]
	return append(results, o.Name)
}
