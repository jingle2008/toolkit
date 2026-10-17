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
}

// Metadata is the top-level structure for metadata.json/yaml.
type Metadata struct {
	Tenants []TenantMetadata `json:"tenants"`
}

// Validate enforces that tenant IDs and tenant names are each unique, so
// a name maps to exactly one tenancy. An entry without a name is allowed
// (its tenant is named by its OCID).
func (m *Metadata) Validate() error {
	ids := make(map[string]bool, len(m.Tenants))
	names := make(map[string]string, len(m.Tenants))
	var errs []error
	for _, t := range m.Tenants {
		if ids[t.ID] {
			errs = append(errs, fmt.Errorf("tenant id %s is listed more than once", t.ID))
		}
		ids[t.ID] = true
		if t.Name == nil || *t.Name == "" {
			continue
		}
		if other, ok := names[*t.Name]; ok && other != t.ID {
			errs = append(errs, fmt.Errorf("tenant name %q is used by both %s and %s", *t.Name, other, t.ID))
			continue
		}
		names[*t.Name] = t.ID
	}
	return errors.Join(errs...)
}

// GetTenants returns all tenants for a given realm.
func (m *Metadata) GetTenants(realm string) []TenantMetadata {
	var out []TenantMetadata
	for _, t := range m.Tenants {
		parts := strings.Split(t.ID, ".")
		if len(parts) >= 3 && parts[2] == realm {
			out = append(out, t)
		}
	}
	return out
}
