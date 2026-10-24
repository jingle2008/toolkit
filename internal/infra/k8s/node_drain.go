package k8s

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubectl/pkg/drain"

	"github.com/jingle2008/toolkit/internal/logging"
)

/*
A drain is a resumable state machine persisted in the DrainAnnotation on
the Node, so it outlives the process that started it: any later call
(StartDrain, AwaitDrain, DrainNode — from the CLI, TUI, or MCP server)
picks it up where it stopped. Every state change is a compare-and-swap
against the Node's resourceVersion, so concurrent drivers never apply a
step twice or regress a finished drain.

	(none) ──StartDrain──▶ starting ──▶ waiting ──▶ evicting ──▶ done
	                          │            │           │
	                          └────────────┴───────────┴──▶ failed

  - starting: the node is cordoned (one patch with the state, which makes
    it the drain lock); blocker pods are checked and single-replica
    InferenceServices restarted.
  - waiting:  the restarted services' surge pods come up elsewhere and
    their old pods leave the node.
  - evicting: everything else is evicted, kubectl-drain style.

Failing before eviction uncordons a node the drain cordoned, in the same
patch that records the failure; failing during eviction leaves it
cordoned, since pods may already be gone.
*/

// DrainAnnotation is the Node annotation holding the drain's DrainState.
const DrainAnnotation = "toolkit/drain"

// DrainPhase is a drain's position in the state machine.
type DrainPhase string

// Drain phases.
const (
	DrainNone     DrainPhase = "none" // no drain recorded on the node
	DrainStarting DrainPhase = "starting"
	DrainWaiting  DrainPhase = "waiting"
	DrainEvicting DrainPhase = "evicting"
	DrainDone     DrainPhase = "done"
	DrainFailed   DrainPhase = "failed"
)

// InProgress reports whether the drain still has work to do.
func (p DrainPhase) InProgress() bool {
	return p == DrainStarting || p == DrainWaiting || p == DrainEvicting
}

// DrainState is the persisted drain record.
type DrainState struct {
	Phase     DrainPhase `json:"phase"`
	StartedAt time.Time  `json:"startedAt,omitzero"`
	// PhaseStartedAt anchors the current phase's timeout.
	PhaseStartedAt time.Time `json:"phaseStartedAt,omitzero"`
	// CordonedByDrain is false when the node was already cordoned; a
	// rollback then leaves it cordoned.
	CordonedByDrain bool `json:"cordonedByDrain"`
	// Restarted lists the InferenceServices restarted, as group:ns/name.
	Restarted []string `json:"restarted,omitempty"`
	// Message explains a failure.
	Message string `json:"message,omitempty"`
}

// DrainStatus is a drain's persisted state plus a live view of the node.
type DrainStatus struct {
	Node string `json:"node"`
	DrainState
	// Deadline is when the current phase times out.
	Deadline *time.Time `json:"deadline,omitempty"`
	// Remaining lists what the current phase still waits on: restarted
	// InferenceServices (waiting) or pods to evict (evicting).
	Remaining []string `json:"remaining,omitempty"`
}

// Drain tunables. Vars so tests can shrink them.
var (
	// drainRescheduleTimeout bounds the wait for restarted InferenceService
	// pods to leave the node. Exceeding it almost always means no other
	// node has spare GPU capacity for the surge pod.
	drainRescheduleTimeout = 20 * time.Minute
	// drainEvictTimeout bounds the eviction phase (e.g. a
	// PodDisruptionBudget that never admits the eviction).
	drainEvictTimeout = 10 * time.Minute
	// drainStartingStale is how long a drain may sit in starting before
	// another driver concludes its owner died mid-start and rolls it back.
	// The start itself takes seconds.
	drainStartingStale = 2 * time.Minute
	// drainBeginTimeout bounds start's planning (blocker check, restart
	// plan) well inside drainStartingStale, so a slow but live owner gives
	// up before another driver could take it for dead and roll it back.
	drainBeginTimeout = time.Minute
	drainPollInterval = 10 * time.Second
	// drainMaxStepErrors is how many consecutive failed steps (API errors,
	// not drain failures) AwaitDrain tolerates before giving up.
	drainMaxStepErrors = 6
	// drainRollbackTimeout bounds recording a failure; it runs on a fresh
	// context so it survives the failed one being canceled.
	drainRollbackTimeout = 30 * time.Second
	// drainProgressEvery is how often a repeating progress line (what a
	// phase still waits on, a refused eviction) is re-logged while its
	// content stays the same; a change is logged at once.
	drainProgressEvery = time.Minute
	drainNow           = time.Now
)

