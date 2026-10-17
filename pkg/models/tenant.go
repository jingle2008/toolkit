package models

import (
	"fmt"
)

// Tenant represents a tenant entity: one tenancy OCID and its name. The
// name comes from tenant metadata, the single source of truth for names;
// a tenancy without a metadata name is named by its OCID. Names and IDs
// are both unique, so the two map one to one.
type Tenant struct {
	Name       string `json:"name"`
	ID         string `json:"id"`
	IsInternal bool   `json:"is_internal"`
	Note       string `json:"note,omitempty"`
}

// GetName returns the name of the tenant.
func (t Tenant) GetName() string {
	return t.Name
}

// GetTenantID returns the tenant's OCID.
func (t Tenant) GetTenantID() string {
	return t.ID
}

// Named reports whether the tenant has a metadata name, rather than
// being named by its OCID.
func (t Tenant) Named() bool {
	return t.Name != t.ID
}

// FilterableFields returns filterable fields for the tenant.
func (t Tenant) FilterableFields() []string {
	return []string{t.ID, t.Name, fmt.Sprint(t.IsInternal), t.Note}
}

// IsFaulty is always false: a tenant is one OCID with one name, so it
// has no state of its own to be wrong. (It satisfies Faulty so the
// Tenant list supports the faulty filter like every other category.)
func (t Tenant) IsFaulty() bool {
	return false
}
