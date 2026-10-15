package k8s

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
	"k8s.io/kubectl/pkg/drain"
)

const drainNodeName = "gpu-1"

var (
	podsGVR  = schema.GroupVersionResource{Version: "v1", Resource: "pods"}
	nodesGVR = schema.GroupVersionResource{Version: "v1", Resource: "nodes"}
)

// isvcV2Single is a v1beta1 endpoint with one replica: drain must restart it.
const isvcV2Single = `
apiVersion: ome.io/v1beta1
kind: InferenceService
metadata:
  name: chat
  namespace: chat-ns
spec:
  engine:
    minReplicas: 1
`

// isvcV2Multi keeps serving through eviction on its other replicas.
const isvcV2Multi = `
apiVersion: ome.io/v1beta1
kind: InferenceService
metadata:
  name: embed
  namespace: embed-ns
spec:
  engine:
    minReplicas: 2
`

// isvcV1Unset is a v1alpha1 endpoint without minReplicas, which counts as 1.
const isvcV1Unset = `
apiVersion: ome.oracle.com/v1alpha1
kind: InferenceService
metadata:
  name: legacy
  namespace: legacy-ns
spec:
  predictor: {}
`

// isvcV1Multi keeps minReplicas under predictor, the v1alpha1 path.
const isvcV1Multi = `
apiVersion: ome.oracle.com/v1alpha1
kind: InferenceService
metadata:
  name: moderator
  namespace: moderator-ns
spec:
  predictor:
    minReplicas: 4
`

// drainPod builds a ReplicaSet-owned pod on the drained node; kubectl's
// eviction filters refuse controller-less pods.
func drainPod(name, namespace string, labels map[string]string, phase corev1.PodPhase) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: v1.ObjectMeta{
			Name: name, Namespace: namespace, Labels: labels,
			OwnerReferences: []v1.OwnerReference{{APIVersion: "apps/v1", Kind: "ReplicaSet", Name: name + "-rs", Controller: new(true)}},
		},
		Spec:   corev1.PodSpec{NodeName: drainNodeName},
		Status: corev1.PodStatus{Phase: phase},
	}
}

func servingPod(name, namespace, label, isvc string) *corev1.Pod {
	return drainPod(name, namespace, map[string]string{label: isvc}, corev1.PodRunning)
}

type drainFixture struct {
	t         *testing.T
	cs        *fake.Clientset
	client    *dynamicfake.FakeDynamicClient
	d         *drainer
	now       time.Time
	evicted   []string
	evictErr  error
	evictions int
}

// newDrainFixture fakes the cluster and the drain seams. Time stands
// still until advance moves it, and a successful eviction removes the pod
// at once, as if it terminated.
func newDrainFixture(t *testing.T, unschedulable bool, pods []*corev1.Pod, isvcs ...string) *drainFixture {
	t.Helper()
	cs := fake.NewSimpleClientset(&corev1.Node{
		ObjectMeta: v1.ObjectMeta{Name: drainNodeName},
		Spec:       corev1.NodeSpec{Unschedulable: unschedulable},
	})
	for _, pod := range pods {
		require.NoError(t, cs.Tracker().Add(pod))
	}
	objs := make([]runtime.Object, 0, len(isvcs))
	for _, src := range isvcs {
		objs = append(objs, parseYAML(t, src))
	}
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{
		inferenceServiceGVR:   "InferenceServiceList",
		inferenceServiceV1GVR: "InferenceServiceList",
	}, objs...)
	f := &drainFixture{
		t: t, cs: cs, client: client,
		d:   &drainer{clientset: cs, client: client, node: drainNodeName},
		now: time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC),
	}

	origNow, origEvict, origPoll := drainNow, evictPod, drainPollInterval
	t.Cleanup(func() { drainNow, evictPod, drainPollInterval = origNow, origEvict, origPoll })
	drainNow = func() time.Time { return f.now }
	drainPollInterval = time.Millisecond
	evictPod = func(_ *drain.Helper, pod corev1.Pod) error {
		f.evictions++
		if f.evictErr != nil {
			return f.evictErr
		}
		f.evicted = append(f.evicted, pod.Namespace+"/"+pod.Name)
		return cs.Tracker().Delete(podsGVR, pod.Namespace, pod.Name)
	}
	return f
}

func (f *drainFixture) advance(d time.Duration) { f.now = f.now.Add(d) }

