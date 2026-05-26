// Package actions provides side-effectful operations for the TUI Model.
// This includes clipboard, k8s, and other effectful helpers, decoupled from the reducer logic.
package actions

import (
	"github.com/atotto/clipboard"

	"github.com/jingle2008/toolkit/pkg/infra/logging"
	"github.com/jingle2008/toolkit/pkg/models"
)

var clipboardWriteAll = clipboard.WriteAll

// CopyItemName copies the name or ID of an item to the clipboard.
// RealmedID implementers (DAC, ImportedModel) get their full OCID;
// other NamedItem implementers get their raw name.
func CopyItemName(item any, env models.Environment, logger logging.Logger) {
	if item == nil {
		logger.Errorw("no item selected for copying name")
		return
	}

	if r, ok := item.(models.RealmedID); ok {
		if err := clipboardWriteAll(r.OCID(env.Realm, env.Region)); err != nil {
			logger.Errorw("failed to copy id to clipboard", "error", err)
		}
	} else if to, ok := item.(models.NamedItem); ok {
		if err := clipboardWriteAll(to.GetName()); err != nil {
			logger.Errorw("failed to copy name to clipboard", "error", err)
		}
	} else {
		logger.Errorw("unsupported item type for copying name", "item", item)
	}
}

// CopyTenantID copies the tenant ID from the current row to the
// clipboard. RealmedTenancyID implementers (DAC, ImportedModel) get
// their full tenancy OCID; TenancyOverride implementers (file-backed
// override types) get whatever's stored in their TenantID field.
func CopyTenantID(item any, env models.Environment, logger logging.Logger) {
	if item == nil {
		logger.Errorw("no item selected for copying tenant ID")
		return
	}

	if r, ok := item.(models.RealmedTenancyID); ok {
		if err := clipboardWriteAll(r.TenancyOCID(env.Realm)); err != nil {
			logger.Errorw("failed to copy tenantID to clipboard", "error", err)
		}
	} else if to, ok := item.(models.TenancyOverride); ok {
		if err := clipboardWriteAll(to.GetTenantID()); err != nil {
			logger.Errorw("failed to copy tenantID to clipboard", "error", err)
		}
	} else {
		logger.Errorw("unsupported item type for copying tenant ID", "item", item)
	}
}
