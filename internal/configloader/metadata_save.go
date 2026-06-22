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
		data, err = json.MarshalIndent(m, "", "    ")
	case ".yaml", ".yml":
		data, err = yaml.Marshal(m)
	default:
		return fmt.Errorf("unsupported metadata file extension: %s", ext)
	}
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}
	if wErr := writeFileAtomic(path, data, 0o600); wErr != nil {
		return fmt.Errorf("failed to write metadata file: %w", wErr)
	}
	return nil
}

// writeFileAtomic writes data to path atomically: it writes a temp file in the
// same directory, fsyncs it, then renames it over path. An interrupted or
// failed write therefore never truncates or corrupts an existing file. Parent
// directories are created if missing.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("failed to create dir: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	// Best-effort cleanup; a no-op once the rename below succeeds.
	defer func() { _ = os.Remove(tmpPath) }()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("failed to write temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("failed to sync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}
	if err := os.Chmod(tmpPath, perm); err != nil {
		return fmt.Errorf("failed to chmod temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("failed to rename temp file: %w", err)
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
