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

	"github.com/jingle2008/toolkit/pkg/infra/logging"
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
	drainPollInterval  = 10 * time.Second
	// drainMaxStepErrors is how many consecutive failed steps (API errors,
	// not drain failures) AwaitDrain tolerates before giving up.
	drainMaxStepErrors = 6
	// drainRollbackTimeout bounds recording a failure; it runs on a fresh
	// context so it survives the failed one being canceled.
	drainRollbackTimeout = 30 * time.Second
	drainNow             = time.Now
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

// evictPod evicts one pod, falling back to deletion where the cluster
// lacks the eviction API. A seam for tests.
var evictPod = func(helper *drain.Helper, pod corev1.Pod) error {
	gv, err := drain.CheckEvictionSupport(helper.Client)
	if err != nil {
		return err
	}
	if gv.Empty() {
		return helper.DeletePod(pod)
	}
	return helper.EvictPod(pod, gv)
}

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
		return DrainStatus{}, err
	}
	status, err := d.status(ctx)
	if err != nil {
		return DrainStatus{}, err
	}
	if st.Phase == DrainFailed {
		return status, errors.New(st.Message)
	}
	return status, nil
}

func (d *drainer) await(ctx context.Context) error {
	logger := logging.FromContext(ctx)
	streak := 0
	for {
		st, err := d.step(ctx, false)
		switch {
		case err != nil && ctx.Err() == nil:
			streak++
			logger.Warnw("drain: step failed", "node", d.node, "attempt", streak, "error", err)
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
// without a drain in progress begins a new one.
func (d *drainer) step(ctx context.Context, start bool) (DrainState, error) {
	for attempt := 0; ; attempt++ {
		st, err := d.stepOnce(ctx, start)
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
	logger := logging.FromContext(ctx)
	now := d.now()
	st := DrainState{Phase: DrainStarting, StartedAt: now, PhaseStartedAt: now, CordonedByDrain: !node.Spec.Unschedulable}
	var cordon *bool
	if st.CordonedByDrain {
		cordon = new(true)
	}
	if err := d.transition(ctx, prev, st, cordon); err != nil {
		return prev, err
	}
	logger.Infow("drain: started", "node", d.node, "cordoned", st.CordonedByDrain)

	pods, err := podsOnNode(ctx, d.clientset, d.node)
	if err != nil {
		return d.fail(ctx, st, err.Error(), true)
	}
	if err := checkDrainBlockers(pods); err != nil {
		return d.fail(ctx, st, err.Error(), true)
	}
	plan, err := planRestarts(ctx, d.client, pods)
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
	if err := d.transition(ctx, st, waiting, nil); err != nil {
		return st, err
	}
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
			return d.fail(ctx, st, fmt.Sprintf("InferenceService pods still on node after %s (likely no spare GPU capacity elsewhere; "+
				"their rollouts stay pending): %s", drainRescheduleTimeout, strings.Join(remaining, ", ")), true)
		}
		return st, nil
	}
	evicting := st
	evicting.Phase, evicting.PhaseStartedAt = DrainEvicting, d.now()
	if err := d.transition(ctx, st, evicting, nil); err != nil {
		return st, err
	}
	logging.FromContext(ctx).Infow("drain: evicting", "node", d.node)
	return d.advanceEvicting(ctx, evicting)
}

// advanceEvicting issues one eviction round and reports done once only
// pods kubectl drain would leave behind (DaemonSet, mirror) remain.
func (d *drainer) advanceEvicting(ctx context.Context, st DrainState) (DrainState, error) {
	logger := logging.FromContext(ctx)
	helper := d.evictHelper(ctx)
	list, errs := helper.GetPodsForDeletion(d.node)
	if len(errs) > 0 {
		return d.fail(ctx, st, "cannot evict pods: "+utilerrors.NewAggregate(errs).Error(), false)
	}
	pods := list.Pods()
	if len(pods) == 0 {
		done := st
		done.Phase, done.PhaseStartedAt = DrainDone, d.now()
		if err := d.transition(ctx, st, done, nil); err != nil {
			return st, err
		}
		logger.Infow("drain: done", "node", d.node)
		return done, nil
	}
	if d.expired(st, drainEvictTimeout) {
		return d.fail(ctx, st, fmt.Sprintf("pods still on node after %s (check their PodDisruptionBudgets): %s",
			drainEvictTimeout, strings.Join(podNames(pods), ", ")), false)
	}
	for _, pod := range pods {
		if pod.DeletionTimestamp != nil {
			continue // already terminating
		}
		// A refused eviction (e.g. 429 from a PodDisruptionBudget) is
		// retried next round until the phase times out.
		if err := evictPod(helper, pod); err != nil && !apierrors.IsNotFound(err) {
			logger.Infow("drain: eviction deferred", "pod", pod.Namespace+"/"+pod.Name, "error", err)
		}
	}
	return st, nil
}

// fail records the drain as failed. With rollback set, a node the drain
// cordoned is uncordoned in the same patch.
func (d *drainer) fail(ctx context.Context, st DrainState, msg string, rollback bool) (DrainState, error) {
	next := st
	next.Phase, next.PhaseStartedAt = DrainFailed, d.now()
	var cordon *bool
	if rollback && st.CordonedByDrain {
		cordon = new(false)
		next.Message = msg + " (node uncordoned)"
	} else {
		next.Message = msg + " (node left cordoned)"
	}
	rctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), drainRollbackTimeout)
	defer cancel()
	if err := d.transition(rctx, st, next, cordon); err != nil {
		return st, fmt.Errorf("record failed drain (%s): %w", msg, err)
	}
	logging.FromContext(ctx).Warnw("drain: failed", "node", d.node, "reason", next.Message)
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
		logging.FromContext(ctx).Warnw("drain: ignoring unreadable drain annotation", "node", node.Name, "value", raw, "error", err)
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
	logger := logging.FromContext(ctx)
	var plan []isvcRef
	for _, ref := range servingRefs(pods) {
		obj, err := client.Resource(ref.api.gvr).Namespace(ref.namespace).Get(ctx, ref.name, v1.GetOptions{})
		if apierrors.IsNotFound(err) {
			logger.Warnw("drain: InferenceService not found, leaving its pods to eviction", "service", ref.String())
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
			logger.Infow("drain: leaving multi-replica InferenceService to eviction", "service", ref.String(), "minReplicas", minReplicas)
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
