package models

import "fmt"

// Tenant represents a tenant entity.
type Tenant struct {
	Name                     string   `json:"name"`
	IDs                      []string `json:"ids"`
	LimitOverrides           int      `json:"limit_overrides"`
	ConsolePropertyOverrides int      `json:"console_property_overrides"`
	PropertyOverrides        int      `json:"property_overrides"`
}

// GetName returns the name of the tenant.
func (t Tenant) GetName() string {
	return t.Name
}

// // GetTenantID returns the tenant ID string.
func (t Tenant) GetTenantID() string {
	if len(t.IDs) > 1 {
		return fmt.Sprintf("%s (+%d)", t.IDs[0], len(t.IDs)-1)
	}
	if len(t.IDs) == 1 {
		return t.IDs[0]
	}
	return ""
}

// GetOverrides returns a string summarizing the tenant's overrides.
func (t Tenant) GetOverrides() string {
	return fmt.Sprintf("%d/%d/%d",
		t.LimitOverrides, t.ConsolePropertyOverrides, t.PropertyOverrides)
}

// GetFilterableFields returns filterable fields for the tenant.
func (t Tenant) GetFilterableFields() []string {
	fields := t.IDs[1:]
	return append(fields, t.GetTenantID(), t.Name)
}
