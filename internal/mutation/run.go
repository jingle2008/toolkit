package mutation

import (
	"context"
	"errors"
	"log/slog"
)

// Plan is what a mutation will do, once its target is looked up.
type Plan struct {
	// Action is what the mutation does. Resolve may refine it (a delete
	// that turns out to be a force-delete).
	Action Action
	// Target names what it acts on, in prompts, results and audit records.
	// Resolve may refine it (an InferenceService's <namespace>/<name>).
	Target string
	// Resolved is what Resolve found, which a dry run reports.
	Resolved Resolved
	// Noop, when Resolve sets it, ends the call with this note: there is
	// nothing to do, so nothing is confirmed, performed or audited.
	Noop string
	// Notice is what the action will do (a scale's sizes) and any warning,
	// for a dry run and the confirmation.
	Notice string
}

// String is "<verb> <kind>/<target>".
func (p Plan) String() string { return p.Action.Verb + " " + p.Action.Kind + "/" + p.Target }

// Op is one mutation, as a front-end describes it.
type Op struct {
	// Plan is the action and target as requested, before Resolve.
	Plan Plan
	// Resolve, when set, looks the target up without changing anything,
	// before the dry run or the confirmation, so a dry run proves the
	// target exists and the confirmation names what was found.
	Resolve func(ctx context.Context, p *Plan) error
	// Perform acts on what Resolve found.
	Perform func(ctx context.Context, p Plan) error
	// AuditFields, when set, adds the action's own fields to every audit
	// record. It is called per record, so fields Perform fills in (a
	// scale's actual sizes) reach the done record.
	AuditFields func() []any
}

// Gate is a front-end's confirmation rules.
type Gate struct {
	// DryRun reports what would run instead of running it.
	DryRun bool
	// Admit, when set, runs before Resolve, on the plan as requested: an
	// error refuses the call before anything is touched.
	Admit func(p Plan) error
	// Confirm, when set, runs after Resolve on a real run: an error ends
	// the call. A Refusal is audited as refused; any other error (the
	// prompt couldn't be read) ends it unaudited.
	Confirm func(ctx context.Context, p Plan) error
}

// Outcome is how a call that returned no error ended.
type Outcome int

// The outcomes.
const (
	Performed Outcome = iota
	DryRun            // reported, not performed
	Noop              // nothing to do
)

// Refusal is a call refused by its Gate, audited as refused.
type Refusal struct{ Reason error }

func (r *Refusal) Error() string { return r.Reason.Error() }
func (r *Refusal) Unwrap() error { return r.Reason }

// Refuse wraps reason as a Refusal.
func Refuse(reason error) error { return &Refusal{Reason: reason} }

// Run is the mutation flow every front-end shares, with the audit record
// of each step:
//
//  1. Admit: refused, before touching anything.
//  2. Resolve looks the target up; a failure ends the call, audited as
//     failed unless a dry run. A Noop it sets ends the call.
//  3. A dry run ends here, audited dry_run.
//  4. Confirm: a Refusal is audited refused.
//  5. Perform, audited begin, then done or failed.
//
// It returns the plan as resolved, which a front-end renders, and how
// the call ended. Errors from the gate, Resolve and Perform are returned
// as they are.
func Run(ctx context.Context, logger *slog.Logger, surface string, gate Gate, op Op) (Plan, Outcome, error) {
	p := op.Plan
	audit := func(phase Phase, err error) {
		var fields []any
		if op.AuditFields != nil {
			fields = op.AuditFields()
		}
		Audit(logger, surface, p.Action, p.Target, phase, err, fields...)
	}

	if gate.Admit != nil {
		if err := gate.Admit(p); err != nil {
			audit(PhaseRefused, nil)
			return p, Performed, err
		}
	}
	if op.Resolve != nil {
		if err := op.Resolve(ctx, &p); err != nil {
			if !gate.DryRun {
				audit(PhaseFailed, err)
			}
			return p, Performed, err
		}
	}
	if p.Noop != "" {
		return p, Noop, nil
	}
	if gate.DryRun {
		audit(PhaseDryRun, nil)
		return p, DryRun, nil
	}
	if gate.Confirm != nil {
		if err := gate.Confirm(ctx, p); err != nil {
			if refusal := (*Refusal)(nil); errors.As(err, &refusal) {
				audit(PhaseRefused, nil)
			}
			return p, Performed, err
		}
	}

	audit(PhaseBegin, nil)
	if err := op.Perform(ctx, p); err != nil {
		audit(PhaseFailed, err)
		return p, Performed, err
	}
	audit(PhaseDone, nil)
	return p, Performed, nil
}
