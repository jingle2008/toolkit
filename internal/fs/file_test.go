package fs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jingle2008/toolkit/internal/testutil"
)

func TestSafeReadFile(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	allowExt := map[string]struct{}{".txt": {}, ".json": {}}
	txtFile := filepath.Join(tmpDir, "a.txt")
	jsonFile := filepath.Join(tmpDir, "b.json")
	mdFile := filepath.Join(tmpDir, "c.md")

	_ = os.WriteFile(txtFile, []byte("foo"), 0o600)          // #nosec G306
	_ = os.WriteFile(jsonFile, []byte("{\"bar\":1}"), 0o600) // #nosec G306
	_ = os.WriteFile(mdFile, []byte("# md"), 0o600)          // #nosec G306

	// Allowed extension
	data, err := SafeReadFile(txtFile, tmpDir, allowExt)
	testutil.RequireNoError(t, err)
	testutil.Equal(t, []byte("foo"), data)

	// Disallowed extension
	_, err = SafeReadFile(mdFile, tmpDir, allowExt)
	testutil.RequireError(t, err)

	// File outside baseDir
	outsideFile := filepath.Join(os.TempDir(), "outside.txt")
	_ = os.WriteFile(outsideFile, []byte("bad"), 0o600) // #nosec G306
	_, err = SafeReadFile(outsideFile, tmpDir, allowExt)
	testutil.RequireError(t, err)

	// Clean up
	_ = os.Remove(outsideFile)
}

func TestSafeReadFile_Success(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "foo.json")
	err := os.WriteFile(path, []byte(`{"ok":true}`), 0o600) // #nosec G306
	testutil.RequireNoError(t, err)

	allow := map[string]struct{}{".json": {}}
	data, err := SafeReadFile(path, dir, allow)
	testutil.RequireNoError(t, err)
	testutil.Equal(t, `{"ok":true}`, string(data))
}

func TestSafeReadFile_DisallowedExt(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "foo.txt")
	err := os.WriteFile(path, []byte("bad"), 0o600) // #nosec G306
	testutil.RequireNoError(t, err)

	allow := map[string]struct{}{".json": {}}
	_, err = SafeReadFile(path, dir, allow)
	testutil.RequireError(t, err)
	testutil.Contains(t, err.Error(), "extension")
}

func TestSafeReadFile_DirTraversal(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	allow := map[string]struct{}{".json": {}}
	evil := filepath.Join(dir, "..", "evil.json")
	_, err := SafeReadFile(evil, dir, allow)
	testutil.RequireError(t, err)
	testutil.Contains(t, err.Error(), "access outside trusted dir")
}

func TestSafeReadFile_OutsideBaseDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	otherDir := t.TempDir()
	path := filepath.Join(otherDir, "foo.json")
	err := os.WriteFile(path, []byte("{}"), 0o600) // #nosec G306
	testutil.RequireNoError(t, err)

	allow := map[string]struct{}{".json": {}}
	_, err = SafeReadFile(path, dir, allow)
	testutil.RequireError(t, err)
	testutil.Contains(t, err.Error(), "access outside trusted dir")
}
