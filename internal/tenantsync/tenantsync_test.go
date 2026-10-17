package tenantsync

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/configloader"
	"github.com/jingle2008/toolkit/pkg/models"
)

// details renders get-account-details output for one tenancy.
func details(id, name, typ, state string) []byte {
	return fmt.Appendf(nil, `{"data":{"account-response":{"account":{"name":%q,"tenancy-id":%q,"account-type":%q,"account-state":%q}}}}`,
		name, id, typ, state)
}

// fakeFetcher serves canned responses or errors per OCID and counts
// calls.
type fakeFetcher struct {
	mu    sync.Mutex
	raw   map[string][]byte
	errs  map[string][]error // returned in order before raw
	calls map[string]int
}

func (f *fakeFetcher) Fetch(_ context.Context, id string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.calls == nil {
		f.calls = map[string]int{}
	}
	f.calls[id]++
	if errs := f.errs[id]; len(errs) > 0 {
		f.errs[id] = errs[1:]
		return nil, errs[0]
	}
	if raw, ok := f.raw[id]; ok {
		return raw, nil
	}
	return nil, ErrNotFound
}

func strp(s string) *string { return &s }
func boolp(b bool) *bool    { return &b }

func TestParseAccount(t *testing.T) {
	t.Parallel()
	a, err := ParseAccount(details("id-a", "acme", "INTERNAL", "ACTIVE"))
	require.NoError(t, err)
	assert.Equal(t, Account{TenancyID: "id-a", Name: "acme", Type: "INTERNAL", State: "ACTIVE"}, a)

	withPrompt := append(details("id-a", "acme", "INTERNAL", "ACTIVE"), "\nERROR: This CLI session has expired\nDo you want to re-authenticate? [Y/n]:"...)
	_, err = ParseAccount(withPrompt)
	require.NoError(t, err, "a prompt after the JSON is ignored")

	_, err = ParseAccount([]byte(`{"data":{}}`))
	require.Error(t, err)
	_, err = ParseAccount([]byte("not json"))
	require.Error(t, err)
}

func TestClassify(t *testing.T) {
	t.Parallel()
	exit := errors.New("exit status 1")
	assert.ErrorIs(t, classify(nil, []byte(`"code": "NotAuthenticated"`), exit), ErrNotAuthenticated)
	assert.ErrorIs(t, classify([]byte("ERROR: This CLI session has expired"), nil, nil), ErrNotAuthenticated)
	assert.ErrorIs(t, classify(nil, []byte(`"code": "TooManyRequests"`), exit), ErrRateLimited)
	assert.ErrorIs(t, classify(nil, []byte(`"code": "NotAuthorizedOrNotFound"`), exit), ErrNotFound)
	assert.ErrorContains(t, classify(nil, []byte("boom\nmore"), exit), "oci-ops: exit status 1: boom")
	assert.ErrorContains(t, classify([]byte("garbage"), nil, nil), "unexpected oci-ops output: garbage")
}

//nolint:paralleltest // replaces the package-level runOCIOpsFn seam
func TestOCIOps_Fetch(t *testing.T) {
	orig := runOCIOpsFn
	t.Cleanup(func() { runOCIOpsFn = orig })
	var gotArgs []string
	runOCIOpsFn = func(_ context.Context, args ...string) ([]byte, []byte, error) {
		gotArgs = args
		if args[len(args)-1] == "id-a" {
			return details("id-a", "acme", "INTERNAL", "ACTIVE"), nil, nil
		}
		return nil, []byte(`"code": "NotAuthenticated"`), errors.New("exit status 1")
	}
	raw, err := OCIOps{Region: "r1"}.Fetch(context.Background(), "id-a")
	require.NoError(t, err)
	assert.Contains(t, string(raw), "acme")
	assert.Equal(t, []string{"account-admin-tool", "accounts", "get-account-details", "--region", "r1", "--account", "id-a"}, gotArgs)
	_, err = OCIOps{Region: "r1"}.Fetch(context.Background(), "id-b")
	assert.ErrorIs(t, err, ErrNotAuthenticated)
}

