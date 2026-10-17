package configloader

import (
	"fmt"
	"path/filepath"
	"strings"

	"sigs.k8s.io/yaml"

	"github.com/jingle2008/toolkit/internal/encoding/jsonutil"
	"github.com/jingle2008/toolkit/internal/fileutil"
	"github.com/jingle2008/toolkit/pkg/models"
)

// LoadMetadata loads tenants from a metadata file (JSON or YAML) and returns a MetadataFile.
func LoadMetadata(path string) (*models.Metadata, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".json":
		return validated(jsonutil.LoadFile[models.Metadata](path))
	case ".yaml", ".yml":
		return validated(loadYAML(path, ext))
	default:
		return nil, fmt.Errorf("unsupported metadata file extension: %s", ext)
	}
}

// validated rejects metadata whose tenant names or IDs aren't unique.
func validated(m *models.Metadata, err error) (*models.Metadata, error) {
	if err != nil {
		return nil, err
	}
	if err := m.Validate(); err != nil {
		return nil, fmt.Errorf("invalid tenant metadata: %w", err)
	}
	return m, nil
}

func loadYAML(path, ext string) (*models.Metadata, error) {
	allowedExt := map[string]struct{}{ext: {}}
	baseDir := filepath.Dir(path)
	data, err := fileutil.SafeReadFile(path, baseDir, allowedExt)
	if err != nil {
		return nil, fmt.Errorf("failed to read YAML file: %w", err)
	}

	var out models.Metadata
	err = yaml.Unmarshal(data, &out)
	return &out, err
}
