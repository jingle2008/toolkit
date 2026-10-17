package configloader

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/pkg/models"
)

func addTenant(id string) func(*models.Metadata) error {
	return func(m *models.Metadata) error {
		return UpsertTenant(m, models.TenantMetadata{ID: id, Name: strptr(id)})
	}
}

func tenantIDs(t *testing.T, path string) []string {
	t.Helper()
	m, err := LoadMetadata(path)
	require.NoError(t, err)
	ids := make([]string, len(m.Tenants))
	for i, e := range m.Tenants {
		ids[i] = e.ID
	}
	return ids
}

func TestUpdateMetadata_CreatesMissingFile(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "metadata.yaml")
	got, err := UpdateMetadata(path, addTenant("a"))
	require.NoError(t, err)
	require.Len(t, got.Tenants, 1)
	assert.Equal(t, []string{"a"}, tenantIDs(t, path))
}

// The change applies to the file as it is now, not as some caller last
// read it: what another process wrote in between survives.
func TestUpdateMetadata_AppliesToCurrentFile(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "metadata.yaml")
	require.NoError(t, SaveMetadata(path, &models.Metadata{Tenants: []models.TenantMetadata{{ID: "other", Status: "ACTIVE"}}}))

	got, err := UpdateMetadata(path, addTenant("a"))
	require.NoError(t, err)
	assert.Len(t, got.Tenants, 2, "returns what was written")
	assert.ElementsMatch(t, []string{"other", "a"}, tenantIDs(t, path))
}

func TestUpdateMetadata_RefusesUnparseableFile(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "metadata.yaml")
	const corrupt = "this: : not: valid: yaml\n\t- broken"
	require.NoError(t, os.WriteFile(path, []byte(corrupt), 0o600))

	_, err := UpdateMetadata(path, func(*models.Metadata) error {
		t.Fatal("change applied to an unreadable file")
		return nil
	})
	require.Error(t, err)
	raw, rerr := os.ReadFile(path) // #nosec G304 -- test-controlled t.TempDir() file
	require.NoError(t, rerr)
	assert.Equal(t, corrupt, string(raw), "left untouched")
}

func TestUpdateMetadata_ChangeErrorWritesNothing(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "metadata.yaml")
	boom := errors.New("boom")
	_, err := UpdateMetadata(path, func(m *models.Metadata) error {
		m.Tenants = append(m.Tenants, models.TenantMetadata{ID: "a"})
		return boom
	})
	require.ErrorIs(t, err, boom)
	assert.NoFileExists(t, path)
}

// Concurrent updates serialize: none is lost.
func TestUpdateMetadata_ConcurrentUpdatesAllLand(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "metadata.yaml")
	const n = 20
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() {
			_, err := UpdateMetadata(path, addTenant(fmt.Sprintf("t%02d", i)))
			assert.NoError(t, err)
		})
	}
	wg.Wait()
	assert.Len(t, tenantIDs(t, path), n)
}
