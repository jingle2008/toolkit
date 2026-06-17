// Package tui — tenant-metadata entry form (EditTenantView).
//
// Lets the user attach a friendly name / internal flag / note to an
// UNRESOLVED DedicatedAICluster or ImportedModel row, persist it to the
// metadata file via loader.TenantMetadataWriter, and auto-refresh so
// the row resolves.
package tui

import (
	"github.com/jingle2008/toolkit/pkg/models"
)

// editTarget identifies the tenant a row points at and whether it can
// be edited (unresolved + has a real tenancy id).
type editTarget struct {
	ocid     string // full tenancy OCID — the metadata entry key
	tenantID string // raw TenantID suffix, for display context
}

// tenantEditTarget inspects a selected item and returns an editTarget
// when it is an unresolved tenant-owned row (DAC or ImportedModel with
// Owner == nil and a real, non-orphan TenantID). ok is false otherwise.
func tenantEditTarget(item any, realm string) (editTarget, bool) { //nolint:unparam // realm varies at runtime; current tests use a single value
	var (
		ocid, tenantID string
		resolved       bool
	)
	switch v := item.(type) {
	case *models.DedicatedAICluster:
		if v == nil {
			return editTarget{}, false
		}
		ocid, tenantID, resolved = v.TenancyOCID(realm), v.TenantID, v.Owner != nil
	case *models.ImportedModel:
		if v == nil {
			return editTarget{}, false
		}
		ocid, tenantID, resolved = v.TenancyOCID(realm), v.TenantID, v.Owner != nil
	default:
		return editTarget{}, false
	}
	if resolved || tenantID == "" || tenantID == "UNKNOWN_TENANCY" {
		return editTarget{}, false
	}
	return editTarget{ocid: ocid, tenantID: tenantID}, true
}