// A cached response is served without a lookup; a live one is saved so
// the next run reuses it.
func TestCached(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "id-a.json"), details("id-a", "acme", "INTERNAL", "ACTIVE"), 0o600))
	next := &fakeFetcher{raw: map[string][]byte{"id-b": details("id-b", "beta", "EXTERNAL", "ACTIVE")}}
	c := Cached{Dir: dir, Next: next}

	_, err := c.Fetch(context.Background(), "id-a")
	require.NoError(t, err)
	assert.Zero(t, next.calls["id-a"], "served from the cache")

	_, err = c.Fetch(context.Background(), "id-b")
	require.NoError(t, err)
	_, err = c.Fetch(context.Background(), "id-b")
	require.NoError(t, err)
	assert.Equal(t, 1, next.calls["id-b"], "saved on the first lookup, reused on the second")

	_, err = c.Fetch(context.Background(), "id-missing")
	assert.ErrorIs(t, err, ErrNotFound)
	assert.NoFileExists(t, filepath.Join(dir, "id-missing.json"), "failures aren't cached")
}

func TestSync(t *testing.T) {
	t.Parallel()
	meta := &models.Metadata{Tenants: []models.TenantMetadata{
		{ID: "id-mine", Name: strp("mine"), IsInternal: boolp(false), Note: strp("keep"), Status: "ACTIVE"},
		{ID: "id-taken", Name: strp("dup")},
	}}
	f := &fakeFetcher{
		raw: map[string][]byte{
			"id-new":   details("id-new", "fresh", "INTERNAL", "ACTIVE"),
			"id-mine":  details("id-mine", "service-name", "INTERNAL", "DELETING"),
			"id-taken": details("id-taken", "dup", "EXTERNAL", "ACTIVE"),
			"id-clash": details("id-clash", "dup", "EXTERNAL", "TERMINATED"),
			"id-retry": details("id-retry", "retried", "SOMETHING", "ACTIVE"),
			"id-wrong": details("id-other", "x", "INTERNAL", "ACTIVE"),
		},
		errs: map[string][]error{"id-retry": {ErrRateLimited, ErrRateLimited}},
	}
	ids := []string{"id-new", "id-mine", "id-taken", "id-clash", "id-retry", "id-wrong", "id-gone"}

	next, r, err := Sync(context.Background(), ids, meta, f, Options{Parallel: 2, Retries: 3})
	require.NoError(t, err)
	assert.Len(t, meta.Tenants, 2, "the input metadata is left alone")
	byID := map[string]models.TenantMetadata{}
	for _, e := range next.Tenants {
		byID[e.ID] = e
	}

	// A new tenancy gets everything.
	assert.Equal(t, models.TenantMetadata{ID: "id-new", Name: strp("fresh"), IsInternal: boolp(true), Status: "ACTIVE"}, byID["id-new"])
	// Yours: name, internal flag and note kept; status refreshed.
	assert.Equal(t, models.TenantMetadata{ID: "id-mine", Name: strp("mine"), IsInternal: boolp(false), Note: strp("keep"), Status: "DELETING"}, byID["id-mine"])
	// A name another tenancy has isn't taken; the rest still lands.
	assert.Nil(t, byID["id-clash"].Name)
	assert.Equal(t, "TERMINATED", byID["id-clash"].Status)
	// Retried past the rate limit; an unknown account type sets no flag.
	assert.Equal(t, "retried", *byID["id-retry"].Name)
	assert.Nil(t, byID["id-retry"].IsInternal)
	assert.Equal(t, 3, f.calls["id-retry"])
	// Failed lookups add nothing.
	assert.NotContains(t, byID, "id-wrong")
	assert.NotContains(t, byID, "id-gone")

	assert.Equal(t, 7, r.Looked)
	assert.ElementsMatch(t, []string{"id-new", "id-retry"}, changeIDs(r.Named))
	assert.ElementsMatch(t, []string{"id-mine", "id-clash"}, changeIDs(r.NameConflicts))
	assert.Equal(t, []Change{{ID: "id-mine", Tenant: "mine", From: "false", To: "true"}}, r.InternalConflicts)
	assert.ElementsMatch(t, []string{"id-new", "id-taken", "id-clash"}, changeIDs(r.InternalSet))
	assert.Contains(t, r.StatusChanged, Change{ID: "id-mine", Tenant: "mine", From: "ACTIVE", To: "DELETING"})
	assert.ElementsMatch(t, []string{"id-wrong", "id-gone"}, changeIDs(r.Failed))
}

