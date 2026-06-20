package k8s

import (
	"context"
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
	go func() {
		fw.Add(nil)
		fw.Modify(nil)
		fw.Delete(nil)
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

func TestWatchGPUNodes_FiresOnNodeEvent(t *testing.T) {
	t.Parallel()
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

func TestWatchBaseModels_FiresOnCREvent(t *testing.T) {
	t.Parallel()
	old := DebounceWindow
	DebounceWindow = 50 * time.Millisecond
	defer func() { DebounceWindow = old }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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
		t.Fatal("expected a trigger tick after CR create")
	}
}
