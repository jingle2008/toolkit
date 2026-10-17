package tenantsync

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"sigs.k8s.io/yaml"

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

// toolkitConfig is the part of toolkit's config file tenant-sync reads.
type toolkitConfig struct {
	RepoPath     string `json:"repo-path"`
	MetadataFile string `json:"metadata-file"`
}

// Main runs tenant-sync with args (without the program name) and returns
// the exit code.
func Main(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if err := run(ctx, args, stdout); err != nil {
		_, _ = fmt.Fprintln(stderr, "tenant-sync:", err)
		return 1
	}
	return 0
}

func run(ctx context.Context, args []string, stdout io.Writer) error {
	home, _ := os.UserHomeDir()
	fs := flag.NewFlagSet("tenant-sync", flag.ContinueOnError)
	cfgFile := fs.String("config", filepath.Join(home, ".config", "toolkit", "config.yaml"), "toolkit config file (for repo-path and metadata-file)")
	repo := fs.String("repo", "", "repo path (default: repo-path from the config file)")
	metaFile := fs.String("metadata", "", "tenant metadata file (default: metadata-file from the config file, else ~/.config/toolkit/metadata.yaml)")
	cacheDir := fs.String("cache-dir", "", "reuse and save raw lookups here, so an interrupted sync resumes; use a fresh directory for current statuses")
	parallel := fs.Int("parallel", 3, "concurrent lookups")
	write := fs.Bool("write", false, "write the metadata file (default: dry run)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if err := resolvePaths(*cfgFile, home, repo, metaFile); err != nil {
		return err
	}

	meta, err := loadMetadata(*metaFile)
	if err != nil {
		return err
	}
	group, err := configloader.LoadTenancyOverrideGroup(ctx, *repo, realm, meta)
	if err != nil {
		return fmt.Errorf("load %s tenants: %w", realm, err)
	}
	ids := make([]string, len(group.Tenants))
	for i, t := range group.Tenants {
		ids[i] = t.ID
	}

	f := newFetcherFn()
	if *cacheDir != "" {
		f = Cached{Dir: *cacheDir, Next: f}
	}
	next, report, err := Sync(ctx, ids, meta, f, Options{Parallel: *parallel, Retries: 4, Backoff: 5 * time.Second})
	if err != nil {
		return err
	}
	printReport(stdout, report)

	if !*write {
		_, _ = fmt.Fprintln(stdout, "dry run: nothing written; rerun with -write to save")
		return nil
	}
	return writeMetadata(stdout, *metaFile, next)
}

// resolvePaths fills an unset repo and metadata path from toolkit's
// config file, the metadata path falling back to toolkit's default.
func resolvePaths(cfgFile, home string, repo, metaFile *string) error {
	cfg, err := readConfig(cfgFile)
	if err != nil {
		return err
	}
	if *repo == "" {
		*repo = cfg.RepoPath
	}
	if *repo == "" {
		return errors.New("no repo path: pass -repo or set repo-path in the config file")
	}
	if *metaFile == "" {
		*metaFile = cfg.MetadataFile
	}
	if *metaFile == "" {
		*metaFile = filepath.Join(home, ".config", "toolkit", "metadata.yaml")
	}
	return nil
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

// readConfig reads toolkit's config file; a missing one is empty.
func readConfig(path string) (toolkitConfig, error) {
	var cfg toolkitConfig
	data, err := os.ReadFile(path) //nolint:gosec // the user's own config file
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse %s: %w", path, err)
	}
	return cfg, nil
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
