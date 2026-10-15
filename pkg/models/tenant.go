package models

import (
	"fmt"
	"slices"
)

// Tenant represents a tenant entity.
type Tenant struct {
	Name       string   `json:"name"`
	IDs        []string `json:"ids"`
	IsInternal bool     `json:"is_internal"`
	Note       string   `json:"note,omitempty"`
	// SharedWith names the other tenants that claim one of this
	// tenant's OCIDs — typically the same tenancy filed under two
	// override directories (e.g. "OABCS1" and "oabcs1"). Populated by
	// the config loader; a non-empty value marks the tenant faulty.
	SharedWith []string `json:"shared_with,omitempty"`
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

// FilterableFields returns filterable fields for the tenant.
func (t Tenant) FilterableFields() []string {
	var fields []string
	if len(t.IDs) > 1 {
		fields = slices.Clone(t.IDs[1:])
	}
	fields = append(fields, t.SharedWith...)
	return append(fields, t.GetTenantID(), t.Name, fmt.Sprint(t.IsInternal), t.Note)
}

// IsFaulty returns true if the tenant has multiple IDs or shares an ID
// with another tenant.
func (t Tenant) IsFaulty() bool {
	return len(t.IDs) > 1 || len(t.SharedWith) > 0
}
