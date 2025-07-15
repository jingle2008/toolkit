// Package actions provides side-effectful operations for the TUI Model.
// This includes clipboard, k8s, and other effectful helpers, decoupled from the reducer logic.
package actions

import (
	"fmt"

	"github.com/atotto/clipboard"
	"github.com/jingle2008/toolkit/pkg/infra/logging"
	"github.com/jingle2008/toolkit/pkg/models"
)

// CopyItemName copies the name or ID of an item to the clipboard.
func CopyItemName(item any, env *models.Environment, logger logging.Logger) {
	if item == nil {
		logger.Errorw("no item selected for copying name")
		return
	}

	if dac, ok := item.(*models.DedicatedAICluster); ok {
		id := fmt.Sprintf("ocid1.generativeaidedicatedaicluster.%s.%s.%s",
			env.Realm, env.Region, dac.Name)
		if err := clipboard.WriteAll(id); err != nil {
			logger.Errorw("failed to copy id to clipboard", "error", err)
		}
	} else if to, ok := item.(models.NamedItem); ok {
		if err := clipboard.WriteAll(to.GetName()); err != nil {
			logger.Errorw("failed to copy name to clipboard", "error", err)
		}
	} else {
		logger.Errorw("unsupported item type for copying name", "item", item)
	}
}

// CopyTenantID copies the tenant ID from the current row to the clipboard if available.
func CopyTenantID(item any, env *models.Environment, logger logging.Logger) {
	if item == nil {
		logger.Errorw("no item selected for copying tenant ID")
		return
	}

	if dac, ok := item.(*models.DedicatedAICluster); ok {
		tenantID := fmt.Sprintf("ocid1.tenancy.%s..%s", env.Realm, dac.TenantID)
		if err := clipboard.WriteAll(tenantID); err != nil {
			logger.Errorw("failed to copy tenantID to clipboard", "error", err)
		}
	} else if to, ok := item.(models.TenancyOverride); ok {
		if err := clipboard.WriteAll(to.GetTenantID()); err != nil {
			logger.Errorw("failed to copy tenantID to clipboard", "error", err)
		}
	} else {
		logger.Errorw("unsupported item type for copying tenant ID", "item", item)
	}
}
