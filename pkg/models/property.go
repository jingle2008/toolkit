package models

type PropertyDefinition struct {
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Type         string   `json:"type"`
	Options      []string `json:"options"`
	DefaultValue string   `json:"default_value"`
}

func (c PropertyDefinition) GetName() string {
	return c.Name
}

func (c PropertyDefinition) GetDescription() string {
	return c.Description
}

func (c PropertyDefinition) GetValue() string {
	return c.DefaultValue
}

func (c PropertyDefinition) GetFilterableFields() []string {
	return []string{c.Name, c.Description}
}

type PropertyRegionalOverride struct {
	Realms  []string `json:"realms"`
	Name    string   `json:"name"`
	Regions []string `json:"regions"`
	Group   string   `json:"group"`
	Values  []struct {
		Value string `json:"value"`
	} `json:"values"`
}

type PropertyTenancyOverride struct {
	Tag string `json:"tag"`
	PropertyRegionalOverride
}

func (o PropertyRegionalOverride) GetName() string {
	return o.Name
}

func (o PropertyRegionalOverride) GetRegions() []string {
	return o.Regions
}

func (o PropertyRegionalOverride) GetValue() string {
	return o.Values[0].Value
}

func (o PropertyTenancyOverride) GetTenantId() string {
	return o.Tag
}

func (o PropertyRegionalOverride) GetFilterableFields() []string {
	results := o.Regions[:]
	return append(results, o.Name)
}
