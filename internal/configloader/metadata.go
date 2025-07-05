package configloader

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/jingle2008/toolkit/internal/encoding/jsonutil"
	fs "github.com/jingle2008/toolkit/internal/fileutil"
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
		return loadYAML(path, ext)
	default:
		return nil, fmt.Errorf("unsupported metadata file extension: %s", ext)
	}
}

func loadYAML(path, ext string) (*models.Metadata, error) {
	allowedExt := map[string]struct{}{ext: {}}
	baseDir := filepath.Dir(path)
	data, err := fs.SafeReadFile(path, baseDir, allowedExt)
	if err != nil {
		return nil, fmt.Errorf("failed to read YAML file: %w", err)
	}

	var out models.Metadata
	err = yaml.Unmarshal(data, &out)
	return &out, err
}
