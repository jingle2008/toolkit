package configloader

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"

	"golang.org/x/sync/errgroup"
)

// TenancyOverrideFingerprint summarizes the files LoadTenancyOverrideGroup
// reads for realm — every entry's path, size and modification time under
// the three tenancy-override realm directories — so a caller can tell a
// cached group is stale with a stat walk (~85ms on oc1) instead of
// re-parsing every file (~350ms). A missing directory is recorded as
// missing. The directories are walked concurrently: on oc1 that is
// thousands of entries.
func TenancyOverrideFingerprint(repoPath, realm string) (string, error) {
	limitsRoot := getLimitsRoot(repoPath)
	keys := []string{limitsKey, consolePropertiesKey, propertiesKey}
	sums := make([][]byte, len(keys))
	var g errgroup.Group
	for i, key := range keys {
		dir := filepath.Join(limitsRoot, key+tenancyOverridesKey, regionalValuesDir, realm)
		g.Go(func() error {
			sum, err := fingerprintDir(dir)
			if err != nil {
				return fmt.Errorf("fingerprint %s: %w", dir, err)
			}
			sums[i] = sum
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return "", err
	}
	h := sha256.New()
	for _, sum := range sums {
		h.Write(sum)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// fingerprintDir hashes the path, size and modification time of every
// entry under dir.
func fingerprintDir(dir string) ([]byte, error) {
	h := sha256.New()
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if path == dir && errors.Is(err, fs.ErrNotExist) {
				_, _ = fmt.Fprintf(h, "%s\x00missing\n", path)
				return nil
			}
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(h, "%s\x00%d\x00%d\n", path, info.Size(), info.ModTime().UnixNano())
		return nil
	})
	return h.Sum(nil), err
}
