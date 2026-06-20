package k8s

import (
	"context"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

// DebounceWindow is the coalescing window for watch triggers: events
// observed within one window collapse to a single reload tick. It is a
// package-level var so tests can shorten it. ~5s matches the TUI's
// "eventual" freshness target without re-listing on every raw event.
var DebounceWindow = 5 * time.Second

// watchTrigger opens every watcher via the given openers and merges
// their events into a single coalesced trigger channel. Each received
// value on the returned channel means "something changed; reload now".
//
// The watch is a TRIGGER, not a data source — event bodies are
// discarded. The caller owns ctx; cancelling it stops all watchers and
// closes the returned channel. The channel also closes if any
// underlying stream dies (the API server closing the connection), which
// the caller treats as a fallback signal.
//
// If any opener returns an error, all already-opened watchers are
// stopped and the error is returned with no channel.
func watchTrigger(
	ctx context.Context,
	window time.Duration,
	openers ...func(context.Context) (watch.Interface, error),
) (<-chan struct{}, error) {
	watchers := make([]watch.Interface, 0, len(openers))
	for _, open := range openers {
		w, err := open(ctx)
		if err != nil {
			for _, prev := range watchers {
				prev.Stop()
			}
			return nil, err
		}
		watchers = append(watchers, w)
	}

	// done is closed when any stream dies; signals fallback to callers.
	done := make(chan struct{})
	var once sync.Once
	closeDone := func() { once.Do(func() { close(done) }) }

	// raw carries one signal per observed event (buffered so a fan-in
	// goroutine never blocks while the coalescer is mid-timer).
	raw := make(chan struct{}, 1)

	var wg sync.WaitGroup
	for _, w := range watchers {
		wg.Add(1)
		go func(w watch.Interface) {
			defer wg.Done()
			fanInWatcher(ctx, w, raw, done, closeDone)
		}(w)
	}

	// stopped is closed after all watchers are stopped and fan-in goroutines exit.
	stopped := make(chan struct{})

	go stopWatchers(ctx, done, watchers, &wg, stopped)

	out := make(chan struct{})
	go coalesce(ctx, window, raw, done, stopped, out)

	return out, nil
}

// fanInWatcher forwards events from a single watcher into raw. It exits when
// ctx is cancelled, done is closed, or the watcher's ResultChan closes (stream
// death). On stream death it calls closeDone so the other goroutines shut down.
func fanInWatcher(
	ctx context.Context,
	w watch.Interface,
	raw chan<- struct{},
	done <-chan struct{},
	closeDone func(),
) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		case _, ok := <-w.ResultChan():
			if !ok {
				closeDone() // stream died
				return
			}
			select {
			case raw <- struct{}{}:
			default: // a signal is already pending; coalesce
			}
		}
	}
}

// stopWatchers waits for ctx cancellation or done, then stops all watchers,
// waits for all fan-in goroutines to exit, and closes stopped.
func stopWatchers(
	ctx context.Context,
	done <-chan struct{},
	watchers []watch.Interface,
	wg *sync.WaitGroup,
	stopped chan<- struct{},
) {
	select {
	case <-ctx.Done():
	case <-done:
	}
	for _, w := range watchers {
		w.Stop()
	}
	wg.Wait()
	close(stopped)
}

// coalesce debounces raw signals into out using window. It exits the debounce
// loop when ctx is cancelled or done is closed, then waits for stopped before
// closing out so callers observe consistent shutdown state.
func coalesce(
	ctx context.Context,
	window time.Duration,
	raw <-chan struct{},
	done <-chan struct{},
	stopped <-chan struct{},
	out chan<- struct{},
) {
	// Wait for watchers to be stopped before closing out, so callers that
	// check fw.IsStopped() immediately after receiving the closed channel
	// observe consistent state.
	defer func() {
		<-stopped
		close(out)
	}()
	var timerC <-chan time.Time
	for {
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		case <-raw:
			if timerC == nil {
				timerC = time.After(window)
			}
		case <-timerC:
			timerC = nil
			select {
			case out <- struct{}{}:
			case <-ctx.Done():
				return
			case <-done:
				return
			}
		}
	}
}