const drainCASRetries = 5

// errDrainSuperseded means another driver changed the drain state first.
var errDrainSuperseded = errors.New("drain state changed concurrently")

const (
	fineTuneLabel       = "finetunedmodel-name"
	replicationJobLabel = "replicationJob"
)

// drainBlockerLabels mark pods whose eviction loses work in flight; a
// node running any of them must not be drained.
var drainBlockerLabels = []string{fineTuneLabel, trainingLabelV2, replicationJobLabel}

var inferenceServiceV1GVR = schema.GroupVersionResource{Group: "ome.oracle.com", Version: "v1alpha1", Resource: "inferenceservices"}

// servingAPI ties a serving pod label to the InferenceService API that owns
// the pod. The two OME generations keep minReplicas at different spec paths.
type servingAPI struct {
	label           string
	gvr             schema.GroupVersionResource
	minReplicasPath []string
}

var servingAPIs = []servingAPI{
	{label: servingLabelV2, gvr: inferenceServiceGVR, minReplicasPath: []string{"engine", "minReplicas"}},
	{label: servingLabelV1, gvr: inferenceServiceV1GVR, minReplicasPath: []string{"predictor", "minReplicas"}},
}

// isvcRef identifies one InferenceService with pods on the node.
type isvcRef struct {
	api       *servingAPI
	namespace string
	name      string
}

func (r isvcRef) String() string {
	return r.api.gvr.Group + ":" + r.namespace + "/" + r.name
}

// parseISVCRef inverts isvcRef.String.
func parseISVCRef(s string) (isvcRef, bool) {
	group, rest, ok := strings.Cut(s, ":")
	if !ok {
		return isvcRef{}, false
	}
	namespace, name, ok := strings.Cut(rest, "/")
	if !ok {
		return isvcRef{}, false
	}
	for i := range servingAPIs {
		if servingAPIs[i].gvr.Group == group {
			return isvcRef{api: &servingAPIs[i], namespace: namespace, name: name}, true
		}
	}
	return isvcRef{}, false
}

// Eviction seams for tests. checkEvictionSupport is an API discovery
// call, so it runs once per eviction round, not once per pod.
var (
	checkEvictionSupport = drain.CheckEvictionSupport
	// evictPod evicts one pod via the eviction API group gv, deleting it
	// instead where the cluster lacks the eviction API (gv empty).
	evictPod = func(helper *drain.Helper, pod corev1.Pod, gv schema.GroupVersion) error {
		if gv.Empty() {
			return helper.DeletePod(pod)
		}
		return helper.EvictPod(pod, gv)
	}
)

/*
StartDrain starts draining a node, or resumes the drain already in
progress, and returns without waiting for it: it cordons the node, refuses
nodes running fine-tuning, training, or replication-job pods, and restarts
single-replica InferenceServices so they surge onto other nodes before
eviction. Drive it to completion with AwaitDrain. A drain that has failed
returns its status along with an error.
*/
func StartDrain(ctx context.Context, kubeconfig, contextName, nodeName string) (DrainStatus, error) {
	d, err := newDrainer(kubeconfig, contextName, nodeName)
	if err != nil {
		return DrainStatus{}, err
	}
	return d.start(ctx)
}

/*
AwaitDrain drives the drain in progress on a node until it finishes,
returning nil once the node is drained and an error if the drain failed.
If ctx ends first the drain stays recorded on the node, to be resumed by
any later call.
*/
func AwaitDrain(ctx context.Context, kubeconfig, contextName, nodeName string) error {
	d, err := newDrainer(kubeconfig, contextName, nodeName)
	if err != nil {
		return err
	}
	return d.await(ctx)
}

// GetDrainStatus reports a node's drain without advancing it.
func GetDrainStatus(ctx context.Context, kubeconfig, contextName, nodeName string) (DrainStatus, error) {
	d, err := newDrainer(kubeconfig, contextName, nodeName)
	if err != nil {
		return DrainStatus{}, err
	}
	return d.status(ctx)
}

