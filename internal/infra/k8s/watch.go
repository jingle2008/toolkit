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
	watchtools "k8s.io/client-go/tools/watch"

	"github.com/jingle2008/toolkit/pkg/infra/logging"
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
// stopAll stops every watcher in the slice.
func stopAll(watchers []watch.Interface) {
	for _, w := range watchers {
		w.Stop()
	}
}

func watchTrigger(
	ctx context.Context,
	window time.Duration,
	openers ...func(context.Context) (watch.Interface, error),
) (<-chan struct{}, error) {
	watchers := make([]watch.Interface, 0, len(openers))
	for _, open := range openers {
		w, err := open(ctx)
		if err != nil {
			stopAll(watchers)
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

	logging.FromContext(ctx).Infow("watch established", "watchers", len(watchers))

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
			logging.FromContext(ctx).Debugw("watch fan-in stopped: context canceled")
			return
		case <-done:
			return
		case _, ok := <-w.ResultChan():
			if !ok {
				// The watcher's channel closed. With the RetryWatcher-backed
				// openers this no longer happens on routine server-side closes
				// (those auto-reconnect); reaching here means RetryWatcher gave
				// up on an unrecoverable error (e.g. 410 Expired) or was
				// stopped. Treat it as a fallback signal: the live indicator
				// drops and the caller does a final one-shot reload.
				logging.FromContext(ctx).Warnw("watch ended (retry exhausted or stopped); live watch will drop")
				closeDone()
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
	stopAll(watchers)
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
				// out is unbuffered: this send blocks until the consumer's
				// waitForTrigger receives, so a coalesced tick is held (never
				// dropped), unlike the raw event channel which intentionally coalesces.
			case <-ctx.Done():
				return
			case <-done:
				return
			}
		}
	}
}

// watcherFunc adapts a plain watch function to cache.WatcherWithContext, the
// interface RetryWatcher reconnects through. RetryWatcher passes its
// lifecycle context and the resume resourceVersion (in opts) on each
// (re)connect; the func re-applies any caller selectors on top.
type watcherFunc func(context.Context, metav1.ListOptions) (watch.Interface, error)

func (f watcherFunc) WatchWithContext(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	return f(ctx, opts)
}

// durableOpener builds an opener whose watch survives transient stream
// closes — idle timeouts, proxy/LB cuts, API-server connection resets. It
// first lists for the collection's current resourceVersion (so the watch
// starts "from now": no initial ADDED replay), then returns a RetryWatcher
// that resumes from the last-seen resourceVersion whenever the stream
// closes. The watcher's channel only closes on ctx cancel or an
// unrecoverable error (e.g. 410 Expired) — not on the routine server-side
// closes that previously dropped the live indicator.
func durableOpener(
	listRV func(context.Context) (string, error),
	watchFn func(context.Context, metav1.ListOptions) (watch.Interface, error),
) func(context.Context) (watch.Interface, error) {
	return func(ctx context.Context) (watch.Interface, error) {
		rv, err := listRV(ctx)
		if err != nil {
			return nil, err
		}
		return watchtools.NewRetryWatcherWithContext(ctx, rv, watcherFunc(watchFn))
	}
}

// crWatchOpener returns a durable opener that watches all objects of one GVR.
func crWatchOpener(client dynamic.Interface, gvr schema.GroupVersionResource) func(context.Context) (watch.Interface, error) {
	return durableOpener(
		func(ctx context.Context) (string, error) {
			l, err := client.Resource(gvr).List(ctx, metav1.ListOptions{Limit: 1})
			if err != nil {
				return "", err
			}
			return l.GetResourceVersion(), nil
		},
		func(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
			return client.Resource(gvr).Watch(ctx, opts)
		},
	)
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
		openers = append(openers, durableOpener(
			func(ctx context.Context) (string, error) {
				l, err := clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{
					LabelSelector: sel,
					FieldSelector: runningPodSelector,
					Limit:         1,
				})
				if err != nil {
					return "", err
				}
				return l.ResourceVersion, nil
			},
			func(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
				opts.LabelSelector = sel
				opts.FieldSelector = runningPodSelector
				return clientset.CoreV1().Pods("").Watch(ctx, opts)
			},
		))
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
	nodeOpener := durableOpener(
		func(ctx context.Context) (string, error) {
			l, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{
				LabelSelector: gpuNodeSelector,
				Limit:         1,
			})
			if err != nil {
				return "", err
			}
			return l.ResourceVersion, nil
		},
		func(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
			opts.LabelSelector = gpuNodeSelector
			return clientset.CoreV1().Nodes().Watch(ctx, opts)
		},
	)
	openers := []func(context.Context) (watch.Interface, error){nodeOpener}
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
