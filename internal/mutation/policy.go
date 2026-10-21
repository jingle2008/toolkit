// Package mutation is the one list of what toolkit can change, shared by
// the TUI, CLI and MCP server: each action's risk tier, which decides how
// strongly every surface confirms it, and the audit record every surface
// writes for it. Keeping both here is what keeps the surfaces from
// drifting apart (a destructive action gated on one surface but not
// another, or audited under different names).
package mutation

import "github.com/jingle2008/toolkit/pkg/infra/logging"

// Tier classifies an action by blast radius.
type Tier int

const (
	// Recoverable actions can be undone or retried: a plain confirmation
	// suffices (TUI y, CLI prompt, MCP confirm=true).
	Recoverable Tier = iota
	// Irreversible actions destroy state and need the strongest
	// confirmation each surface has (TUI capital Y, CLI --yes, MCP
	// confirm_target).
	Irreversible
)

// Action is one kind of change: a verb on a kind of resource.
type Action struct {
	Verb string // as audited: cordon, drain, restart, terminate, delete, …
	Kind string // as audited: node, gpu_pool, dac, inference_service, tenant
	Tier Tier
}

// String returns "<verb> <kind>", e.g. "terminate node".
func (a Action) String() string { return a.Verb + " " + a.Kind }

// The actions. Verb and Kind are the audit record's action and kind
// fields, unchanged from before this package existed.
var (
	CordonNode                  = Action{"cordon", "node", Recoverable}
	UncordonNode                = Action{"uncordon", "node", Recoverable}
	DrainNode                   = Action{"drain", "node", Recoverable}
	RestartNode                 = Action{"restart", "node", Recoverable}
	TerminateNode               = Action{"terminate", "node", Irreversible}
	ScaleGPUPool                = Action{"scale", "gpu_pool", Recoverable}
	DeleteDAC                   = Action{"delete", "dac", Irreversible}
	RestartInferenceService     = Action{"restart", "inference_service", Recoverable}
	DeleteInferenceService      = Action{"delete", "inference_service", Irreversible}
	ForceDeleteInferenceService = Action{"force-delete", "inference_service", Irreversible}
	SetTenant                   = Action{"set", "tenant", Recoverable}
)

// Actions returns every action, for surfaces and tests to check they
// cover the whole list.
func Actions() []Action {
	return []Action{
		CordonNode, UncordonNode, DrainNode, RestartNode, TerminateNode,
		ScaleGPUPool, DeleteDAC,
		RestartInferenceService, DeleteInferenceService, ForceDeleteInferenceService,
		SetTenant,
	}
}

// Lookup returns the action with verb and kind.
func Lookup(verb, kind string) (Action, bool) {
	for _, a := range Actions() {
		if a.Verb == verb && a.Kind == kind {
			return a, true
		}
	}
	return Action{}, false
}

// InferenceServiceDelete returns the delete of an orphaned
// InferenceService as planned: a force-delete (finalizer removal) when
// force, else a plain delete.
func InferenceServiceDelete(force bool) Action {
	if force {
		return ForceDeleteInferenceService
	}
	return DeleteInferenceService
}

// Phase is where in its life a mutation's audit record was written.
type Phase string

// The phases, in order. Refused and DryRun end a mutation that never ran.
const (
	PhaseRefused Phase = "refused" // confirmation missing or wrong
	PhaseDryRun  Phase = "dry_run" // resolved, not performed
	PhaseBegin   Phase = "begin"
	PhaseFailed  Phase = "failed"
	PhaseDone    Phase = "done"
)

// Audit writes the "mutation" record for action on target at phase,
// from surface ("tui", "cli" or "mcp"). err is recorded on PhaseFailed,
// which logs at error level; every other phase logs at info. fields are
// the action's own key-value pairs (a scale's sizes), appended after the
// standard ones so a search on target still matches.
func Audit(logger logging.Logger, surface string, a Action, target string, phase Phase, err error, fields ...any) {
	kv := append([]any{
		"action", a.Verb,
		"kind", a.Kind,
		"target", target,
		"surface", surface,
		"phase", string(phase),
		"dry_run", phase == PhaseDryRun,
	}, fields...)
	if phase == PhaseFailed {
		logger.Errorw("mutation", append(kv, "error", err)...)
		return
	}
	logger.Infow("mutation", kv...)
}