/*
DrainNode starts (or resumes) a node's drain and blocks until it finishes:
StartDrain followed by AwaitDrain. It can take tens of minutes; callers
must not impose a short deadline.
*/
func DrainNode(ctx context.Context, kubeconfig, contextName, nodeName string) error {
	d, err := newDrainer(kubeconfig, contextName, nodeName)
	if err != nil {
		return err
	}
	if _, err := d.start(ctx); err != nil {
		return err
	}
	return d.await(ctx)
}

type drainer struct {
	clientset kubernetes.Interface
	client    dynamic.Interface
	node      string
	// noted throttles repeating progress lines; see shouldNote.
	noted map[string]noteMark
}

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

func newDrainer(kubeconfig, contextName, nodeName string) (*drainer, error) {
	clientset, err := NewClientsetFromKubeConfig(kubeconfig, contextName)
	if err != nil {
		return nil, err
	}
	client, err := NewDynamicClientFromKubeConfig(kubeconfig, contextName)
	if err != nil {
		return nil, err
	}
	return &drainer{clientset: clientset, client: client, node: nodeName}, nil
}

func (d *drainer) start(ctx context.Context) (DrainStatus, error) {
	st, err := d.step(ctx, true)
	if err != nil {
		// The error may have struck after the drain was recorded and the
		// node cordoned. If a drain is now in progress, report that rather
		// than a failure, so the caller goes on to drive it instead of
		// leaving a cordoned node that nothing advances.
		status, serr := d.status(ctx)
		if serr != nil || !status.Phase.InProgress() {
			return DrainStatus{}, err
		}
		note(ctx, true, "drain: start step failed; drain is in progress, continuing", "node", d.node, "error", err)
		return status, nil
	}
	status, err := d.status(ctx)
	if err != nil {
		// The step succeeded; report it without the live view.
		logging.FromContext(ctx).Warn("drain: status read failed", "node", d.node, "error", err)
		status = DrainStatus{Node: d.node, DrainState: st}
	}
	if st.Phase == DrainFailed {
		return status, errors.New(st.Message)
	}
	return status, nil
}

func (d *drainer) await(ctx context.Context) error {
	streak := 0
	for {
		st, err := d.step(ctx, false)
		switch {
		case err != nil && ctx.Err() == nil:
			streak++
			note(ctx, true, "drain: step failed", "node", d.node, "attempt", streak, "error", err)
			if streak >= drainMaxStepErrors {
				return fmt.Errorf("drain node %s: %w (the drain stays recorded on the node; run drain again to resume)", d.node, err)
			}
		case err != nil:
		case st.Phase == DrainDone:
			return nil
		case st.Phase == DrainFailed:
			return errors.New(st.Message)
		case st.Phase == DrainNone:
			return fmt.Errorf("no drain in progress on node %s", d.node)
		default:
			streak = 0
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("stopped following drain of node %s: %w (the drain stays recorded on the node; run drain again to resume)",
				d.node, ctx.Err())
		case <-time.After(drainPollInterval):
		}
	}
}

// step advances the drain by at most one phase. With start set, a node
// without a drain in progress begins a new one — judged by the state seen
// on the first attempt only: a retry after losing a race must never start
// over a drain the winner just finished or rolled back.
func (d *drainer) step(ctx context.Context, start bool) (DrainState, error) {
	for attempt := 0; ; attempt++ {
		st, err := d.stepOnce(ctx, start && attempt == 0)
		if errors.Is(err, errDrainSuperseded) && attempt < drainCASRetries {
			continue // another driver moved first; act on its state
		}
		return st, err
	}
}

func (d *drainer) stepOnce(ctx context.Context, start bool) (DrainState, error) {
	node, err := d.clientset.CoreV1().Nodes().Get(ctx, d.node, v1.GetOptions{})
	if err != nil {
		return DrainState{}, err
	}
	st := readDrainState(ctx, node)
	switch st.Phase {
	case DrainStarting:
		if d.expired(st, drainStartingStale) {
			return d.fail(ctx, st, "drain start did not complete (its driver exited mid-start)", true)
		}
		return st, nil // the owner is still starting
	case DrainWaiting:
		return d.advanceWaiting(ctx, st)
	case DrainEvicting:
		return d.advanceEvicting(ctx, st)
	}
	if !start {
		return st, nil
	}
	return d.begin(ctx, node, st)
}

