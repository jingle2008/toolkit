package configloader

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"sigs.k8s.io/yaml"

	"github.com/jingle2008/toolkit/internal/fileutil"
	"github.com/jingle2008/toolkit/pkg/models"
)

// SaveMetadata writes m to path, choosing JSON or YAML by the file
// extension. Parent directories are created if missing. Mirrors
// LoadMetadata's extension contract.
func SaveMetadata(path string, m *models.Metadata) error {
	ext := strings.ToLower(filepath.Ext(path))
	var (
		data []byte
		err  error
	)
	switch ext {
	case ".json":
		data, err = json.MarshalIndent(m, "", "    ")
	case ".yaml", ".yml":
		data, err = yaml.Marshal(m)
	default:
		return fmt.Errorf("unsupported metadata file extension: %s", ext)
	}
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}
	if wErr := fileutil.WriteFileAtomic(path, data, 0o600); wErr != nil {
		return fmt.Errorf("failed to write metadata file: %w", wErr)
	}
	return nil
}

// UpsertTenant merges entry into m: if a tenant with the same ID
// already exists it is replaced in place, otherwise entry is appended.
// It refuses, leaving m unchanged, a name another tenancy in the same realm
// already has: names map one to one to IDs within a realm. Status is
// machine-owned (`toolkit tenant sync` writes it), so an entry without one
// keeps the existing status; likewise an entry without an internal flag
// keeps the existing one.
//
// The name is stored trimmed, and the check compares trimmed names: one
// with a stray space ("acme ") would otherwise get past it and list as a
// second "acme". A name that is only whitespace is refused.
func UpsertTenant(m *models.Metadata, entry models.TenantMetadata) error {
	if err := trimTenantName(m, &entry); err != nil {
		return err
	}
	for i := range m.Tenants {
		if m.Tenants[i].ID == entry.ID {
			if entry.Status == "" {
				entry.Status = m.Tenants[i].Status
			}
			if entry.IsInternal == nil {
				entry.IsInternal = m.Tenants[i].IsInternal
			}
			m.Tenants[i] = entry
			return nil
		}
	}
	m.Tenants = append(m.Tenants, entry)
	return nil
}

// trimTenantName trims entry's name (if it has one) as UpsertTenant
// stores it, or says why it can't be stored: it is blank, or another
// tenancy in the realm has it.
func trimTenantName(m *models.Metadata, entry *models.TenantMetadata) error {
	if entry.Name == nil {
		return nil
	}
	name := strings.TrimSpace(*entry.Name)
	if name == "" && *entry.Name != "" {
		return errors.New("tenant name is blank")
	}
	realm := models.TenancyRealm(entry.ID)
	for _, t := range m.Tenants {
		if name != "" && t.ID != entry.ID && t.Name != nil && strings.TrimSpace(*t.Name) == name && models.TenancyRealm(t.ID) == realm {
			return fmt.Errorf("tenant name %q is already used by %s", name, t.ID)
		}
	}
	entry.Name = &name
	return nil
}
