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
	// Status is the tenancy's account state as the account admin service
	// reports it (ACTIVE, DELETING, TERMINATED, ALWAYS_FREE_ONLY, ...),
	// recorded in metadata by `toolkit tenant sync`; empty when unknown.
	Status string `json:"status,omitempty"`
}

// Tenancy account states that mark a tenancy as going or gone.
const (
	TenantStatusDeleting   = "DELETING"
	TenantStatusTerminated = "TERMINATED"
)

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
	return []string{t.ID, t.Name, fmt.Sprint(t.IsInternal), t.Note, t.Status}
}

// IsFaulty reports whether the tenancy is being deleted or is terminated:
// whatever the repo still holds for it is left over.
func (t Tenant) IsFaulty() bool {
	return t.Status == TenantStatusDeleting || t.Status == TenantStatusTerminated
}
