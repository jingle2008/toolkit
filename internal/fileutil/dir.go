/*
Package fs provides utility functions for directory operations,
such as listing files with a specific extension.
*/
package fs

import (
	"context"
	"os"
	"path/filepath"

	"fmt"
)

/*
ListFiles returns absolute file paths under dirPath whose extension matches ext.

Parameters:
  - dirPath: the directory to search
  - ext: the file extension to match (e.g., ".json")

Returns:
  - []string: absolute file paths matching the extension
  - error: if the directory cannot be read
*/
func ListFiles(ctx context.Context, dirPath, ext string) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var out []string

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ext {
			out = append(out, filepath.Join(dirPath, e.Name()))
		}
	}
	return out, nil
}
