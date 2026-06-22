package k8s

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	cgotesting "k8s.io/client-go/testing"
)

// openFake returns an opener that hands back the given FakeWatcher.
func openFake(fw *watch.FakeWatcher) func(context.Context) (watch.Interface, error) {
	return func(context.Context) (watch.Interface, error) { return fw, nil }
}

func TestWatchTrigger_CoalescesBurst(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fw := watch.NewFake()
	window := 50 * time.Millisecond
	trig, err := watchTrigger(ctx, window, openFake(fw))
	require.NoError(t, err)

	// A burst of three events within one window must collapse to one tick.
	fed := make(chan struct{})
	go func() {
		fw.Add(nil)
		fw.Modify(nil)
		fw.Delete(nil)
		close(fed)
	}()

	select {
	case <-trig:
		// got the coalesced tick
	case <-time.After(time.Second):
		t.Fatal("expected a coalesced trigger tick")
	}

	// No second tick should arrive for the same burst.
	select {
	case <-trig:
		t.Fatal("burst should coalesce to a single tick")
	case <-time.After(3 * window):
	}

	// Ensure the injection goroutine has fully completed before cancel()/teardown
	// so that fw.Stop() (called by watchTrigger's stopper) cannot race with
	// fw.Delete() (the last send in the goroutine above).
	<-fed
}

func TestWatchTrigger_OpenerErrorStopsOthers(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fw := watch.NewFake()

	badOpener := func(context.Context) (watch.Interface, error) {
		return nil, assert.AnError
	}
	_, err := watchTrigger(ctx, 50*time.Millisecond, openFake(fw), badOpener)
	require.Error(t, err)
	assert.True(t, fw.IsStopped(), "already-opened watcher must be stopped on opener failure")
}

func TestWatchTrigger_CtxCancelClosesChannel(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	fw := watch.NewFake()
	trig, err := watchTrigger(ctx, 50*time.Millisecond, openFake(fw))
	require.NoError(t, err)

	cancel()

	select {
	case _, ok := <-trig:
		assert.False(t, ok, "channel must be closed after ctx cancel")
	case <-time.After(time.Second):
		t.Fatal("trigger channel should close after ctx cancel")
	}
	assert.True(t, fw.IsStopped(), "watcher must be stopped after ctx cancel")
}

func TestWatchTrigger_StreamDeathClosesChannel(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	fw := watch.NewFake()
	trig, err := watchTrigger(ctx, 50*time.Millisecond, openFake(fw))
	require.NoError(t, err)

	fw.Stop() // simulate the API server closing the stream (closes ResultChan)

	select {
	case _, ok := <-trig:
		assert.False(t, ok, "channel must close when an underlying stream dies")
	case <-time.After(time.Second):
		t.Fatal("trigger channel should close on stream death")
	}
}

