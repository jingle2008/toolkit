package models

import (
	"errors"
	"fmt"
	"strings"
)

// TenantMetadata represents a single tenant entry in the external metadata file.
type TenantMetadata struct {
	Name       *string `json:"name,omitempty"`
	ID         string  `json:"id"`
	IsInternal *bool   `json:"is_internal,omitempty"`
	Note       *string `json:"note,omitempty"`
	// Status is the tenancy's account state, written by `toolkit tenant sync`. It
	// is machine-owned: an upsert without one keeps the existing value.
	Status string `json:"status,omitempty"`
}

// Metadata is the top-level structure for metadata.json/yaml.
type Metadata struct {
	Tenants []TenantMetadata `json:"tenants"`
}

// Validate enforces that tenant IDs are unique and that tenant names are
// unique within a realm, so a name maps to exactly one tenancy wherever
// tenants are listed (always per realm). An entry without a name is
// allowed (its tenant is named by its OCID).
func (m *Metadata) Validate() error {
	ids := make(map[string]bool, len(m.Tenants))
	names := make(map[[2]string]string, len(m.Tenants))
	var errs []error
	for _, t := range m.Tenants {
		if ids[t.ID] {
			errs = append(errs, fmt.Errorf("tenant id %s is listed more than once", t.ID))
		}
		ids[t.ID] = true
		if t.Name == nil || *t.Name == "" {
			continue
		}
		key := [2]string{TenancyRealm(t.ID), *t.Name}
		if other, ok := names[key]; ok && other != t.ID {
			errs = append(errs, fmt.Errorf("tenant name %q is used by both %s and %s", *t.Name, other, t.ID))
			continue
		}
		names[key] = t.ID
	}
	return errors.Join(errs...)
}

// TenancyRealm returns the realm segment of a tenancy OCID
// (ocid1.tenancy.<realm>..<id>), or "" when id has none.
func TenancyRealm(id string) string {
	parts := strings.Split(id, ".")
	if len(parts) >= 3 {
		return parts[2]
	}
	return ""
}

// GetTenants returns all tenants for a given realm.
func (m *Metadata) GetTenants(realm string) []TenantMetadata {
	var out []TenantMetadata
	for _, t := range m.Tenants {
		if TenancyRealm(t.ID) == realm {
			out = append(out, t)
		}
	}
	return out
}
