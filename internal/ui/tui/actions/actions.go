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
func CopyItemName(item any, env models.Environment, logger logging.Logger) {
	if item == nil {
		logger.Errorw("no item selected for copying name")
		return
	}

	if dac, ok := item.(*models.DedicatedAICluster); ok {
		id := dac.GetID(env.Realm, env.Region)
		if err := clipboardWriteAll(id); err != nil {
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

// CopyTenantID copies the tenant ID from the current row to the clipboard if available.
func CopyTenantID(item any, env models.Environment, logger logging.Logger) {
	if item == nil {
		logger.Errorw("no item selected for copying tenant ID")
		return
	}

	if dac, ok := item.(*models.DedicatedAICluster); ok {
		tenantID := dac.GetTenantID(env.Realm)
		if err := clipboardWriteAll(tenantID); err != nil {
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