// TestDurableOpener_ReconnectsOnStreamClose is the regression guard for the
// dropped live indicator: a durableOpener's watcher must NOT terminate when
// the underlying stream closes (the routine idle/proxy cut). RetryWatcher
// should reconnect — open a fresh underlying watch — and keep delivering
// events on the same channel. (RetryWatcher's restart delay is ~1s, so this
// test is intentionally not instant.)
//
//nolint:paralleltest // real-time RetryWatcher restart delay; keep serial for stable timing
func TestDurableOpener_ReconnectsOnStreamClose(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var mu sync.Mutex
	var fakes []*watch.FakeWatcher
	getFake := func(i int) *watch.FakeWatcher {
		mu.Lock()
		defer mu.Unlock()
		if i >= len(fakes) {
			return nil
		}
		return fakes[i]
	}
	waitForFake := func(n int) *watch.FakeWatcher {
		t.Helper()
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			if fw := getFake(n - 1); fw != nil {
				return fw
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatalf("underlying watch #%d was never opened (no reconnect)", n)
		return nil
	}

	opener := durableOpener(
		func(context.Context) (string, error) { return "1", nil },
		func(context.Context, metav1.ListOptions) (watch.Interface, error) {
			fw := watch.NewFake()
			mu.Lock()
			fakes = append(fakes, fw)
			mu.Unlock()
			return fw, nil
		},
	)

	w, err := opener(ctx)
	require.NoError(t, err)
	defer w.Stop()

	first := waitForFake(1)
	first.Stop() // simulate the API server / proxy closing the idle stream

	// RetryWatcher must open a SECOND underlying watch (reconnect).
	second := waitForFake(2)

	// An event on the reconnected stream must still reach the consumer, and
	// the channel must not have closed.
	go second.Add(&corev1.Pod{ObjectMeta: metav1.ObjectMeta{ResourceVersion: "2"}})
	select {
	case _, ok := <-w.ResultChan():
		require.True(t, ok, "watcher channel must stay open across reconnect")
	case <-time.After(3 * time.Second):
		t.Fatal("expected an event after reconnect")
	}
}

// TestDurableOpener_WatchesFromListedResourceVersion guards the "start from
// now" contract: durableOpener must list for the current resourceVersion and
// resume the watch from it, so reconnecting never replays the whole collection
// as ADDED events. If the RV threading broke, the watch would receive an empty
// ResourceVersion and replay everything.
//
//nolint:paralleltest // RetryWatcher opens the watch from a background goroutine
func TestDurableOpener_WatchesFromListedResourceVersion(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	gotRV := make(chan string, 1)
	opener := durableOpener(
		func(context.Context) (string, error) { return "12345", nil },
		func(_ context.Context, opts metav1.ListOptions) (watch.Interface, error) {
			select {
			case gotRV <- opts.ResourceVersion:
			default:
			}
			return watch.NewFake(), nil
		},
	)
	w, err := opener(ctx)
	require.NoError(t, err)
	defer w.Stop()

	select {
	case rv := <-gotRV:
		assert.Equal(t, "12345", rv, "watch must resume from the resourceVersion returned by the initial list")
	case <-time.After(2 * time.Second):
		t.Fatal("watch was never opened")
	}
}

// TestWatchGPUNodes_AppliesSelectors guards selector propagation: the node
// watch must carry the GPU node label selector, and every pod watch must carry
// one bounded GPU pod label selector plus the Running field selector. A
// refactor dropping `opts.LabelSelector = sel` would silently turn these into a
// cluster-wide firehose — this test fails if that happens.
//
//nolint:paralleltest // RetryWatcher opens watches from background goroutines
func TestWatchGPUNodes_AppliesSelectors(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cs := fake.NewSimpleClientset()

	nodeSel := make(chan string, 1)
	cs.PrependWatchReactor("nodes", func(action cgotesting.Action) (bool, watch.Interface, error) {
		r := action.(cgotesting.WatchAction).GetWatchRestrictions()
		select {
		case nodeSel <- r.Labels.String():
		default:
		}
		return true, watch.NewFake(), nil
	})

	var mu sync.Mutex
	type podRestriction struct{ labels, fields string }
	var podRs []podRestriction
	cs.PrependWatchReactor("pods", func(action cgotesting.Action) (bool, watch.Interface, error) {
		r := action.(cgotesting.WatchAction).GetWatchRestrictions()
		mu.Lock()
		podRs = append(podRs, podRestriction{r.Labels.String(), r.Fields.String()})
		mu.Unlock()
		return true, watch.NewFake(), nil
	})

	_, err := WatchGPUNodes(ctx, cs)
	require.NoError(t, err)

	select {
	case got := <-nodeSel:
		assert.Equal(t, gpuNodeSelector, got, "node watch must carry the GPU node label selector")
	case <-time.After(2 * time.Second):
		t.Fatal("nodes watch never opened")
	}

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(podRs) >= len(gpuPodSelectors)
	}, 2*time.Second, 10*time.Millisecond, "expected one pod watch per GPU pod selector")

	mu.Lock()
	defer mu.Unlock()
	wantLabels := make(map[string]bool, len(gpuPodSelectors))
	for _, s := range gpuPodSelectors {
		wantLabels[s] = true
	}
	for _, r := range podRs {
		assert.Equal(t, runningPodSelector, r.fields, "pod watch must be scoped to Running pods")
		assert.Truef(t, wantLabels[r.labels], "pod watch label selector %q is not one of the bounded gpuPodSelectors", r.labels)
	}
}

//nolint:paralleltest // mutates package-global DebounceWindow; must not run in parallel
func TestWatchGPUNodes_FiresOnNodeEvent(t *testing.T) {
	// Not parallel: this test writes the package-level DebounceWindow global.
	old := DebounceWindow
	DebounceWindow = 50 * time.Millisecond
	defer func() { DebounceWindow = old }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cs := fake.NewSimpleClientset()
	trig, err := WatchGPUNodes(ctx, cs)
	require.NoError(t, err)

	// Creating a node produces an Added event on the Nodes watch.
	_, err = cs.CoreV1().Nodes().Create(ctx, &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "gpu-1",
			Labels: map[string]string{"nvidia.com/gpu.present": "true"},
		},
	}, metav1.CreateOptions{})
	require.NoError(t, err)

	select {
	case <-trig:
	case <-time.After(time.Second):
		t.Fatal("expected a trigger tick after node create")
	}
}

//nolint:paralleltest // mutates package-global DebounceWindow; must not run in parallel
func TestWatchBaseModels_FiresOnCREvent(t *testing.T) {
	// Not parallel: this test writes the package-level DebounceWindow global.
	old := DebounceWindow
	DebounceWindow = 50 * time.Millisecond
	defer func() { DebounceWindow = old }()

	ctx, cancel := context.WithCancel(context.Background())
	// Do NOT defer cancel() here — we call it explicitly after observing the
	// tick so that teardown (Stop) cannot race with the Create's internal send.

	gvr := schema.GroupVersionResource{Group: "ome.io", Version: "v1beta1", Resource: "clusterbasemodels"}
	scheme := runtime.NewScheme()
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme,
		map[schema.GroupVersionResource]string{
			gvr: "ClusterBaseModelList",
			{Group: "ome.io", Version: "v1beta1", Resource: "basemodels"}: "BaseModelList",
		})

	trig, err := WatchBaseModels(ctx, client)
	require.NoError(t, err)

	_, err = client.Resource(gvr).Create(ctx, newCBM("m1", nil, nil, nil, nil), metav1.CreateOptions{})
	require.NoError(t, err)

	select {
	case <-trig:
	case <-time.After(time.Second):
		cancel()
		t.Fatal("expected a trigger tick after CR create")
	}

	// Cancel after observing the tick, then drain until trig closes to let
	// watchTrigger's internal goroutines finish cleanly.
	cancel()
	for range trig { //nolint:revive // intentional: drain until watchTrigger closes the channel
	}
}