func (d *drainer) begin(ctx context.Context, node *corev1.Node, prev DrainState) (DrainState, error) {
	now := d.now()
	st := DrainState{Phase: DrainStarting, StartedAt: now, PhaseStartedAt: now}
	var cordon *bool
	switch {
	case !node.Spec.Unschedulable:
		st.CordonedByDrain = true
		cordon = new(true)
	case prev.Phase == DrainFailed && prev.CordonedByDrain:
		// A drain that failed during eviction left the node cordoned. That
		// cordon is still the toolkit's, not an operator's, so this drain
		// inherits it and may lift it again. A done drain's cordon is not
		// inherited: that node is presumably in maintenance.
		st.CordonedByDrain = true
	}
	if err := d.transition(ctx, prev, st, cordon); err != nil {
		return prev, err
	}
	note(ctx, false, "drain: started, node cordoned", "node", d.node, "cordonedByDrain", st.CordonedByDrain)

	pctx, cancel := context.WithTimeout(ctx, drainBeginTimeout)
	defer cancel()
	pods, err := podsOnNode(pctx, d.clientset, d.node)
	if err != nil {
		return d.fail(ctx, st, err.Error(), true)
	}
	if err := checkDrainBlockers(pods); err != nil {
		return d.fail(ctx, st, err.Error(), true)
	}
	plan, err := planRestarts(pctx, d.client, pods)
	if err != nil {
		return d.fail(ctx, st, err.Error(), true)
	}

	// Record the plan before acting on it: a driver that dies mid-restart
	// then times out and rolls back instead of restarting services twice.
	waiting := st
	waiting.Phase, waiting.PhaseStartedAt = DrainWaiting, d.now()
	for _, ref := range plan {
		waiting.Restarted = append(waiting.Restarted, ref.String())
	}
	if err := d.transition(pctx, st, waiting, nil); err != nil {
		if errors.Is(err, errDrainSuperseded) {
			return st, err // another driver rolled this start back
		}
		return d.fail(ctx, st, "record restart plan: "+err.Error(), true)
	}
	d.noteRestartPlan(ctx, waiting.Restarted)
	for _, ref := range plan {
		if err := patchRestartedAt(ctx, d.client, ref.api.gvr, ref.namespace, ref.name, drainNow()); err != nil {
			return d.fail(ctx, waiting, err.Error(), true)
		}
	}
	return d.advanceWaiting(ctx, waiting)
}

func (d *drainer) advanceWaiting(ctx context.Context, st DrainState) (DrainState, error) {
	pods, err := podsOnNode(ctx, d.clientset, d.node)
	if err != nil {
		return st, err
	}
	if remaining := remainingServices(st, pods); len(remaining) > 0 {
		if d.expired(st, drainRescheduleTimeout) {
			return d.fail(ctx, st, fmt.Sprintf("InferenceService pods still on node after %s (likely no spare GPU capacity elsewhere): %s; "+
				"their restarts were already triggered, so the rollouts finish wherever capacity allows, possibly back on this node",
				drainRescheduleTimeout, strings.Join(remaining, ", ")), true)
		}
		if d.shouldNote("waiting", strings.Join(remaining, ","), drainProgressEvery) {
			note(ctx, false, "drain: waiting for restarted InferenceServices to leave the node", "node", d.node,
				"remaining", remaining, "deadline", st.PhaseStartedAt.Add(drainRescheduleTimeout).Format(time.RFC3339))
		}
		return st, nil
	}
	evicting := st
	evicting.Phase, evicting.PhaseStartedAt = DrainEvicting, d.now()
	if err := d.transition(ctx, st, evicting, nil); err != nil {
		return st, err
	}
	note(ctx, false, "drain: evicting remaining pods", "node", d.node)
	return d.advanceEvicting(ctx, evicting)
}

