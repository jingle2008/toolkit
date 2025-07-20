package models

// PropertyDefinition represents a property definition.
type PropertyDefinition struct {
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Type         string   `json:"type"`
	Options      []string `json:"options"`
	DefaultValue string   `json:"default_value"`
}

// GetName returns the name of the property definition.
func (c PropertyDefinition) GetName() string {
	return c.Name
}

// GetDescription returns the description of the property definition.
func (c PropertyDefinition) GetDescription() string {
	return c.Description
}

// GetValue returns the default value of the property definition.
func (c PropertyDefinition) GetValue() string {
	return c.DefaultValue
}

// GetFilterableFields returns filterable fields for the property definition.
func (c PropertyDefinition) GetFilterableFields() []string {
	return []string{c.Name, c.Description}
}

// IsFaulty returns false by default for PropertyDefinition.
func (c PropertyDefinition) IsFaulty() bool {
	return false
}

// PropertyRegionalOverride represents a regional override for a property.
type PropertyRegionalOverride struct {
	Realms  []string `json:"realms"`
	Name    string   `json:"name"`
	Regions []string `json:"regions"`
	Group   string   `json:"group"`
	Values  []struct {
		Value string `json:"value"`
	} `json:"values"`
}

// GetName returns the name of the property regional override.
func (o PropertyRegionalOverride) GetName() string {
	return o.Name
}

// GetRegions returns the regions of the property regional override.
func (o PropertyRegionalOverride) GetRegions() []string {
	return o.Regions
}

// GetValue returns the value of the property regional override.
func (o PropertyRegionalOverride) GetValue() string {
	return o.Values[0].Value
}

// GetFilterableFields returns filterable fields for the property regional override.
func (o PropertyRegionalOverride) GetFilterableFields() []string {
	return append(o.Regions, o.Name)
}

// IsFaulty returns false by default for PropertyRegionalOverride.
func (o PropertyRegionalOverride) IsFaulty() bool {
	return false
}

// PropertyTenancyOverride represents a tenancy override for a property.
type PropertyTenancyOverride struct {
	Tag string `json:"tag"`
	PropertyRegionalOverride
}

// GetTenantID returns the tenant tag of the property tenancy override.
func (o PropertyTenancyOverride) GetTenantID() string {
	return o.Tag
}

// GetFilterableFields returns filterable fields for the property tenancy override.
func (o PropertyTenancyOverride) GetFilterableFields() []string {
	return append(o.Regions, o.Name, o.Tag)
}