func (f *drainFixture) deletePod(namespace, name string) {
	f.t.Helper()
	require.NoError(f.t, f.cs.Tracker().Delete(podsGVR, namespace, name))
}

func (f *drainFixture) node() *corev1.Node {
	f.t.Helper()
	node, err := f.cs.CoreV1().Nodes().Get(context.Background(), drainNodeName, v1.GetOptions{})
	require.NoError(f.t, err)
	return node
}

func (f *drainFixture) state() DrainState {
	f.t.Helper()
	return readDrainState(context.Background(), f.node())
}

// setAnnotation writes through the tracker, bypassing the reactor chain,
// so reactors may call it.
func (f *drainFixture) setAnnotation(value string) {
	f.t.Helper()
	obj, err := f.cs.Tracker().Get(nodesGVR, "", drainNodeName)
	require.NoError(f.t, err)
	node := obj.(*corev1.Node).DeepCopy()
	node.Annotations = map[string]string{DrainAnnotation: value}
	require.NoError(f.t, f.cs.Tracker().Update(nodesGVR, node, ""))
}

func (f *drainFixture) setState(st DrainState) {
	f.t.Helper()
	raw, err := json.Marshal(st)
	require.NoError(f.t, err)
	f.setAnnotation(string(raw))
}

func (f *drainFixture) step() DrainState {
	f.t.Helper()
	st, err := f.d.step(context.Background(), false)
	require.NoError(f.t, err)
	return st
}

// restarted lists the "<group>:<namespace>/<name>" of every
// InferenceService the drain patched.
func (f *drainFixture) restarted() []string {
	var got []string
	for _, action := range f.client.Actions() {
		if patch, ok := action.(k8stesting.PatchAction); ok {
			got = append(got, patch.GetResource().Group+":"+patch.GetNamespace()+"/"+patch.GetName())
		}
	}
	return got
}

func TestDrain_NoServicesDrainsToDone(t *testing.T) { //nolint:paralleltest // shared drain seams
	f := newDrainFixture(t, false, []*corev1.Pod{drainPod("worker", "default", nil, corev1.PodRunning)})

	status, err := f.d.start(context.Background())
	require.NoError(t, err)
	// Nothing to reschedule: the start falls straight through to eviction.
	assert.Equal(t, DrainEvicting, status.Phase)
	assert.True(t, status.CordonedByDrain)
	assert.Equal(t, []string{"default/worker"}, f.evicted)

	require.NoError(t, f.d.await(context.Background()))
	assert.Equal(t, DrainDone, f.state().Phase)
	assert.True(t, f.node().Spec.Unschedulable)
	assert.Empty(t, f.restarted())
}

func TestDrain_Blockers(t *testing.T) { //nolint:paralleltest // shared drain seams
	cases := []struct {
		name          string
		pod           *corev1.Pod
		unschedulable bool
		wantErr       string
	}{
		{
			name:    "fine-tuning pod",
			pod:     drainPod("ft-0", "ft", map[string]string{fineTuneLabel: "m"}, corev1.PodRunning),
			wantErr: "ft/ft-0 (finetunedmodel-name) (node uncordoned)",
		},
		{
			name:    "training job pod",
			pod:     drainPod("tj-0", "tj", map[string]string{trainingLabelV2: "j"}, corev1.PodPending),
			wantErr: "tj/tj-0 (ome.io/trainingjob) (node uncordoned)",
		},
		{
			name:    "replication job pod",
			pod:     drainPod("rj-0", "rj", map[string]string{replicationJobLabel: "r"}, corev1.PodRunning),
			wantErr: "rj/rj-0 (replicationJob) (node uncordoned)",
		},
		{
			name:          "already cordoned node stays cordoned",
			pod:           drainPod("ft-0", "ft", map[string]string{fineTuneLabel: "m"}, corev1.PodRunning),
			unschedulable: true,
			wantErr:       "ft/ft-0 (finetunedmodel-name) (node left cordoned)",
		},
	}
	for _, tc := range cases { //nolint:paralleltest // shared drain seams
		t.Run(tc.name, func(t *testing.T) {
			f := newDrainFixture(t, tc.unschedulable, []*corev1.Pod{tc.pod})

			status, err := f.d.start(context.Background())
			require.ErrorContains(t, err, "must not be drained")
			assert.ErrorContains(t, err, tc.wantErr)
			assert.Equal(t, DrainFailed, status.Phase)
			assert.Equal(t, tc.unschedulable, f.node().Spec.Unschedulable, "node must be left as found")
			assert.Zero(t, f.evictions)
		})
	}
}

