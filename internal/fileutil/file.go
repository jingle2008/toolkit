/*
Package fs provides utility functions for secure file operations,
including path validation and extension whitelisting.
*/
package fs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

/*
SafeReadFile reads a file from disk securely.

It enforces:
- Path cleaning (removes ../ etc.)
- Absolute path resolution
- Ensures the file is within the trusted baseDir
- Only allows files with extensions in allowExt (e.g. {".json":{}, ".yaml":{}})

Parameters:
  - path: the file path to read
  - baseDir: the trusted base directory
  - allowExt: a set of allowed file extensions

Returns:
  - file contents as []byte
  - error if the file cannot be read or does not meet security checks
*/
func SafeReadFile(path string, baseDir string, allowExt map[string]struct{}) ([]byte, error) {
	clean := filepath.Clean(path)

	absTarget, err := filepath.Abs(clean)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	absBase, err := filepath.Abs(filepath.Clean(baseDir))
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Ensure absTarget is within absBase
	if !strings.HasPrefix(absTarget, absBase+string(os.PathSeparator)) && absTarget != absBase {
		return nil, fmt.Errorf("access outside trusted dir %s", absBase)
	}

	ext := strings.ToLower(filepath.Ext(absTarget))
	if _, ok := allowExt[ext]; !ok {
		return nil, fmt.Errorf("extension %s not permitted", ext)
	}

	return os.ReadFile(absTarget) // #nosec G304 -- absTarget validated above
}
