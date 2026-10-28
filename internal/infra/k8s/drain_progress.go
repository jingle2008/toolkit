// Drain progress: the milestones a drain reports as it goes (to a
// caller's DrainProgressFunc and the log), throttled when they repeat.

package k8s

import (
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"

	"github.com/jingle2008/toolkit/internal/logging"
)

type noteMark struct {
	sig string
	at  time.Time
}

// DrainProgressFunc receives a drain's milestones as one-line summaries.
type DrainProgressFunc func(line string)

type drainProgressKey struct{}

// WithDrainProgress returns a ctx whose drains also report each milestone
// they log to report, e.g. to show progress on a terminal.
func WithDrainProgress(ctx context.Context, report DrainProgressFunc) context.Context {
	return context.WithValue(ctx, drainProgressKey{}, report)
}

// DrainProgressFrom returns ctx's progress hook, or a no-op.
func DrainProgressFrom(ctx context.Context) DrainProgressFunc {
	if report, ok := ctx.Value(drainProgressKey{}).(DrainProgressFunc); ok && report != nil {
		return report
	}
	return func(string) {}
}

// note logs a drain milestone and reports it to ctx's progress hook.
func note(ctx context.Context, warn bool, msg string, kv ...any) {
	logger := logging.FromContext(ctx)
	if warn {
		logger.Warn(msg, kv...)
	} else {
		logger.Info(msg, kv...)
	}
	var b strings.Builder
	b.WriteString(msg)
	for i := 0; i+1 < len(kv); i += 2 {
		fmt.Fprintf(&b, " %v=%v", kv[i], kv[i+1])
	}
	DrainProgressFrom(ctx)(b.String())
}

/*
shouldNote reports whether a repeating progress line is due: when its
content sig differs from the last one noted under key, or, with every > 0,
when every has passed since. It keeps a long wait or a PDB-blocked
eviction from flooding the log (and the TUI's bounded log window) with a
line per poll.
*/
func (d *drainer) shouldNote(key, sig string, every time.Duration) bool {
	now := drainNow()
	if last, ok := d.noted[key]; ok && last.sig == sig && (every <= 0 || now.Sub(last.at) < every) {
		return false
	}
	if d.noted == nil {
		d.noted = map[string]noteMark{}
	}
	d.noted[key] = noteMark{sig: sig, at: now}
	return true
}

func (d *drainer) noteRestartPlan(ctx context.Context, restarted []string) {
	if len(restarted) == 0 {
		note(ctx, false, "drain: no single-replica InferenceServices to restart", "node", d.node)
		return
	}
	note(ctx, false, "drain: restarting single-replica InferenceServices", "node", d.node, "services", restarted)
}

// noteEvictionWarnings logs, once each, kubectl drain's warnings (e.g.
// DaemonSet pods ignored) and the pods whose emptyDir data eviction loses.
// The latter is detected here because kubectl's own emptyDir warning is
// overwritten by its later filters for any controller-managed pod.
func (d *drainer) noteEvictionWarnings(ctx context.Context, warnings string, pods []corev1.Pod) {
	if warnings != "" && d.shouldNote("evict-warnings", warnings, 0) {
		note(ctx, true, "drain: "+warnings, "node", d.node)
	}
	if lossy := emptyDirPods(pods); len(lossy) > 0 && d.shouldNote("evict-emptydir", strings.Join(lossy, ","), 0) {
		note(ctx, true, "drain: evicting pods with emptyDir volumes; their emptyDir data is lost", "node", d.node, "pods", lossy)
	}
}