func TestDrain_FinishedBlockerPodsDoNotBlock(t *testing.T) { //nolint:paralleltest // shared drain seams
	f := newDrainFixture(t, false, []*corev1.Pod{
		drainPod("ft-done", "ft", map[string]string{fineTuneLabel: "m"}, corev1.PodSucceeded),
		drainPod("rj-failed", "rj", map[string]string{replicationJobLabel: "r"}, corev1.PodFailed),
	})

	_, err := f.d.start(context.Background())
	require.NoError(t, err)
	require.NoError(t, f.d.await(context.Background()))
	assert.Equal(t, DrainDone, f.state().Phase)
}

func TestDrain_RestartsSingleReplicaServicesThenEvicts(t *testing.T) { //nolint:paralleltest // shared drain seams
	f := newDrainFixture(t, false, []*corev1.Pod{
		servingPod("chat-a", "chat-ns", servingLabelV2, "chat"),
		servingPod("embed-a", "embed-ns", servingLabelV2, "embed"),
		servingPod("legacy-a", "legacy-ns", servingLabelV1, "legacy"),
		servingPod("moderator-a", "moderator-ns", servingLabelV1, "moderator"),
		servingPod("gone-a", "gone-ns", servingLabelV2, "gone"),
	}, isvcV2Single, isvcV2Multi, isvcV1Unset, isvcV1Multi)

	status, err := f.d.start(context.Background())
	require.NoError(t, err)
	// Multi-replica and CR-less services are left to the eviction phase.
	want := []string{"ome.io:chat-ns/chat", "ome.oracle.com:legacy-ns/legacy"}
	assert.Equal(t, want, f.restarted())
	assert.Equal(t, DrainWaiting, status.Phase)
	assert.Equal(t, want, status.Restarted)
	assert.Equal(t, want, status.Remaining)
	require.NotNil(t, status.Deadline)
	assert.Equal(t, f.now.Add(drainRescheduleTimeout), *status.Deadline)

	// Still waiting while either restarted service has a pod on the node.
	f.deletePod("chat-ns", "chat-a")
	assert.Equal(t, DrainWaiting, f.step().Phase)
	assert.Zero(t, f.evictions)

	f.deletePod("legacy-ns", "legacy-a")
	assert.Equal(t, DrainEvicting, f.step().Phase)
	assert.ElementsMatch(t, []string{"embed-ns/embed-a", "moderator-ns/moderator-a", "gone-ns/gone-a"}, f.evicted)
	assert.Equal(t, DrainDone, f.step().Phase)
	assert.Len(t, f.restarted(), 2, "no service is restarted twice")
}

func TestDrain_RestartsEachServiceOnce(t *testing.T) { //nolint:paralleltest // shared drain seams
	f := newDrainFixture(t, false, []*corev1.Pod{
		servingPod("chat-a", "chat-ns", servingLabelV2, "chat"),
		servingPod("chat-b", "chat-ns", servingLabelV2, "chat"),
	}, isvcV2Single)

	_, err := f.d.start(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{"ome.io:chat-ns/chat"}, f.restarted())
}

func TestDrain_ResumeDoesNotRestartAgain(t *testing.T) { //nolint:paralleltest // shared drain seams
	f := newDrainFixture(t, false, []*corev1.Pod{servingPod("chat-a", "chat-ns", servingLabelV2, "chat")}, isvcV2Single)

	first, err := f.d.start(context.Background())
	require.NoError(t, err)
	// A second process starting the same drain resumes it instead.
	second, err := (&drainer{clientset: f.cs, client: f.client, node: drainNodeName}).start(context.Background())
	require.NoError(t, err)
	assert.Equal(t, DrainWaiting, second.Phase)
	assert.Equal(t, first.StartedAt, second.StartedAt)
	assert.Len(t, f.restarted(), 1)
}