func changeIDs(cs []Change) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.ID
	}
	return out
}

// An expired session stops the sync rather than failing every tenancy.
func TestSync_NotAuthenticatedStops(t *testing.T) {
	t.Parallel()
	f := &fakeFetcher{errs: map[string][]error{"id-a": {ErrNotAuthenticated}}}
	_, _, err := Sync(context.Background(), []string{"id-a", "id-b"}, &models.Metadata{}, f, Options{Parallel: 1})
	assert.ErrorIs(t, err, ErrNotAuthenticated)
}

// writeRepo lays out one oc1 limit override for each id.
func writeRepo(t *testing.T, ids ...string) string {
	t.Helper()
	repo := t.TempDir()
	limits := filepath.Join(repo, "shared_modules", "limits")
	for _, kind := range []string{"console_properties", "properties"} {
		require.NoError(t, os.MkdirAll(filepath.Join(limits, kind+"_tenancy_overrides", "regional_values", "oc1"), 0o750))
	}
	for i, id := range ids {
		dir := filepath.Join(limits, "limits_tenancy_overrides", "regional_values", "oc1", fmt.Sprintf("t%d", i))
		require.NoError(t, os.MkdirAll(dir, 0o750))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "l.json"), fmt.Appendf(nil, `{"name":"l","tenant_id":%q}`, id), 0o600))
	}
	return repo
}

//nolint:paralleltest // replaces the package-level newFetcherFn seam
func TestRun_DryRunThenWrite(t *testing.T) {
	orig := newFetcherFn
	t.Cleanup(func() { newFetcherFn = orig })
	f := &fakeFetcher{raw: map[string][]byte{
		"ocid1.tenancy.oc1..a": details("ocid1.tenancy.oc1..a", "acme", "INTERNAL", "ACTIVE"),
		"ocid1.tenancy.oc1..b": details("ocid1.tenancy.oc1..b", "beta", "EXTERNAL", "DELETING"),
	}}
	newFetcherFn = func() Fetcher { return f }
	cfg := Config{
		RepoPath:     writeRepo(t, "ocid1.tenancy.oc1..a", "ocid1.tenancy.oc1..b"),
		MetadataFile: filepath.Join(t.TempDir(), "metadata.yaml"),
		Parallel:     2,
	}
	ctx := context.Background()

	var out bytes.Buffer
	require.NoError(t, Run(ctx, cfg, &out))
	assert.Contains(t, out.String(), "looked up 2 oc1 tenancies: 2 named, 2 status changes")
	assert.Contains(t, out.String(), "dry run")
	assert.NoFileExists(t, cfg.MetadataFile, "a dry run writes nothing")

	cfg.Write = true
	out.Reset()
	require.NoError(t, Run(ctx, cfg, &out))
	assert.Contains(t, out.String(), "wrote "+cfg.MetadataFile)
	assert.NotContains(t, out.String(), "backup", "no file to back up yet")
	data, err := os.ReadFile(cfg.MetadataFile)
	require.NoError(t, err)
	assert.Contains(t, string(data), "name: acme")
	assert.Contains(t, string(data), "status: DELETING")

	// A second write backs up the first, and reports a status change.
	f.raw["ocid1.tenancy.oc1..a"] = details("ocid1.tenancy.oc1..a", "acme", "INTERNAL", "TERMINATED")
	out.Reset()
	require.NoError(t, Run(ctx, cfg, &out))
	assert.Contains(t, out.String(), "acme: ACTIVE -> TERMINATED")
	backups, _ := filepath.Glob(cfg.MetadataFile + ".bak-*")
	assert.Len(t, backups, 1)
}

