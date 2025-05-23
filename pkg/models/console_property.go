package models

type ConsolePropertyDefinition struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Value       string `json:"value"`
}

func (c ConsolePropertyDefinition) GetName() string {
	return c.Name
}

func (c ConsolePropertyDefinition) GetDescription() string {
	return c.Description
}

func (c ConsolePropertyDefinition) GetValue() string {
	return c.Value
}

func (c ConsolePropertyDefinition) GetFilterableFields() []string {
	return []string{c.Name, c.Description}
}

type ConsolePropertyRegionalOverride struct {
	Realms  []string `json:"realms"`
	Name    string   `json:"name"`
	Regions []string `json:"regions"`
	Service string   `json:"service"`
	Values  []struct {
		Value string `json:"value"`
	} `json:"values"`
}

type ConsolePropertyTenancyOverride struct {
	TenantID string `json:"tenant_id"`
	ConsolePropertyRegionalOverride
}

func (o ConsolePropertyRegionalOverride) GetName() string {
	return o.Name
}

func (o ConsolePropertyRegionalOverride) GetRegions() []string {
	return o.Regions
}

func (o ConsolePropertyRegionalOverride) GetValue() string {
	return o.Values[0].Value
}

func (o ConsolePropertyTenancyOverride) GetTenantId() string {
	return o.TenantID
}

func (o ConsolePropertyRegionalOverride) GetFilterableFields() []string {
	results := o.Regions[:]
	return append(results, o.Name)
}
