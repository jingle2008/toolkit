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

// FilterableFields returns filterable fields for the property definition.
func (c PropertyDefinition) FilterableFields() []string {
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
	if len(o.Values) == 0 {
		return ""
	}
	return o.Values[0].Value
}

// FilterableFields returns filterable fields for the property regional override.
func (o PropertyRegionalOverride) FilterableFields() []string {
	return append(o.Regions, o.Name)
}

// IsFaulty returns false by default for PropertyRegionalOverride.
func (o PropertyRegionalOverride) IsFaulty() bool {
	return false
}

// PropertyTenancyOverride represents a tenancy override for a property.
//
// TenantName is the originating tenant directory name (populated by
// the configloader, same convention as LimitTenancyOverride).
// TenantID is the per-record tenant identifier; the JSON key is kept
// as "tag" for back-compat with the on-disk record format.
type PropertyTenancyOverride struct {
	TenantName string `json:"tenant"`
	TenantID   string `json:"tag"`
	PropertyRegionalOverride
}

// GetTenantID returns the tenant ID of the property tenancy override.
func (o PropertyTenancyOverride) GetTenantID() string {
	return o.TenantID
}

// SetTenantName stamps the tenant short name onto the override.
// See LimitTenancyOverride.SetTenantName.
func (o *PropertyTenancyOverride) SetTenantName(name string) {
	o.TenantName = name
}

// FilterableFields returns filterable fields for the property tenancy override.
func (o PropertyTenancyOverride) FilterableFields() []string {
	return append(o.Regions, o.Name, o.TenantName, o.TenantID)
}
