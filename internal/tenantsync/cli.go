package tenantsync

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/jingle2008/toolkit/internal/configloader"
	"github.com/jingle2008/toolkit/pkg/models"
)

const (
	realm = "oc1"
	// region hosts the account admin service for oc1.
	region = "us-ashburn-1"
)

// newFetcherFn builds the live fetcher; a seam for tests.
var newFetcherFn = func() Fetcher { return OCIOps{Region: region} }

// Config is one run of the sync.
type Config struct {
	RepoPath     string // the repo whose oc1 tenancy overrides list the tenancies
	MetadataFile string // the tenant metadata file to update
	CacheDir     string // optional: reuse and save raw lookups here
	Parallel     int    // concurrent lookups
	Write        bool   // write the metadata file; otherwise a dry run
}

// Run syncs every oc1 tenancy, prints what changed (or would) to stdout,
// and writes the metadata file when cfg.Write is set.
func Run(ctx context.Context, cfg Config, stdout io.Writer) error {
	if cfg.RepoPath == "" {
		return errors.New("no repo path: set repo-path in the config file or pass --repo-path")
	}
	if cfg.MetadataFile == "" {
		return errors.New("no metadata file: set metadata-file in the config file or pass --metadata-file")
	}
	meta, err := loadMetadata(cfg.MetadataFile)
	if err != nil {
		return err
	}
	group, err := configloader.LoadTenancyOverrideGroup(ctx, cfg.RepoPath, realm, meta)
	if err != nil {
		return fmt.Errorf("load %s tenants: %w", realm, err)
	}
	ids := make([]string, len(group.Tenants))
	for i, t := range group.Tenants {
		ids[i] = t.ID
	}

	f := newFetcherFn()
	if cfg.CacheDir != "" {
		f = Cached{Dir: cfg.CacheDir, Next: f}
	}
	next, report, err := Sync(ctx, ids, meta, f, Options{Parallel: cfg.Parallel, Retries: 4, Backoff: 5 * time.Second})
	if err != nil {
		return err
	}
	printReport(stdout, report)

	if !cfg.Write {
		_, _ = fmt.Fprintln(stdout, "dry run: nothing written; rerun with --write to save")
		return nil
	}
	return writeMetadata(stdout, cfg.MetadataFile, next)
}

// writeMetadata backs up path, saves m there, and loads it back to
// confirm toolkit will accept it.
func writeMetadata(stdout io.Writer, path string, m *models.Metadata) error {
	backup, err := backupFile(path)
	if err != nil {
		return err
	}
	if err := configloader.SaveMetadata(path, m); err != nil {
		return err
	}
	if _, err := configloader.LoadMetadata(path); err != nil {
		return fmt.Errorf("written metadata doesn't load back (backup: %s): %w", backup, err)
	}
	_, _ = fmt.Fprintf(stdout, "wrote %s", path)
	if backup != "" {
		_, _ = fmt.Fprintf(stdout, " (backup: %s)", backup)
	}
	_, _ = fmt.Fprintln(stdout)
	return nil
}

// loadMetadata loads the metadata file; a missing one is empty, and the
// first write creates it.
func loadMetadata(path string) (*models.Metadata, error) {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return &models.Metadata{}, nil
	}
	return configloader.LoadMetadata(path)
}

// backupFile copies path next to itself with a timestamp suffix and
// returns the copy's path, or "" when path doesn't exist yet.
func backupFile(path string) (string, error) {
	data, err := os.ReadFile(path) //nolint:gosec // the metadata file being synced
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	backup := path + ".bak-" + time.Now().Format("20060102-150405")
	return backup, os.WriteFile(backup, data, 0o600) //nolint:gosec // next to the user's own metadata file
}

// printReport writes the counts, then every line that needs a look:
// conflicts, failures, and status changes from a known status (a first
// recorded status is only counted).
func printReport(w io.Writer, r Report) {
	_, _ = fmt.Fprintf(w, "looked up %d %s tenancies: %d named, %d status changes, %d internal flags set, %d name conflicts, %d internal conflicts, %d failed\n",
		r.Looked, realm, len(r.Named), len(r.StatusChanged), len(r.InternalSet), len(r.NameConflicts), len(r.InternalConflicts), len(r.Failed))
	section := func(title string, cs []Change, line func(Change) string) {
		if len(cs) == 0 {
			return
		}
		_, _ = fmt.Fprintln(w, title)
		for _, c := range cs {
			_, _ = fmt.Fprintln(w, "  "+line(c))
		}
	}
	var changed []Change
	for _, c := range r.StatusChanged {
		if c.From != "" {
			changed = append(changed, c)
		}
	}
	section("status changes:", changed, func(c Change) string { return fmt.Sprintf("%s: %s -> %s", c.Tenant, c.From, c.To) })
	section("name conflicts (yours kept):", r.NameConflicts, func(c Change) string {
		if c.From == "" {
			return fmt.Sprintf("%s: service name %s", c.ID, c.To)
		}
		return fmt.Sprintf("%s: yours %q, service %q", c.ID, c.From, c.To)
	})
	section("internal conflicts (yours kept):", r.InternalConflicts, func(c Change) string {
		return fmt.Sprintf("%s: yours %s, service %s", c.Tenant, c.From, c.To)
	})
	section("failed lookups (entries unchanged):", r.Failed, func(c Change) string { return fmt.Sprintf("%s: %s", c.ID, c.To) })
}
