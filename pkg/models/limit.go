package models

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

func (c LimitDefinition) GetName() string {
	return c.Name
}

func (c LimitDefinition) GetDescription() string {
	return c.Description
}

func (c LimitDefinition) GetFilterableFields() []string {
	return []string{c.Name, c.Description}
}

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

func (o LimitTenancyOverride) GetName() string {
	return o.Name
}

func (o LimitTenancyOverride) GetTenantId() string {
	return o.TenantID
}

func (o LimitTenancyOverride) GetFilterableFields() []string {
	results := o.Regions[:]
	return append(results, o.Name)
}