// hookFetcher runs onFirst before its first lookup, standing in for
// another toolkit process writing the metadata file mid-sync.
type hookFetcher struct {
	Fetcher
	once    sync.Once
	onFirst func()
}

func (h *hookFetcher) Fetch(ctx context.Context, id string) ([]byte, error) {
	h.once.Do(h.onFirst)
	return h.Fetcher.Fetch(ctx, id)
}

// A `set tenant` that lands while the lookups run survives the write:
// the results apply to the file as it is when written.
//
//nolint:paralleltest // replaces the package-level newFetcherFn seam
func TestRun_WriteKeepsConcurrentEdits(t *testing.T) {
	orig := newFetcherFn
	t.Cleanup(func() { newFetcherFn = orig })
	cfg := Config{
		RepoPath:     writeRepo(t, "ocid1.tenancy.oc1..a"),
		MetadataFile: filepath.Join(t.TempDir(), "metadata.yaml"),
		Parallel:     1,
		Write:        true,
	}
	newFetcherFn = func() Fetcher {
		return &hookFetcher{
			Fetcher: &fakeFetcher{raw: map[string][]byte{
				"ocid1.tenancy.oc1..a": details("ocid1.tenancy.oc1..a", "acme", "INTERNAL", "ACTIVE"),
			}},
			onFirst: func() {
				_, err := configloader.UpdateMetadata(cfg.MetadataFile, func(m *models.Metadata) error {
					return configloader.UpsertTenant(m, models.TenantMetadata{ID: "ocid1.tenancy.oc1..z", Name: strp("zed")})
				})
				require.NoError(t, err)
			},
		}
	}

	var out bytes.Buffer
	require.NoError(t, Run(context.Background(), cfg, &out))
	got, err := configloader.LoadMetadata(cfg.MetadataFile)
	require.NoError(t, err)
	byID := map[string]models.TenantMetadata{}
	for _, e := range got.Tenants {
		byID[e.ID] = e
	}
	require.Contains(t, byID, "ocid1.tenancy.oc1..z", "the mid-sync edit was overwritten")
	assert.Equal(t, "ACTIVE", byID["ocid1.tenancy.oc1..a"].Status)
	backups, _ := filepath.Glob(cfg.MetadataFile + ".bak-*")
	require.Len(t, backups, 1, "the file the sync replaced is backed up")
	saved, err := os.ReadFile(backups[0])
	require.NoError(t, err)
	assert.Contains(t, string(saved), "ocid1.tenancy.oc1..z", "the backup holds the file as written mid-sync")
	assert.NotContains(t, string(saved), "acme", "taken before the sync's write")
}

//nolint:paralleltest // replaces the package-level newFetcherFn seam
func TestRun_Errors(t *testing.T) {
	orig := newFetcherFn
	t.Cleanup(func() { newFetcherFn = orig })
	newFetcherFn = func() Fetcher {
		return &fakeFetcher{errs: map[string][]error{"ocid1.tenancy.oc1..a": {ErrNotAuthenticated}}}
	}
	ctx := context.Background()
	dir := t.TempDir()
	repo := writeRepo(t, "ocid1.tenancy.oc1..a")
	var out bytes.Buffer

	require.ErrorContains(t, Run(ctx, Config{MetadataFile: "m.yaml"}, &out), "no repo path")
	require.ErrorContains(t, Run(ctx, Config{RepoPath: repo}, &out), "no metadata file")
	require.ErrorIs(t, Run(ctx, Config{RepoPath: repo, MetadataFile: filepath.Join(dir, "m.yaml")}, &out), ErrNotAuthenticated)

	bad := filepath.Join(dir, "bad.yaml")
	require.NoError(t, os.WriteFile(bad, []byte("tenants:\n- {id: a, name: x}\n- {id: b, name: x}\n"), 0o600))
	require.ErrorContains(t, Run(ctx, Config{RepoPath: repo, MetadataFile: bad}, &out), "invalid tenant metadata")
}
