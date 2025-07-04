package models

import (
	"strings"
)

// TenantMetadata represents a single tenant entry in the external metadata file.
type TenantMetadata struct {
	Name       *string `json:"name,omitempty" yaml:"name,omitempty"`
	ID         string  `json:"id" yaml:"id"`
	IsInternal *bool   `json:"is_internal,omitempty" yaml:"is_internal,omitempty"`
	Note       *string `json:"note,omitempty" yaml:"note,omitempty"`
}

// Metadata is the top-level structure for metadata.json/yaml.
type Metadata struct {
	Tenants []TenantMetadata `json:"tenants" yaml:"tenants"`
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
