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
	return auditedWith(logger, a, target, nil, perform)
}

// auditedWith is audited with the action's own fields on every record.
// fields is called per record, so ones perform fills in (a scale's
// actual sizes) reach the done record; nil adds none.
func auditedWith(logger logging.Logger, a mutation.Action, target string, fields func() []any, perform func() error) error {
	audit := func(phase mutation.Phase, err error) {
		var kv []any
		if fields != nil {
			kv = fields()
		}
		mutation.Audit(logger, surfaceTUI, a, target, phase, err, kv...)
	}
	audit(mutation.PhaseBegin, nil)
	if err := perform(); err != nil {
		audit(mutation.PhaseFailed, err)
		return err
	}
	audit(mutation.PhaseDone, nil)
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