// advanceEvicting issues one eviction round and reports done once only
// pods kubectl drain would leave behind (DaemonSet, mirror) remain.
func (d *drainer) advanceEvicting(ctx context.Context, st DrainState) (DrainState, error) {
	helper := d.evictHelper(ctx)
	list, errs := helper.GetPodsForDeletion(d.node)
	if list == nil {
		// The pod list itself failed: transient, retried next round.
		return st, fmt.Errorf("list pods to evict from node %s: %w", d.node, utilerrors.NewAggregate(errs))
	}
	if len(errs) > 0 {
		// Policy refusals (e.g. a pod no controller manages) never clear.
		return d.fail(ctx, st, "cannot evict pods: "+utilerrors.NewAggregate(errs).Error(), false)
	}
	pods := list.Pods()
	d.noteEvictionWarnings(ctx, list.Warnings(), pods)
	if len(pods) == 0 {
		done := st
		done.Phase, done.PhaseStartedAt = DrainDone, d.now()
		if err := d.transition(ctx, st, done, nil); err != nil {
			return st, err
		}
		note(ctx, false, "drain: done, node drained and left cordoned", "node", d.node)
		return done, nil
	}
	if d.expired(st, drainEvictTimeout) {
		return d.fail(ctx, st, fmt.Sprintf("pods still on node after %s (check their PodDisruptionBudgets): %s",
			drainEvictTimeout, strings.Join(podNames(pods), ", ")), false)
	}
	gv, err := checkEvictionSupport(d.clientset)
	if err != nil {
		return st, fmt.Errorf("check eviction support: %w", err)
	}
	names := podNames(pods)
	if d.shouldNote("evicting", strings.Join(names, ","), drainProgressEvery) {
		note(ctx, false, "drain: waiting for pods to leave the node", "node", d.node,
			"remaining", names, "deadline", st.PhaseStartedAt.Add(drainEvictTimeout).Format(time.RFC3339))
	}
	d.evictRound(ctx, helper, pods, gv)
	return st, nil
}

// evictRound evicts every pod not already terminating. A refused eviction
// (e.g. 429 from a PodDisruptionBudget) is retried next round until the
// phase times out.
func (d *drainer) evictRound(ctx context.Context, helper *drain.Helper, pods []corev1.Pod, gv schema.GroupVersion) {
	for _, pod := range pods {
		if pod.DeletionTimestamp != nil {
			continue // already terminating
		}
		name := pod.Namespace + "/" + pod.Name
		err := evictPod(helper, pod, gv)
		switch {
		case err == nil:
			note(ctx, false, "drain: evicted", "pod", name)
		case apierrors.IsNotFound(err):
		case d.shouldNote("deferred:"+name, err.Error(), drainProgressEvery):
			note(ctx, false, "drain: eviction deferred, will retry", "pod", name, "error", err)
		}
	}
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

// fail records the drain as failed. With rollback set, a node the drain
// cordoned is uncordoned in the same patch, and the record stops claiming
// a cordon: one found later is an operator's, which the next drain's
// rollback must keep.
func (d *drainer) fail(ctx context.Context, st DrainState, msg string, rollback bool) (DrainState, error) {
	next := st
	next.Phase, next.PhaseStartedAt = DrainFailed, d.now()
	var cordon *bool
	if rollback && st.CordonedByDrain {
		cordon = new(false)
		next.CordonedByDrain = false
		next.Message = msg + " (node uncordoned)"
	} else {
		next.Message = msg + " (node left cordoned)"
	}
	rctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), drainRollbackTimeout)
	defer cancel()
	if err := d.transition(rctx, st, next, cordon); err != nil {
		return st, fmt.Errorf("record failed drain (%s): %w", msg, err)
	}
	note(ctx, true, "drain: failed", "node", d.node, "reason", next.Message)
	return next, nil
}

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

func (d *drainer) evictHelper(ctx context.Context) *drain.Helper {
	logger := logging.FromContext(ctx)
	return &drain.Helper{
		Ctx:                 ctx,
		Client:              d.clientset,
		Out:                 logWriter{logger},
		ErrOut:              logWriter{logger},
		IgnoreAllDaemonSets: true,
		DeleteEmptyDirData:  true,
		GracePeriodSeconds:  -1, // Use pod's termination grace period
	}
}

func (d *drainer) now() time.Time {
	return drainNow().UTC().Truncate(time.Second)
}

func (d *drainer) expired(st DrainState, timeout time.Duration) bool {
	return drainNow().After(st.PhaseStartedAt.Add(timeout))
}