// crWatchOpener returns an opener that watches all objects of one GVR.
func crWatchOpener(client dynamic.Interface, gvr schema.GroupVersionResource) func(context.Context) (watch.Interface, error) {
	return func(ctx context.Context) (watch.Interface, error) {
		return client.Resource(gvr).Watch(ctx, metav1.ListOptions{})
	}
}

// gpuPodWatchOpeners returns one pod-watch opener per GPU pod label
// selector, scoped to Running pods. This mirrors the bounded selectors
// the allocation path lists with (gpuPodSelectors + runningPodSelector),
// so the trigger reacts to the GPU pods that drive node allocation,
// node issues, and DAC replica stats — without a cluster-wide pod
// firehose. A GPU-consuming pod outside these selectors will not trigger
// a reload until a watched resource also changes (acceptable for the
// eventual-freshness target).
func gpuPodWatchOpeners(clientset kubernetes.Interface) []func(context.Context) (watch.Interface, error) {
	openers := make([]func(context.Context) (watch.Interface, error), 0, len(gpuPodSelectors))
	for _, sel := range gpuPodSelectors {
		openers = append(openers, func(ctx context.Context) (watch.Interface, error) {
			return clientset.CoreV1().Pods("").Watch(ctx, metav1.ListOptions{
				LabelSelector: sel,
				FieldSelector: runningPodSelector,
			})
		})
	}
	return openers
}

var (
	clusterBaseModelGVR = schema.GroupVersionResource{Group: "ome.io", Version: "v1beta1", Resource: "clusterbasemodels"}
	baseModelGVR        = schema.GroupVersionResource{Group: "ome.io", Version: "v1beta1", Resource: "basemodels"}
	dacV1GVR            = schema.GroupVersionResource{Group: "ome.oracle.com", Version: "v1alpha1", Resource: "dedicatedaiclusters"}
	dacV2GVR            = schema.GroupVersionResource{Group: "ome.io", Version: "v1beta1", Resource: "dedicatedaiclusters"}
)

// WatchBaseModels triggers on ClusterBaseModel CR changes.
func WatchBaseModels(ctx context.Context, client dynamic.Interface) (<-chan struct{}, error) {
	return watchTrigger(ctx, DebounceWindow, crWatchOpener(client, clusterBaseModelGVR))
}

// WatchImportedModels triggers on namespaced BaseModel and
// ClusterBaseModel CR changes (the two sources LoadImportedModels merges).
func WatchImportedModels(ctx context.Context, client dynamic.Interface) (<-chan struct{}, error) {
	return watchTrigger(ctx, DebounceWindow,
		crWatchOpener(client, baseModelGVR),
		crWatchOpener(client, clusterBaseModelGVR),
	)
}

// WatchGPUNodes triggers on Node changes plus GPU pod changes (pods
// drive allocation and node issues).
func WatchGPUNodes(ctx context.Context, clientset kubernetes.Interface) (<-chan struct{}, error) {
	openers := []func(context.Context) (watch.Interface, error){
		func(ctx context.Context) (watch.Interface, error) {
			return clientset.CoreV1().Nodes().Watch(ctx, metav1.ListOptions{
				LabelSelector: "nvidia.com/gpu.present=true",
			})
		},
	}
	openers = append(openers, gpuPodWatchOpeners(clientset)...)
	return watchTrigger(ctx, DebounceWindow, openers...)
}

// WatchGPUWorkloads triggers on GPU pod changes.
func WatchGPUWorkloads(ctx context.Context, clientset kubernetes.Interface) (<-chan struct{}, error) {
	return watchTrigger(ctx, DebounceWindow, gpuPodWatchOpeners(clientset)...)
}

// WatchDedicatedAIClusters triggers on DAC CR changes (both API
// versions) plus GPU pod changes (pods drive replica stats).
func WatchDedicatedAIClusters(ctx context.Context, client dynamic.Interface, clientset kubernetes.Interface) (<-chan struct{}, error) {
	openers := []func(context.Context) (watch.Interface, error){
		crWatchOpener(client, dacV1GVR),
		crWatchOpener(client, dacV2GVR),
	}
	openers = append(openers, gpuPodWatchOpeners(clientset)...)
	return watchTrigger(ctx, DebounceWindow, openers...)
}
