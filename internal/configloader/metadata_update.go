package configloader

import (
	"errors"
	"fmt"
	"os"

	"github.com/jingle2008/toolkit/pkg/models"
)

// CheckTenantUpsert reports whether UpsertTenant would take entry into
// the metadata file as it is now, writing nothing: the check a set-tenant
// dry run makes. exists says whether the file already has the tenancy.
// A missing file is empty; one that doesn't load is an error, as
// UpdateMetadata would refuse it.
func CheckTenantUpsert(path string, entry models.TenantMetadata) (exists bool, err error) {
	m := &models.Metadata{}
	if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
		if m, err = LoadMetadata(path); err != nil {
			return false, fmt.Errorf("metadata file %s: %w", path, err)
		}
	}
	for _, t := range m.Tenants {
		exists = exists || t.ID == entry.ID
	}
	return exists, UpsertTenant(m, entry)
}

// UpdateMetadata applies change to the metadata file as it is now and
// saves the result, returning what it wrote. Every toolkit process (the
// TUI, an MCP server, `set tenant`, `tenant sync`) writes through it, so
// none overwrites what another wrote since it last read the file:
// holding an exclusive lock on path+".lock", it re-reads path (a missing
// file is empty), applies change, and saves atomically. A file that
// doesn't load is left untouched and change is not called; nor is
// anything written when change fails.
func UpdateMetadata(path string, change func(*models.Metadata) error) (*models.Metadata, error) {
	unlock, err := lockFile(path + ".lock")
	if err != nil {
		return nil, fmt.Errorf("lock metadata file: %w", err)
	}
	defer unlock()

	m := &models.Metadata{}
	if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
		if m, err = LoadMetadata(path); err != nil {
			return nil, fmt.Errorf("refusing to overwrite metadata file %s: %w", path, err)
		}
	}
	if err := change(m); err != nil {
		return nil, err
	}
	if err := SaveMetadata(path, m); err != nil {
		return nil, err
	}
	return m, nil
}
