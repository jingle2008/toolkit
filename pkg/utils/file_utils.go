package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SafeReadFile securely reads a file from disk, enforcing:
// - Path cleaning (removes ../ etc.)
// - Absolute path resolution
// - Ensures the file is within the trusted baseDir
// - Only allows files with extensions in allowExt (e.g. {".json":{}, ".yaml":{}})
// Returns file contents or error.
func SafeReadFile(path string, baseDir string, allowExt map[string]struct{}) ([]byte, error) {
	clean := filepath.Clean(path)

	absTarget, err := filepath.Abs(clean)
	if err != nil {
		return nil, err
	}
	absBase, err := filepath.Abs(filepath.Clean(baseDir))
	if err != nil {
		return nil, err
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
