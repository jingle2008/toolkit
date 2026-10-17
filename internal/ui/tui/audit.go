package tui

import (
	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/mutation"
	"github.com/jingle2008/toolkit/pkg/infra/logging"
	"github.com/jingle2008/toolkit/pkg/models"
)

// surfaceTUI is the audit record's surface for every TUI mutation.
const surfaceTUI = "tui"

// audited runs perform, the confirmed action a on target, between the
// shared audit records the CLI and MCP write too: begin, then done or
// failed. Commands call it off the Update loop, where the action runs.
// target is the resource's own name (a node, pool or DAC name, or
// <namespace>/<name>), as the other surfaces audit it.
func audited(logger logging.Logger, a mutation.Action, target string, perform func() error) error {
	mutation.Audit(logger, surfaceTUI, a, target, mutation.PhaseBegin, nil)
	if err := perform(); err != nil {
		mutation.Audit(logger, surfaceTUI, a, target, mutation.PhaseFailed, err)
		return err
	}
	mutation.Audit(logger, surfaceTUI, a, target, mutation.PhaseDone, nil)
	return nil
}

// auditName is the audit target for the current category's row at key:
// the resource's own name, <namespace>/<name> for an InferenceService,
// as the commands' records and the CLI and MCP name it.
func (m *Model) auditName(key models.ItemKey) string {
	k, ok := key.(models.ScopedItemKey)
	if !ok {
		return itemKeyString(key)
	}
	if m.category == domain.InferenceService {
		return k.Scope + "/" + k.Name
	}
	return k.Name
}
