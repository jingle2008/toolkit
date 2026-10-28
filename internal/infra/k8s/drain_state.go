// Drain state: the DrainState recorded on the Node (DrainAnnotation),
// read back and written with compare-and-swap on its resourceVersion so
// concurrent drivers don't clobber each other.

package k8s

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	"github.com/jingle2008/toolkit/internal/logging"
)

const drainCASRetries = 5

// errDrainSuperseded means another driver changed the drain state first.
var errDrainSuperseded = errors.New("drain state changed concurrently")

/*
transition atomically replaces the node's drain state from with next,
optionally setting spec.unschedulable in the same patch. It returns
errDrainSuperseded when the stored state is no longer from, and retries
resourceVersion conflicts caused by unrelated node updates.
*/
func (d *drainer) transition(ctx context.Context, from, next DrainState, unschedulable *bool) error {
	raw, err := json.Marshal(next)
	if err != nil {
		return fmt.Errorf("marshal drain state: %w", err)
	}
	for range drainCASRetries {
		node, err := d.clientset.CoreV1().Nodes().Get(ctx, d.node, v1.GetOptions{})
		if err != nil {
			return err
		}
		if !readDrainState(ctx, node).sameAs(from) {
			return errDrainSuperseded
		}
		patch := map[string]any{"metadata": map[string]any{
			// A stale resourceVersion makes the API server reject the patch.
			"resourceVersion": node.ResourceVersion,
			"annotations":     map[string]string{DrainAnnotation: string(raw)},
		}}
		if unschedulable != nil {
			patch["spec"] = map[string]any{"unschedulable": *unschedulable}
		}
		body, err := json.Marshal(patch)
		if err != nil {
			return fmt.Errorf("marshal drain patch: %w", err)
		}
		_, err = d.clientset.CoreV1().Nodes().Patch(ctx, d.node, types.MergePatchType, body, v1.PatchOptions{})
		if !apierrors.IsConflict(err) {
			return err
		}
	}
	return fmt.Errorf("update drain state on node %s: too many conflicts", d.node)
}

func (s DrainState) sameAs(o DrainState) bool {
	return s.Phase == o.Phase && s.StartedAt.Equal(o.StartedAt) && s.PhaseStartedAt.Equal(o.PhaseStartedAt)
}

// readDrainState decodes the node's drain annotation. A missing or
// unreadable annotation reads as no drain, so a new one can replace it.
func readDrainState(ctx context.Context, node *corev1.Node) DrainState {
	raw, ok := node.Annotations[DrainAnnotation]
	if !ok {
		return DrainState{Phase: DrainNone}
	}
	var st DrainState
	if err := json.Unmarshal([]byte(raw), &st); err != nil || st.Phase == "" {
		logging.FromContext(ctx).Warn("drain: ignoring unreadable drain annotation", "node", node.Name, "value", raw, "error", err)
		return DrainState{Phase: DrainNone}
	}
	return st
}

func (d *drainer) status(ctx context.Context) (DrainStatus, error) {
	node, err := d.clientset.CoreV1().Nodes().Get(ctx, d.node, v1.GetOptions{})
	if err != nil {
		return DrainStatus{}, err
	}
	out := DrainStatus{Node: d.node, DrainState: readDrainState(ctx, node)}
	var timeout time.Duration
	switch out.Phase {
	case DrainStarting:
		timeout = drainStartingStale
	case DrainWaiting:
		timeout = drainRescheduleTimeout
		pods, err := podsOnNode(ctx, d.clientset, d.node)
		if err != nil {
			return DrainStatus{}, err
		}
		out.Remaining = remainingServices(out.DrainState, pods)
	case DrainEvicting:
		timeout = drainEvictTimeout
		list, errs := d.evictHelper(ctx).GetPodsForDeletion(d.node)
		if len(errs) > 0 {
			return DrainStatus{}, utilerrors.NewAggregate(errs)
		}
		out.Remaining = podNames(list.Pods())
	default:
		return out, nil
	}
	deadline := out.PhaseStartedAt.Add(timeout)
	out.Deadline = &deadline
	return out, nil
}