func TestDrain_RescheduleTimeout(t *testing.T) { //nolint:paralleltest // shared drain seams
	for _, unschedulable := range []bool{false, true} {
		f := newDrainFixture(t, unschedulable, []*corev1.Pod{servingPod("chat-a", "chat-ns", servingLabelV2, "chat")}, isvcV2Single)
		_, err := f.d.start(context.Background())
		require.NoError(t, err)

		f.advance(drainRescheduleTimeout - time.Second)
		assert.Equal(t, DrainWaiting, f.step().Phase)

		f.advance(2 * time.Second)
		st := f.step()
		assert.Equal(t, DrainFailed, st.Phase)
		assert.Contains(t, st.Message, "no spare GPU capacity")
		assert.Contains(t, st.Message, "ome.io:chat-ns/chat")
		if unschedulable {
			assert.Contains(t, st.Message, "(node left cordoned)")
			assert.True(t, f.node().Spec.Unschedulable, "a node cordoned before the drain stays cordoned")
		} else {
			assert.Contains(t, st.Message, "(node uncordoned)")
			assert.False(t, f.node().Spec.Unschedulable)
		}
		assert.Zero(t, f.evictions)
	}
}

func TestDrain_EvictionTimeoutLeavesCordoned(t *testing.T) { //nolint:paralleltest // shared drain seams
	f := newDrainFixture(t, false, []*corev1.Pod{drainPod("guarded", "default", nil, corev1.PodRunning)})
	f.evictErr = apierrors.NewTooManyRequests("Cannot evict pod as it would violate the pod's disruption budget.", 10)

	status, err := f.d.start(context.Background())
	require.NoError(t, err)
	assert.Equal(t, DrainEvicting, status.Phase)
	assert.Equal(t, []string{"default/guarded"}, status.Remaining)

	f.advance(drainEvictTimeout + time.Second)
	st := f.step()
	assert.Equal(t, DrainFailed, st.Phase)
	assert.Contains(t, st.Message, "PodDisruptionBudgets): default/guarded (node left cordoned)")
	assert.True(t, f.node().Spec.Unschedulable)
}

func TestDrain_UnevictablePodLeavesCordoned(t *testing.T) { //nolint:paralleltest // shared drain seams
	bare := drainPod("bare", "default", nil, corev1.PodRunning)
	bare.OwnerReferences = nil
	f := newDrainFixture(t, false, []*corev1.Pod{bare})

	_, err := f.d.start(context.Background())
	require.ErrorContains(t, err, "cannot evict pods")
	assert.ErrorContains(t, err, "(node left cordoned)")
	assert.True(t, f.node().Spec.Unschedulable)
}

func TestDrain_StaleStartRollsBack(t *testing.T) { //nolint:paralleltest // shared drain seams
	f := newDrainFixture(t, true, nil)
	f.setState(DrainState{Phase: DrainStarting, StartedAt: f.now, PhaseStartedAt: f.now, CordonedByDrain: true})

	f.advance(drainStartingStale - time.Second)
	assert.Equal(t, DrainStarting, f.step().Phase, "a fresh start belongs to its owner")
	assert.True(t, f.node().Spec.Unschedulable)

	f.advance(2 * time.Second)
	st := f.step()
	assert.Equal(t, DrainFailed, st.Phase)
	assert.Contains(t, st.Message, "exited mid-start) (node uncordoned)")
	assert.False(t, f.node().Spec.Unschedulable)
}

func TestDrain_RetriesResourceVersionConflicts(t *testing.T) { //nolint:paralleltest // shared drain seams
	f := newDrainFixture(t, false, nil)
	conflicts := 0
	f.cs.PrependReactor("patch", "nodes", func(k8stesting.Action) (bool, runtime.Object, error) {
		if conflicts < 2 {
			conflicts++
			return true, nil, apierrors.NewConflict(schema.GroupResource{Resource: "nodes"}, drainNodeName, nil)
		}
		return false, nil, nil
	})

	_, err := f.d.start(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2, conflicts)
	assert.True(t, f.node().Spec.Unschedulable)
}

