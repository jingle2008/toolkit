package utils

import (
	"os"
	"path/filepath"
)

// ListFiles returns absolute file paths under dirPath whose extension matches ext.
func ListFiles(dirPath, ext string) ([]string, error) {
	var out []string
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ext {
			out = append(out, filepath.Join(dirPath, e.Name()))
		}
	}
	return out, nil
}
