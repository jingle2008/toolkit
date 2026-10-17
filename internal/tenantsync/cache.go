package tenantsync

import (
	"context"
	"errors"
	"os"
	"path/filepath"
)

// Cached serves lookups from Dir (<ocid>.json, raw oci-ops output) and
// saves every live fetch there, so a sync stopped by an expired session
// resumes where it left off. A cached response is as old as its file:
// point at a fresh directory for current statuses.
type Cached struct {
	Dir  string
	Next Fetcher
}

// Fetch implements Fetcher.
func (c Cached) Fetch(ctx context.Context, ocid string) ([]byte, error) {
	path := filepath.Join(c.Dir, ocid+".json")
	if raw, err := os.ReadFile(path); err == nil { //nolint:gosec // a file the sync wrote, named by the tenancy OCID
		if _, perr := ParseAccount(raw); perr == nil {
			return raw, nil
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	raw, err := c.Next.Fetch(ctx, ocid)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(c.Dir, 0o700); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return nil, err
	}
	return raw, nil
}
