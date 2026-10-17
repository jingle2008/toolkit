package tui

import (
	"github.com/jingle2008/toolkit/internal/mutation"
	"github.com/jingle2008/toolkit/pkg/infra/logging"
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
