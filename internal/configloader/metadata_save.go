package configloader

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"sigs.k8s.io/yaml"

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
		data, err = json.MarshalIndent(m, "", "  ")
	case ".yaml", ".yml":
		data, err = yaml.Marshal(m)
	default:
		return fmt.Errorf("unsupported metadata file extension: %s", ext)
	}
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}
	if mkErr := os.MkdirAll(filepath.Dir(path), 0o755); mkErr != nil {
		return fmt.Errorf("failed to create metadata dir: %w", mkErr)
	}
	if wErr := os.WriteFile(path, data, 0o600); wErr != nil {
		return fmt.Errorf("failed to write metadata file: %w", wErr)
	}
	return nil
}

// UpsertTenant merges entry into m: if a tenant with the same ID
// already exists it is replaced in place, otherwise entry is appended.
func UpsertTenant(m *models.Metadata, entry models.TenantMetadata) {
	for i := range m.Tenants {
		if m.Tenants[i].ID == entry.ID {
			m.Tenants[i] = entry
			return
		}
	}
	m.Tenants = append(m.Tenants, entry)
}