func TestDrain_ConcurrentStartYieldsToWinner(t *testing.T) { //nolint:paralleltest // shared drain seams
	f := newDrainFixture(t, false, []*corev1.Pod{servingPod("chat-a", "chat-ns", servingLabelV2, "chat")}, isvcV2Single)
	winner := DrainState{Phase: DrainStarting, StartedAt: f.now.Add(-time.Second), PhaseStartedAt: f.now.Add(-time.Second), CordonedByDrain: true}
	raced := false
	f.cs.PrependReactor("patch", "nodes", func(k8stesting.Action) (bool, runtime.Object, error) {
		if raced {
			return false, nil, nil
		}
		raced = true
		f.setState(winner) // another driver locks the node first
		return true, nil, apierrors.NewConflict(schema.GroupResource{Resource: "nodes"}, drainNodeName, nil)
	})

	status, err := f.d.start(context.Background())
	require.NoError(t, err)
	assert.Equal(t, DrainStarting, status.Phase)
	assert.Equal(t, winner.StartedAt, status.StartedAt)
	assert.Empty(t, f.restarted(), "the loser must not act")
}

func TestDrain_UnreadableAnnotationIsReplaced(t *testing.T) { //nolint:paralleltest // shared drain seams
	f := newDrainFixture(t, false, nil)
	f.setAnnotation("{not json")

	_, err := f.d.start(context.Background())
	require.NoError(t, err)
	assert.Equal(t, DrainDone, f.step().Phase)
}

func TestDrain_FinishedDrainStartsFresh(t *testing.T) { //nolint:paralleltest // shared drain seams
	f := newDrainFixture(t, true, nil)
	f.setState(DrainState{Phase: DrainFailed, StartedAt: f.now.Add(-time.Hour), Message: "old"})

	status, err := f.d.start(context.Background())
	require.NoError(t, err)
	assert.Equal(t, f.now, status.StartedAt)
	assert.Empty(t, status.Message)
	assert.False(t, status.CordonedByDrain)
}

func TestAwaitDrain_FailedDrainErrors(t *testing.T) { //nolint:paralleltest // shared drain seams
	f := newDrainFixture(t, false, nil)
	f.setState(DrainState{Phase: DrainFailed, Message: "boom (node uncordoned)"})
	assert.EqualError(t, f.d.await(context.Background()), "boom (node uncordoned)")
}

func TestAwaitDrain_NoDrainErrors(t *testing.T) { //nolint:paralleltest // shared drain seams
	f := newDrainFixture(t, false, nil)
	assert.ErrorContains(t, f.d.await(context.Background()), "no drain in progress")
}

func TestAwaitDrain_CanceledContextKeepsTheDrain(t *testing.T) { //nolint:paralleltest // shared drain seams
	f := newDrainFixture(t, false, []*corev1.Pod{servingPod("chat-a", "chat-ns", servingLabelV2, "chat")}, isvcV2Single)
	_, err := f.d.start(context.Background())
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	err = f.d.await(ctx)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.ErrorContains(t, err, "run drain again to resume")
	assert.Equal(t, DrainWaiting, f.state().Phase)
	assert.True(t, f.node().Spec.Unschedulable)
}

func TestAwaitDrain_PersistentAPIErrorsGiveUp(t *testing.T) { //nolint:paralleltest // shared drain seams
	f := newDrainFixture(t, false, nil)
	f.setState(DrainState{Phase: DrainWaiting, StartedAt: f.now, PhaseStartedAt: f.now})
	f.cs.PrependReactor("list", "pods", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewServiceUnavailable("apiserver down")
	})
	assert.ErrorContains(t, f.d.await(context.Background()), "apiserver down")
}

func TestDrainStatus_None(t *testing.T) { //nolint:paralleltest // shared drain seams
	f := newDrainFixture(t, false, nil)
	status, err := f.d.status(context.Background())
	require.NoError(t, err)
	assert.Equal(t, DrainNone, status.Phase)
	assert.Nil(t, status.Deadline)
	assert.False(t, f.node().Spec.Unschedulable, "status must not act")
}

func TestDrain_NodeNotFound(t *testing.T) { //nolint:paralleltest // shared drain seams
	f := newDrainFixture(t, false, nil)
	f.d.node = "missing"
	_, err := f.d.start(context.Background())
	require.Error(t, err)
}

func TestParseISVCRef(t *testing.T) {
	t.Parallel()
	for _, s := range []string{"ome.io:ns/name", "ome.oracle.com:ns/name"} {
		ref, ok := parseISVCRef(s)
		require.True(t, ok, s)
		assert.Equal(t, s, ref.String())
	}
	for _, s := range []string{"ome.io", "ome.io:ns", "other.io:ns/name"} {
		_, ok := parseISVCRef(s)
		assert.False(t, ok, s)
	}
}
