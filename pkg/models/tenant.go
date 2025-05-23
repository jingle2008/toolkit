package models

import "fmt"

type Tenant struct {
	Name                     string   `json:"name"`
	Ids                      []string `json:"ids"`
	LimitOverrides           int      `json:"limit_overrides"`
	ConsolePropertyOverrides int      `json:"console_property_overrides"`
	PropertyOverrides        int      `json:"property_overrides"`
}

func (t Tenant) GetName() string {
	return t.Name
}

func (t Tenant) GetTenantId() string {
	var tenantId string
	if len(t.Ids) > 1 {
		tenantId = fmt.Sprintf("%s (+%d)", t.Ids[0], len(t.Ids)-1)
	} else {
		tenantId = t.Ids[0]
	}
	return tenantId
}

func (t Tenant) GetOverrides() string {
	return fmt.Sprintf("%d/%d/%d",
		t.LimitOverrides, t.ConsolePropertyOverrides, t.PropertyOverrides)
}

func (t Tenant) GetFilterableFields() []string {
	fields := t.Ids[1:]
	return append(fields, t.GetTenantId(), t.Name)
}