func podsOnNode(ctx context.Context, clientset kubernetes.Interface, nodeName string) ([]corev1.Pod, error) {
	list, err := clientset.CoreV1().Pods("").List(ctx, v1.ListOptions{
		FieldSelector: "spec.nodeName=" + nodeName,
	})
	if err != nil {
		return nil, fmt.Errorf("list pods on node %s: %w", nodeName, err)
	}
	return list.Items, nil
}

// podActive reports whether the pod still holds the node: not yet
// completed or failed.
func podActive(pod *corev1.Pod) bool {
	return pod.Status.Phase != corev1.PodSucceeded && pod.Status.Phase != corev1.PodFailed
}

// emptyDirPods names the pods that mount an emptyDir volume.
func emptyDirPods(pods []corev1.Pod) []string {
	var names []string
	for i := range pods {
		for _, vol := range pods[i].Spec.Volumes {
			if vol.EmptyDir != nil {
				names = append(names, pods[i].Namespace+"/"+pods[i].Name)
				break
			}
		}
	}
	return names
}

func podNames(pods []corev1.Pod) []string {
	names := make([]string, len(pods))
	for i := range pods {
		names[i] = pods[i].Namespace + "/" + pods[i].Name
	}
	return names
}

func checkDrainBlockers(pods []corev1.Pod) error {
	var blockers []string
	for i := range pods {
		pod := &pods[i]
		if !podActive(pod) {
			continue
		}
		for _, label := range drainBlockerLabels {
			if _, ok := pod.Labels[label]; ok {
				blockers = append(blockers, fmt.Sprintf("%s/%s (%s)", pod.Namespace, pod.Name, label))
				break
			}
		}
	}
	if len(blockers) > 0 {
		return fmt.Errorf("node runs pods that must not be drained: %s", strings.Join(blockers, ", "))
	}
	return nil
}

// servingRefs returns the distinct InferenceServices with live pods in
// pods, sorted for deterministic processing.
func servingRefs(pods []corev1.Pod) []isvcRef {
	seen := map[string]isvcRef{}
	for i := range pods {
		pod := &pods[i]
		if !podActive(pod) || pod.DeletionTimestamp != nil {
			continue
		}
		for j := range servingAPIs {
			api := &servingAPIs[j]
			if name := pod.Labels[api.label]; name != "" {
				ref := isvcRef{api: api, namespace: pod.Namespace, name: name}
				seen[ref.String()] = ref
				break
			}
		}
	}
	refs := make([]isvcRef, 0, len(seen))
	for _, ref := range seen {
		refs = append(refs, ref)
	}
	slices.SortFunc(refs, func(a, b isvcRef) int { return strings.Compare(a.String(), b.String()) })
	return refs
}

/*
planRestarts picks the InferenceServices on the node to restart: those
whose minReplicas is at most 1 (unset counts as 1). Multi-replica services
are left to eviction, their other replicas serving meanwhile; so is a
service whose CR is gone.
*/
func planRestarts(ctx context.Context, client dynamic.Interface, pods []corev1.Pod) ([]isvcRef, error) {
	var plan []isvcRef
	for _, ref := range servingRefs(pods) {
		obj, err := client.Resource(ref.api.gvr).Namespace(ref.namespace).Get(ctx, ref.name, v1.GetOptions{})
		if apierrors.IsNotFound(err) {
			note(ctx, true, "drain: InferenceService not found, leaving its pods to eviction", "service", ref.String())
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("get InferenceService %s: %w", ref, err)
		}
		spec, _ := obj.Object["spec"].(map[string]any)
		minReplicas, ok := nestedCount(spec, ref.api.minReplicasPath...)
		if !ok {
			minReplicas = 1
		}
		if minReplicas > 1 {
			note(ctx, false, "drain: leaving multi-replica InferenceService to eviction", "service", ref.String(), "minReplicas", minReplicas)
			continue
		}
		plan = append(plan, ref)
	}
	return plan, nil
}

// remainingServices lists the restarted InferenceServices that still have
// live pods on the node.
func remainingServices(st DrainState, pods []corev1.Pod) []string {
	var remaining []string
	for _, s := range st.Restarted {
		ref, ok := parseISVCRef(s)
		if !ok {
			continue
		}
		for i := range pods {
			pod := &pods[i]
			if podActive(pod) && pod.Namespace == ref.namespace && pod.Labels[ref.api.label] == ref.name {
				remaining = append(remaining, s)
				break
			}
		}
	}
	return remaining
}
