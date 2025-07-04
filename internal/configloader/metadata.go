package configloader

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jingle2008/toolkit/internal/encoding/jsonutil"
	"github.com/jingle2008/toolkit/pkg/models"
	"gopkg.in/yaml.v3"
)

// LoadMetadata loads tenants from a metadata file (JSON or YAML) and returns a MetadataFile.
func LoadMetadata(path string) (*models.Metadata, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".json":
		return jsonutil.LoadFile[models.Metadata](path)
	case ".yaml", ".yml":
		return loadYAML(path)
	default:
		return nil, fmt.Errorf("unsupported metadata file extension: %s", ext)
	}
}

func loadYAML(path string) (*models.Metadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read YAML file: %w", err)
	}

	var out models.Metadata
	err = yaml.Unmarshal(data, &out)
	return &out, err
}
