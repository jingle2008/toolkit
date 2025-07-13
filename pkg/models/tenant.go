package models

import "fmt"

// Tenant represents a tenant entity.
type Tenant struct {
	Name       string   `json:"name"`
	IDs        []string `json:"ids"`
	IsInternal bool     `json:"is_internal"`
	Note       string   `json:"note,omitempty"`
}

// GetName returns the name of the tenant.
func (t Tenant) GetName() string {
	return t.Name
}

// GetTenantID returns the tenant ID string.
func (t Tenant) GetTenantID() string {
	if len(t.IDs) > 1 {
		return fmt.Sprintf("%s (+%d)", t.IDs[0], len(t.IDs)-1)
	}
	if len(t.IDs) == 1 {
		return t.IDs[0]
	}
	return ""
}

// GetFilterableFields returns filterable fields for the tenant.
func (t Tenant) GetFilterableFields() []string {
	fields := t.IDs[1:]
	return append(fields, t.GetTenantID(), t.Name, fmt.Sprint(t.IsInternal), t.Note)
}

// IsFaulty returns true if the tenant has multiple IDs.
func (t Tenant) IsFaulty() bool {
	return len(t.IDs) > 1
}
