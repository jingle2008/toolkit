# Live k8s Category Watches Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the five Kubernetes-backed TUI categories (BaseModel, ImportedModel, GPUNode, GPUWorkload, DedicatedAICluster) update on their own while in view, falling back to the existing one-shot load when a watch can't be established.

**Architecture:** A Kubernetes watch acts only as a *trigger*. A new `internal/infra/k8s/watch.go` opens `Watch()` streams on each category's resources, fans them into one goroutine that coalesces events over a debounce window and emits `struct{}` ticks on a channel. A new optional `loader.Watcher` interface (type-asserted at the call site, mirroring `TenantMetadataWriter`) exposes these as `(<-chan struct{}, error)`. In the TUI, entering a watched category issues `tea.Batch(loadXxxCmd, startWatchCmd)`; each trigger tick re-runs the existing `loadXxxCmd`, so all enrichment/grouping/handlers are reused unchanged.

**Tech Stack:** Go, `k8s.io/client-go` v0.36.2 (typed `kubernetes.Interface` + `dynamic.Interface`, `k8s.io/apimachinery/.../watch`), Bubble Tea (`github.com/charmbracelet/bubbletea`), lipgloss, testify, `client-go/kubernetes/fake` + `client-go/dynamic/fake`.

## Global Constraints

- Watch is a trigger only; never translate watch events into domain models — always re-run the existing `LoadXxx` loader on a trigger.
- Scope: exactly the five `domain.Category` values where `NeedsKubeConfig()` is true — BaseModel, ImportedModel, GPUNode, GPUWorkload, DedicatedAICluster.
- Freshness target: eventual (~5–10s). Debounce window default `5 * time.Second`, injectable for tests.
- Fallback: any watch setup failure, unsupported watcher, or mid-stream death falls back to the one-shot load. No auto-reconnect.
- All async messages carry `Gen int`; the reducer drops a message when `gen != m.gen` (gen 0 is never dropped — but watch messages always carry a real, non-zero gen).
- `Watcher` is an OPTIONAL capability, NOT embedded in `loader.Composite` (mirrors `TenantMetadataWriter`); callers type-assert and degrade gracefully.
- Watch goroutines are bound to a `context.Context`; cancelling it (via `m.newLoadContext()` on navigation) must stop all streams and close the trigger channel — no leaked goroutines.
- GPU pod triggers use the existing `gpuPodSelectors` label selectors + `runningPodSelector` field selector (bounded), not a cluster-wide pod firehose.

> **Spec deviation (intentional):** The spec said `Watcher` is "folded into `Composite`". During planning we found the codebase keeps optional loader capabilities out of `Composite` on purpose (interfaces.go:98-110, `TenantMetadataWriter`) so test fakes need not implement them. This plan makes `Watcher` optional/type-asserted instead. The spec's loader section is updated to match.

---

### Task 1: k8s watch-trigger fan-in helper

**Files:**
- Create: `internal/infra/k8s/watch.go`
- Test: `internal/infra/k8s/watch_test.go`

**Interfaces:**
- Consumes: `k8s.io/apimachinery/pkg/watch` (`watch.Interface`), `context`, `time`, `sync`.
- Produces:
  - `var DebounceWindow = 5 * time.Second`
  - `func watchTrigger(ctx context.Context, window time.Duration, openers ...func(context.Context) (watch.Interface, error)) (<-chan struct{}, error)` — opens all watchers (on any opener error, stops already-opened ones and returns the error), fans their events into one goroutine, coalesces events seen within `window` into a single `struct{}` tick, and returns the trigger channel. The channel is closed when `ctx` is cancelled or any underlying stream closes (mid-stream death).

- [ ] **Step 1: Write the failing test**

```go
package k8s

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/watch"
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/infra/k8s/ -run TestWatchTrigger -v`
Expected: FAIL — `undefined: watchTrigger`.

- [ ] **Step 3: Write minimal implementation**

```go
package k8s

import (
	"context"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/watch"
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
		}(w)
	}

	// Stop every watcher once ctx is cancelled or a stream dies.
	go func() {
		select {
		case <-ctx.Done():
		case <-done:
		}
		for _, w := range watchers {
			w.Stop()
		}
		wg.Wait()
	}()

	out := make(chan struct{})
	go func() {
		defer close(out)
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
	}()

	return out, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/infra/k8s/ -run TestWatchTrigger -v`
Expected: PASS (all four subtests).

- [ ] **Step 5: Commit**

```bash
git add internal/infra/k8s/watch.go internal/infra/k8s/watch_test.go
git commit -m "feat(k8s): add coalescing watch-trigger fan-in helper"
```

---

### Task 2: Per-category watch functions

**Files:**
- Modify: `internal/infra/k8s/watch.go`
- Test: `internal/infra/k8s/watch_test.go`

**Interfaces:**
- Consumes: `watchTrigger` and `DebounceWindow` (Task 1); existing package vars `gpuPodSelectors` (pod_query.go:29), `runningPodSelector` (pod_query.go:19); `dynamic.Interface`, `kubernetes.Interface`.
- Produces:
  - `func WatchBaseModels(ctx context.Context, client dynamic.Interface) (<-chan struct{}, error)`
  - `func WatchImportedModels(ctx context.Context, client dynamic.Interface) (<-chan struct{}, error)`
  - `func WatchGPUNodes(ctx context.Context, clientset kubernetes.Interface) (<-chan struct{}, error)`
  - `func WatchGPUWorkloads(ctx context.Context, clientset kubernetes.Interface) (<-chan struct{}, error)`
  - `func WatchDedicatedAIClusters(ctx context.Context, client dynamic.Interface, clientset kubernetes.Interface) (<-chan struct{}, error)`

- [ ] **Step 1: Write the failing test**

```go
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
```

Add these imports to `watch_test.go`:

```go
import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/infra/k8s/ -run 'TestWatchGPUNodes_FiresOnNodeEvent|TestWatchBaseModels_FiresOnCREvent' -v`
Expected: FAIL — `undefined: WatchGPUNodes` / `undefined: WatchBaseModels`.

- [ ] **Step 3: Write minimal implementation**

Append to `internal/infra/k8s/watch.go` (and add the imports `metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"`, `"k8s.io/apimachinery/pkg/runtime/schema"`, `"k8s.io/client-go/dynamic"`, `"k8s.io/client-go/kubernetes"`, `"k8s.io/apimachinery/pkg/watch"` to the file's import block):

```go
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
		sel := sel
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/infra/k8s/ -run TestWatch -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/infra/k8s/watch.go internal/infra/k8s/watch_test.go
git commit -m "feat(k8s): add per-category watch-trigger functions"
```

---

### Task 3: Optional loader.Watcher interface + production impl

**Files:**
- Modify: `internal/infra/loader/interfaces.go` (add `Watcher` interface; do NOT embed in `Composite`)
- Modify: `internal/infra/loader/production/production.go` (implement the five methods)
- Test: `internal/infra/loader/production/production_watch_test.go` (create)

**Interfaces:**
- Consumes: `k8s.WatchBaseModels`/`WatchImportedModels`/`WatchGPUNodes`/`WatchGPUWorkloads`/`WatchDedicatedAIClusters` (Task 2); existing `k8s.NewClientsetFromKubeConfig`/`NewDynamicClientFromKubeConfig` (client.go); `env.KubeContext()`.
- Produces: `loader.Watcher` interface and its `*production.Client` implementation:
  - `WatchBaseModels(ctx, kubeCfg string, env models.Environment) (<-chan struct{}, error)`
  - `WatchImportedModels(ctx, kubeCfg string, env models.Environment) (<-chan struct{}, error)`
  - `WatchGPUNodes(ctx, kubeCfg string, env models.Environment) (<-chan struct{}, error)`
  - `WatchGPUWorkloads(ctx, kubeCfg string, env models.Environment) (<-chan struct{}, error)`
  - `WatchDedicatedAIClusters(ctx, kubeCfg string, env models.Environment) (<-chan struct{}, error)`

- [ ] **Step 1: Add the interface (no test yet — interface-only)**

Append to `internal/infra/loader/interfaces.go`, right after the `TenantMetadataWriter` block:

```go
/*
Watcher is an OPTIONAL capability: establishing Kubernetes watches that
emit a coalesced "reload now" signal for the k8s-backed categories. Like
TenantMetadataWriter it is deliberately kept out of Composite so the many
fake loaders used in tests need not implement it. Callers type-assert a
Composite to this interface and fall back to a one-shot load when the
assertion fails or a method returns an error.

Each method returns a channel that yields one value whenever the
category's underlying resources change (debounced). The caller owns ctx;
cancelling it stops the watch and closes the channel. The channel also
closes if the stream dies, which the caller treats as a fallback signal.
*/
type Watcher interface {
	WatchBaseModels(ctx context.Context, kubeCfg string, env models.Environment) (<-chan struct{}, error)
	WatchImportedModels(ctx context.Context, kubeCfg string, env models.Environment) (<-chan struct{}, error)
	WatchGPUNodes(ctx context.Context, kubeCfg string, env models.Environment) (<-chan struct{}, error)
	WatchGPUWorkloads(ctx context.Context, kubeCfg string, env models.Environment) (<-chan struct{}, error)
	WatchDedicatedAIClusters(ctx context.Context, kubeCfg string, env models.Environment) (<-chan struct{}, error)
}
```

- [ ] **Step 2: Write the failing test**

Create `internal/infra/loader/production/production_watch_test.go`:

```go
package production

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/jingle2008/toolkit/internal/infra/loader"
)

func TestClient_ImplementsWatcher(t *testing.T) {
	t.Parallel()
	var _ loader.Watcher = (*Client)(nil)
	assert.Implements(t, (*loader.Watcher)(nil), &Client{})
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/infra/loader/production/ -run TestClient_ImplementsWatcher -v`
Expected: FAIL — `*Client does not implement loader.Watcher (missing method WatchBaseModels)`.

- [ ] **Step 4: Write minimal implementation**

Append to `internal/infra/loader/production/production.go` (the `k8s` and `context`/`models` imports already exist):

```go
// Compile-time guard: *Client must satisfy the optional Watcher
// interface, kept out of Composite (see loader.Watcher docs).
var _ loader.Watcher = (*Client)(nil)

// WatchBaseModels establishes a watch on ClusterBaseModel CRs.
func (Client) WatchBaseModels(ctx context.Context, kubeCfg string, env models.Environment) (<-chan struct{}, error) {
	client, err := k8s.NewDynamicClientFromKubeConfig(kubeCfg, env.KubeContext())
	if err != nil {
		return nil, err
	}
	return k8s.WatchBaseModels(ctx, client)
}

// WatchImportedModels establishes a watch on the imported-model sources.
func (Client) WatchImportedModels(ctx context.Context, kubeCfg string, env models.Environment) (<-chan struct{}, error) {
	client, err := k8s.NewDynamicClientFromKubeConfig(kubeCfg, env.KubeContext())
	if err != nil {
		return nil, err
	}
	return k8s.WatchImportedModels(ctx, client)
}

// WatchGPUNodes establishes a watch on GPU nodes and GPU pods.
func (Client) WatchGPUNodes(ctx context.Context, kubeCfg string, env models.Environment) (<-chan struct{}, error) {
	cs, err := k8s.NewClientsetFromKubeConfig(kubeCfg, env.KubeContext())
	if err != nil {
		return nil, err
	}
	return k8s.WatchGPUNodes(ctx, cs)
}

// WatchGPUWorkloads establishes a watch on GPU pods.
func (Client) WatchGPUWorkloads(ctx context.Context, kubeCfg string, env models.Environment) (<-chan struct{}, error) {
	cs, err := k8s.NewClientsetFromKubeConfig(kubeCfg, env.KubeContext())
	if err != nil {
		return nil, err
	}
	return k8s.WatchGPUWorkloads(ctx, cs)
}

// WatchDedicatedAIClusters establishes a watch on DAC CRs and GPU pods.
func (Client) WatchDedicatedAIClusters(ctx context.Context, kubeCfg string, env models.Environment) (<-chan struct{}, error) {
	dyn, err := k8s.NewDynamicClientFromKubeConfig(kubeCfg, env.KubeContext())
	if err != nil {
		return nil, err
	}
	cs, err := k8s.NewClientsetFromKubeConfig(kubeCfg, env.KubeContext())
	if err != nil {
		return nil, err
	}
	return k8s.WatchDedicatedAIClusters(ctx, dyn, cs)
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/infra/loader/production/ -run TestClient_ImplementsWatcher -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/infra/loader/interfaces.go internal/infra/loader/production/production.go internal/infra/loader/production/production_watch_test.go
git commit -m "feat(loader): add optional Watcher capability + production impl"
```

---

### Task 4: TUI watch messages and commands

**Files:**
- Modify: `internal/ui/tui/messages.go`
- Modify: `internal/ui/tui/loader_cmd.go`
- Test: `internal/ui/tui/loader_cmd_watch_test.go` (create)

**Interfaces:**
- Consumes: `loader.Watcher` (Task 3); `domain.Category`; existing `loader.Composite` field type.
- Produces:
  - Messages: `watchStartedMsg{Cat domain.Category; Trigger <-chan struct{}; Gen int}`, `watchTriggeredMsg{Cat domain.Category; Gen int}`, `watchClosedMsg{Cat domain.Category; Gen int}`, `watchUnavailableMsg{Cat domain.Category; Gen int}`.
  - `func startWatchCmd(ctx context.Context, ld loader.Composite, cat domain.Category, kubeCfg string, env models.Environment, gen int) tea.Cmd`
  - `func waitForTriggerCmd(cat domain.Category, trigger <-chan struct{}, gen int) tea.Cmd`

- [ ] **Step 1: Add message types**

Append to `internal/ui/tui/messages.go`:

```go
// watchStartedMsg signals that a category watch is live; Trigger yields
// a value on each debounced cluster change.
type watchStartedMsg struct {
	Cat     domain.Category
	Trigger <-chan struct{}
	Gen     int
}

// watchTriggeredMsg signals one debounced change; the reducer re-runs
// the category's loader and re-arms the listener.
type watchTriggeredMsg struct {
	Cat domain.Category
	Gen int
}

// watchClosedMsg signals the trigger channel closed (ctx cancel or
// stream death); the reducer falls back to a final one-shot load.
type watchClosedMsg struct {
	Cat domain.Category
	Gen int
}

// watchUnavailableMsg signals watch setup failed or is unsupported; the
// static load result stays on screen, no live indicator.
type watchUnavailableMsg struct {
	Cat domain.Category
	Gen int
}
```

- [ ] **Step 2: Write the failing test**

Create `internal/ui/tui/loader_cmd_watch_test.go`:

```go
package tui

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/pkg/models"
)

// watchableLoader is a fakeLoader that also implements loader.Watcher.
type watchableLoader struct {
	loaderStub // existing minimal Composite stub used in TUI tests
	trigger    chan struct{}
	err        error
}

func (w *watchableLoader) WatchGPUNodes(_ context.Context, _ string, _ models.Environment) (<-chan struct{}, error) {
	if w.err != nil {
		return nil, w.err
	}
	return w.trigger, nil
}

func TestStartWatchCmd_SuccessEmitsStarted(t *testing.T) {
	t.Parallel()
	trig := make(chan struct{}, 1)
	ld := &watchableLoader{trigger: trig}
	cmd := startWatchCmd(context.Background(), ld, domain.GPUNode, "kc", models.Environment{}, 7)
	require.NotNil(t, cmd)

	msg := cmd()
	started, ok := msg.(watchStartedMsg)
	require.True(t, ok, "expected watchStartedMsg, got %T", msg)
	assert.Equal(t, domain.GPUNode, started.Cat)
	assert.Equal(t, 7, started.Gen)
	assert.NotNil(t, started.Trigger)
}

func TestStartWatchCmd_UnsupportedEmitsUnavailable(t *testing.T) {
	t.Parallel()
	// plainLoader implements Composite but NOT loader.Watcher.
	ld := plainLoader{}
	cmd := startWatchCmd(context.Background(), ld, domain.GPUNode, "kc", models.Environment{}, 3)
	msg := cmd()
	unavail, ok := msg.(watchUnavailableMsg)
	require.True(t, ok, "expected watchUnavailableMsg, got %T", msg)
	assert.Equal(t, 3, unavail.Gen)
}

func TestWaitForTriggerCmd_TickEmitsTriggered(t *testing.T) {
	t.Parallel()
	trig := make(chan struct{}, 1)
	trig <- struct{}{}
	cmd := waitForTriggerCmd(domain.GPUNode, trig, 5)
	msg := cmd()
	triggered, ok := msg.(watchTriggeredMsg)
	require.True(t, ok, "expected watchTriggeredMsg, got %T", msg)
	assert.Equal(t, 5, triggered.Gen)
}

func TestWaitForTriggerCmd_ClosedEmitsClosed(t *testing.T) {
	t.Parallel()
	trig := make(chan struct{})
	close(trig)
	cmd := waitForTriggerCmd(domain.GPUNode, trig, 9)
	msg := cmd()
	_, ok := msg.(watchClosedMsg)
	require.True(t, ok, "expected watchClosedMsg, got %T", msg)
}
```

> **Note for the implementer:** `loaderStub`/`plainLoader` are the existing in-package test doubles that satisfy `loader.Composite`. Reuse whichever the TUI tests already define (grep `loader.Composite` in `internal/ui/tui/*_test.go`). If a `Composite`-only stub does not yet exist, add a minimal one named `plainLoader` that embeds the existing fake. Do NOT add the Watch methods to it — its purpose is to exercise the "unsupported" path.

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/ui/tui/ -run 'TestStartWatchCmd|TestWaitForTriggerCmd' -v`
Expected: FAIL — `undefined: startWatchCmd` / `undefined: waitForTriggerCmd`.

- [ ] **Step 4: Write minimal implementation**

Append to `internal/ui/tui/loader_cmd.go`. Add `loader` is already imported; the `tea`, `context`, `domain`, `models` imports exist.

```go
// startWatchCmd type-asserts the loader to loader.Watcher and starts the
// watch for cat. On success it returns watchStartedMsg with the trigger
// channel; if the loader doesn't support watching or setup fails, it
// returns watchUnavailableMsg so the caller keeps the one-shot load
// result with no live indicator.
func startWatchCmd(ctx context.Context, ld loader.Composite, cat domain.Category, kubeCfg string, env models.Environment, gen int) tea.Cmd {
	return func() tea.Msg {
		w, ok := ld.(loader.Watcher)
		if !ok {
			return watchUnavailableMsg{Cat: cat, Gen: gen}
		}
		var (
			trigger <-chan struct{}
			err     error
		)
		switch cat {
		case domain.BaseModel:
			trigger, err = w.WatchBaseModels(ctx, kubeCfg, env)
		case domain.ImportedModel:
			trigger, err = w.WatchImportedModels(ctx, kubeCfg, env)
		case domain.GPUNode:
			trigger, err = w.WatchGPUNodes(ctx, kubeCfg, env)
		case domain.GPUWorkload:
			trigger, err = w.WatchGPUWorkloads(ctx, kubeCfg, env)
		case domain.DedicatedAICluster:
			trigger, err = w.WatchDedicatedAIClusters(ctx, kubeCfg, env)
		default:
			return watchUnavailableMsg{Cat: cat, Gen: gen}
		}
		if err != nil {
			return watchUnavailableMsg{Cat: cat, Gen: gen}
		}
		return watchStartedMsg{Cat: cat, Trigger: trigger, Gen: gen}
	}
}

// waitForTriggerCmd blocks (in the tea runtime's goroutine) on one value
// from the trigger channel: a tick → watchTriggeredMsg, a close →
// watchClosedMsg.
func waitForTriggerCmd(cat domain.Category, trigger <-chan struct{}, gen int) tea.Cmd {
	return func() tea.Msg {
		if _, ok := <-trigger; !ok {
			return watchClosedMsg{Cat: cat, Gen: gen}
		}
		return watchTriggeredMsg{Cat: cat, Gen: gen}
	}
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/ui/tui/ -run 'TestStartWatchCmd|TestWaitForTriggerCmd' -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/ui/tui/messages.go internal/ui/tui/loader_cmd.go internal/ui/tui/loader_cmd_watch_test.go
git commit -m "feat(tui): add watch messages and start/wait commands"
```

---

### Task 5: Reducer handling for watch messages + `watching` state

**Files:**
- Modify: `internal/ui/tui/model_state.go` (add `watching bool` field)
- Modify: `internal/ui/tui/model_reducer.go` (add `handleWatch*` reducers)
- Modify: `internal/ui/tui/model_update.go` (route the four watch messages)
- Test: `internal/ui/tui/reducer_watch_test.go` (create)

**Interfaces:**
- Consumes: `watchStartedMsg`/`watchTriggeredMsg`/`watchClosedMsg`/`watchUnavailableMsg` (Task 4); `waitForTriggerCmd` (Task 4); existing `m.gen`, `m.loadCtx`, `m.loader`, `m.kubeConfig`, `m.environment`; existing per-category `loadXxxCmd` constructors (loader_cmd.go).
- Produces:
  - `m.watching bool`
  - `func (m *Model) handleWatchStarted(msg watchStartedMsg) tea.Cmd`
  - `func (m *Model) handleWatchTriggered(msg watchTriggeredMsg) tea.Cmd`
  - `func (m *Model) handleWatchClosed(msg watchClosedMsg) tea.Cmd`
  - `func (m *Model) handleWatchUnavailable(msg watchUnavailableMsg)`
  - `func (m *Model) reloadCategoryCmd(cat domain.Category, gen int) tea.Cmd` — maps a category to its existing `loadXxxCmd`.

- [ ] **Step 1: Add the `watching` field**

In `internal/ui/tui/model_state.go`, immediately after the `gen int` field (line 92), add:

```go
	// watching is true while a live k8s watch is active for the current
	// category; drives the status-bar live indicator. Reset on every
	// category change and cleared on watch fallback.
	watching bool
```

- [ ] **Step 2: Write the failing test**

Create `internal/ui/tui/reducer_watch_test.go`:

```go
package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
)

func TestHandleWatchStarted_SetsWatchingAndArms(t *testing.T) {
	t.Parallel()
	m := newTestModel(t) // existing TUI test constructor
	m.gen = 4
	m.category = domain.GPUNode
	trig := make(chan struct{}, 1)

	cmd := m.handleWatchStarted(watchStartedMsg{Cat: domain.GPUNode, Trigger: trig, Gen: 4})
	assert.True(t, m.watching)
	require.NotNil(t, cmd, "must re-arm the trigger listener")
}

func TestHandleWatchStarted_StaleIgnored(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.gen = 5
	trig := make(chan struct{}, 1)

	cmd := m.handleWatchStarted(watchStartedMsg{Cat: domain.GPUNode, Trigger: trig, Gen: 2})
	assert.False(t, m.watching, "stale watchStartedMsg must not enable watching")
	assert.Nil(t, cmd)
}

func TestHandleWatchTriggered_ReloadsAndRearms(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.gen = 3
	m.category = domain.GPUNode

	cmd := m.handleWatchTriggered(watchTriggeredMsg{Cat: domain.GPUNode, Gen: 3})
	require.NotNil(t, cmd, "trigger must produce reload + re-arm cmds")
}

func TestHandleWatchTriggered_StaleIgnored(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.gen = 8
	cmd := m.handleWatchTriggered(watchTriggeredMsg{Cat: domain.GPUNode, Gen: 1})
	assert.Nil(t, cmd)
}

func TestHandleWatchClosed_ClearsWatchingAndReloads(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.gen = 2
	m.category = domain.GPUNode
	m.watching = true

	cmd := m.handleWatchClosed(watchClosedMsg{Cat: domain.GPUNode, Gen: 2})
	assert.False(t, m.watching, "closed watch clears the live indicator")
	require.NotNil(t, cmd, "closed watch issues one final reload")
}

func TestHandleWatchUnavailable_LeavesWatchingFalse(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.gen = 2
	m.watching = false
	m.handleWatchUnavailable(watchUnavailableMsg{Cat: domain.GPUNode, Gen: 2})
	assert.False(t, m.watching)
}
```

> **Note for the implementer:** `newTestModel(t)` stands for the existing TUI test helper that builds a minimal `*Model`. Grep `func newTestModel` / existing `_test.go` constructors in `internal/ui/tui/` and use the real one; if helpers build the model differently (e.g. `newModelForTest`), match that name.

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/ui/tui/ -run 'TestHandleWatch' -v`
Expected: FAIL — `m.handleWatchStarted undefined` (and the others).

- [ ] **Step 4: Write minimal implementation**

Add to `internal/ui/tui/model_reducer.go`:

```go
// reloadCategoryCmd returns the existing one-shot load command for a
// watched category. Used both for the trigger-driven refresh and the
// final reload when a watch dies. Returns nil for non-watched categories.
func (m *Model) reloadCategoryCmd(cat domain.Category, gen int) tea.Cmd {
	switch cat {
	case domain.BaseModel:
		return loadBaseModelsCmd(m.loadCtx, m.loader, m.kubeConfig, m.environment, gen)
	case domain.ImportedModel:
		return loadImportedModelsCmd(m.loadCtx, m.loader, m.kubeConfig, m.environment, gen)
	case domain.GPUNode:
		return loadGPUNodesCmd(m.loadCtx, m.loader, m.kubeConfig, m.environment, gen)
	case domain.GPUWorkload:
		return loadGPUWorkloadsCmd(m.loadCtx, m.loader, m.kubeConfig, m.environment, gen)
	case domain.DedicatedAICluster:
		return loadDedicatedAIClustersCmd(m.loadCtx, m.loader, m.kubeConfig, m.environment, gen)
	default:
		return nil
	}
}

// handleWatchStarted marks the category live and arms the trigger
// listener. A stale gen (the user already navigated away) is ignored;
// the watch goroutine is already being torn down via loadCtx cancel.
func (m *Model) handleWatchStarted(msg watchStartedMsg) tea.Cmd {
	if msg.Gen != m.gen {
		return nil
	}
	m.watching = true
	return waitForTriggerCmd(msg.Cat, msg.Trigger, msg.Gen)
}

// handleWatchTriggered re-runs the category loader and re-arms the
// listener so subsequent changes keep flowing.
func (m *Model) handleWatchTriggered(msg watchTriggeredMsg) tea.Cmd {
	if msg.Gen != m.gen {
		return nil
	}
	reload := m.reloadCategoryCmd(msg.Cat, msg.Gen)
	if reload == nil {
		return nil
	}
	return tea.Batch(m.beginTask(), reload, m.waitForTrigger(msg.Cat, msg.Gen))
}

// handleWatchClosed falls back to a final one-shot load and clears the
// live indicator (no auto-reconnect).
func (m *Model) handleWatchClosed(msg watchClosedMsg) tea.Cmd {
	if msg.Gen != m.gen {
		return nil
	}
	m.watching = false
	reload := m.reloadCategoryCmd(msg.Cat, msg.Gen)
	if reload == nil {
		return nil
	}
	return tea.Batch(m.beginTask(), reload)
}

// handleWatchUnavailable records that no live watch is active. The
// static load result remains on screen.
func (m *Model) handleWatchUnavailable(msg watchUnavailableMsg) {
	if msg.Gen != m.gen {
		return
	}
	m.watching = false
}
```

`handleWatchTriggered` references a re-arm helper that does not capture the trigger channel directly (the channel lives only in the `watchStartedMsg`/the running `waitForTriggerCmd`). Re-arming must reuse the SAME channel. Store it on the model. Add to `model_state.go` next to `watching`:

```go
	// watchTrigger is the active category's trigger channel; held so a
	// watchTriggeredMsg can re-arm the listener on the same stream.
	watchTrigger <-chan struct{}
```

Set it in `handleWatchStarted` and add the helper:

```go
// in handleWatchStarted, before returning:
	m.watchTrigger = msg.Trigger
```

```go
// waitForTrigger re-arms the listener on the stored trigger channel.
func (m *Model) waitForTrigger(cat domain.Category, gen int) tea.Cmd {
	if m.watchTrigger == nil {
		return nil
	}
	return waitForTriggerCmd(cat, m.watchTrigger, gen)
}
```

Now route the messages. In `internal/ui/tui/model_update.go`, add cases to the top-level `Update` switch (next to the other data/loaded cases, around line 50):

```go
	case watchStartedMsg:
		return m, m.handleWatchStarted(msg)
	case watchTriggeredMsg:
		return m, m.handleWatchTriggered(msg)
	case watchClosedMsg:
		return m, m.handleWatchClosed(msg)
	case watchUnavailableMsg:
		m.handleWatchUnavailable(msg)
		return m, nil
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/ui/tui/ -run 'TestHandleWatch' -v`
Expected: PASS.

- [ ] **Step 6: Reset `watching` on category change**

In `internal/ui/tui/reducer_category.go`, inside `updateCategoryCore`, in the `else` branch that runs when the category actually changes (the block starting at line 37 `} else {`), add after `m.showFaulty = false`:

```go
		m.watching = false
		m.watchTrigger = nil
```

This clears the live indicator the instant the user navigates away; the old watch goroutine is independently torn down by the existing `m.newLoadContext()` call (loadCtx cancel), and any late watch message is dropped by the gen check.

- [ ] **Step 7: Run the package tests**

Run: `go test ./internal/ui/tui/ -run 'TestHandleWatch|TestUpdateCategory' -v`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/ui/tui/model_state.go internal/ui/tui/model_reducer.go internal/ui/tui/model_update.go internal/ui/tui/reducer_category.go internal/ui/tui/reducer_watch_test.go
git commit -m "feat(tui): reduce watch messages and track live state"
```

---

### Task 6: Start the watch on category entry

**Files:**
- Modify: `internal/ui/tui/reducer_category.go` (the five `handleXxxCategory` for k8s categories)
- Test: `internal/ui/tui/reducer_category_watch_test.go` (create)

**Interfaces:**
- Consumes: `startWatchCmd` (Task 4); existing `loadXxxCmd` constructors; `tea.Batch`.
- Produces: the five `handleXxxCategory` now return `tea.Batch(<existing load cmd>, startWatchCmd(...))`.

- [ ] **Step 1: Write the failing test**

Create `internal/ui/tui/reducer_category_watch_test.go`:

```go
package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
)

// collectMsgs runs a (possibly batched) cmd and returns the message
// types it produced. tea.Batch returns a BatchMsg of sub-commands.
func collectMsgTypes(t *testing.T, cmd tea.Cmd) []string {
	t.Helper()
	if cmd == nil {
		return nil
	}
	var types []string
	msg := cmd()
	switch m := msg.(type) {
	case tea.BatchMsg:
		for _, c := range m {
			if c == nil {
				continue
			}
			types = append(types, msgTypeName(c()))
		}
	default:
		types = append(types, msgTypeName(msg))
	}
	return types
}

func TestHandleGPUNodeCategory_BatchesLoadAndWatch(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.newLoadContext()
	m.dataset = nil // force a load

	cmd := m.handleGPUNodeCategory(true, m.bumpGen())
	require.NotNil(t, cmd)
	types := collectMsgTypes(t, cmd)
	// Expect both a load result and a watch lifecycle message.
	assert.Contains(t, types, "tui.gpuNodesLoadedMsg")
	assert.True(t,
		contains(types, "tui.watchStartedMsg") || contains(types, "tui.watchUnavailableMsg"),
		"expected a watch lifecycle message, got %v", types)
}
```

> **Note for the implementer:** `msgTypeName` and `contains` are tiny test helpers — add them to this file if not already present:
> ```go
> func msgTypeName(msg tea.Msg) string { return fmt.Sprintf("%T", msg) }
> func contains(s []string, v string) bool { for _, x := range s { if x == v { return true } }; return false }
> ```
> (add `"fmt"` to imports). The fake loader used by `newTestModel` must return a value for `LoadGPUNodesByPool`; reuse the existing fake. Whether the watch yields `watchStartedMsg` or `watchUnavailableMsg` depends on whether that fake implements `loader.Watcher` — the assertion accepts either.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ui/tui/ -run TestHandleGPUNodeCategory_BatchesLoadAndWatch -v`
Expected: FAIL — the current `handleGPUNodeCategory` returns only the load cmd (no watch message in the batch).

- [ ] **Step 3: Write minimal implementation**

In `internal/ui/tui/reducer_category.go`, replace the five k8s-category handlers so each batches its existing load cmd with `startWatchCmd`. Example for `handleGPUNodeCategory` (lines 148-153):

```go
func (m *Model) handleGPUNodeCategory(refresh bool, gen int) tea.Cmd {
	watch := startWatchCmd(m.loadCtx, m.loader, domain.GPUNode, m.kubeConfig, m.environment, gen)
	if m.dataset == nil || m.dataset.GPUNodeMap == nil || refresh {
		return tea.Batch(loadGPUNodesCmd(m.loadCtx, m.loader, m.kubeConfig, m.environment, gen), watch)
	}
	// Cached: paint stays as-is, but still go live.
	return watch
}
```

Apply the same shape to the other four:
- `handleBaseModelCategory` → load `loadBaseModelsCmd`, `domain.BaseModel`
- `handleImportedModelCategory` → `loadImportedModelsCmd`, `domain.ImportedModel`
- `handleGPUWorkloadCategory` → `loadGPUWorkloadsCmd`, `domain.GPUWorkload`
- `handleDedicatedAIClusterCategory` → `loadDedicatedAIClustersCmd`, `domain.DedicatedAICluster`

Each returns `tea.Batch(<load>, watch)` on cache-miss/refresh, and `watch` alone when cached. `GPUPool` and the override/tenant handlers are unchanged (not k8s-watched).

Ensure `domain` and `tea` are imported in `reducer_category.go` (both already are).

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/ui/tui/ -run TestHandleGPUNodeCategory_BatchesLoadAndWatch -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/ui/tui/reducer_category.go internal/ui/tui/reducer_category_watch_test.go
git commit -m "feat(tui): start k8s category watch on entry alongside load"
```

---

### Task 7: Status-bar live indicator

**Files:**
- Modify: `internal/ui/tui/styles.go` (add a `Live` style)
- Modify: `internal/ui/tui/model_view.go` (render the live cell when `m.watching`)
- Test: `internal/ui/tui/model_view_watch_test.go` (create)

**Interfaces:**
- Consumes: `m.watching` (Task 5); existing `Styles` struct + `statusView` composition (model_view.go:60-105).
- Produces: a `Live lipgloss.Style` and a live cell rendered into the status bar.

- [ ] **Step 1: Write the failing test**

Create `internal/ui/tui/model_view_watch_test.go`:

```go
package tui

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStatusView_ShowsLiveIndicatorWhenWatching(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.viewWidth = 120
	m.viewHeight = 40
	m.updateLayout(m.viewWidth, m.viewHeight)

	m.watching = false
	off := m.statusView() // call the existing status-bar render method
	m.watching = true
	on := m.statusView()

	assert.NotEqual(t, off, on, "live indicator must change the status bar")
	assert.True(t, strings.Contains(on, "LIVE") || strings.Contains(on, "●"),
		"expected a live marker in the status bar, got %q", on)
}
```

> **Note for the implementer:** the status-bar render method is the one containing the `loadingCell` logic (model_view.go:60-105). Confirm its exact name (grep `loadingCell` / `func (m *Model)` in `model_view.go`) and call that from the test instead of `statusView()` if it differs.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ui/tui/ -run TestStatusView_ShowsLiveIndicatorWhenWatching -v`
Expected: FAIL — no live marker in output (and possibly `m.statusView undefined` if the method name differs; fix the call per the note, then it fails on the assertion).

- [ ] **Step 3: Write minimal implementation**

In `internal/ui/tui/styles.go`, add a field to `Styles`:

```go
	Live lipgloss.Style
```

and in `DefaultStyles()` build it (near `stats`, ~line 44) and include it in the returned struct:

```go
	live := statusNugget.
		Background(lipgloss.Color("#2EA043")). // green = live
		Bold(true)
```

```go
		Live: live,
```

In `internal/ui/tui/model_view.go`, in the status-bar render method, add a live cell next to `loadingCell` (after line 88):

```go
	liveCell := ""
	if m.watching {
		liveCell = m.styles.Live.Render("● LIVE")
	}
```

Include it in the width budget and the join. Update the `inputWidth` calc (line 94) to subtract `w(liveCell)`:

```go
	inputWidth := max(m.viewWidth-w(contextCell)-w(loadingCell)-w(liveCell)-w(statsCell)-
		w(m.textInput.Prompt)-1, 0)
```

and add `liveCell` to the `JoinHorizontal` (between `loadingCell` and `statsCell`, line 100-105):

```go
	return lipgloss.JoinHorizontal(lipgloss.Top,
		contextCell,
		inputCell,
		loadingCell,
		liveCell,
		statsCell,
	)
```

> If `m.styles` is the `table.Styles` (a different type) and the TUI `Styles` set lives elsewhere on the model, render with the TUI `Styles` field instead (grep how `m` accesses `Context`/`Stats` styles in this method). The cell content (`"● LIVE"`) is what the test asserts; the exact style wiring follows the existing pattern for `statsCell`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/ui/tui/ -run TestStatusView_ShowsLiveIndicatorWhenWatching -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/ui/tui/styles.go internal/ui/tui/model_view.go internal/ui/tui/model_view_watch_test.go
git commit -m "feat(tui): show live indicator in status bar while watching"
```

---

### Task 8: Full verification + lint

**Files:** none (verification only)

- [ ] **Step 1: Run the full test suite**

Run: `go test ./...`
Expected: PASS (no regressions in k8s, loader, or tui packages).

- [ ] **Step 2: Run the linter / vet**

Run: `go vet ./...` and the project linter (check `Makefile`/`.golangci.yml`; run `golangci-lint run` if configured).
Expected: clean. Pay attention to: goroutine-leak/`containedctx` lint on the new context-bound goroutines (mirror the existing `//nolint:containedctx` annotation pattern used on `loadCtx`), and cyclomatic-complexity on `Update` (it already carries a `//nolint:cyclop`; if adding four cases trips a threshold, extend the existing annotation rather than restructuring).

- [ ] **Step 3: Manual smoke (optional, requires a cluster)**

Run the TUI against a real environment, navigate into GPUNode, and confirm: the `● LIVE` indicator appears; a `kubectl label node ...`/pod change reflects within ~5–10s; navigating to a non-k8s category clears the indicator. If no cluster is available, note this step as skipped.

- [ ] **Step 4: Final commit (if any lint fixups were needed)**

```bash
git add -A
git commit -m "chore(tui): lint fixups for k8s category watches"
```

---

## Self-Review

**Spec coverage:**
- Watch-as-trigger principle → Tasks 1, 2, 5 (`reloadCategoryCmd` re-runs `LoadXxx`). ✓
- All five categories → Task 2 (five `WatchXxx`), Task 3 (five loader methods), Task 4 (`startWatchCmd` switch), Task 6 (five handlers). ✓
- Per-category trigger table incl. DAC+pods → Task 2 (`WatchDedicatedAIClusters` includes `gpuPodWatchOpeners`). ✓
- Activation on category view → Task 6 (start on entry), Task 5 Step 6 (clear on leave) + existing `newLoadContext` teardown. ✓
- Eventual freshness / 5s debounce, injectable → Task 1 (`DebounceWindow` var). ✓
- Fallback to one-shot load → Task 4 (`watchUnavailableMsg`), Task 5 (`handleWatchClosed` final reload). ✓
- Live indicator, cleared on fallback/leave → Task 7 (render), Task 5 (`watching=false` on closed/unavailable/category-change). ✓
- Lifecycle bound to loadCtx, no leaks → Task 1 (ctx-cancel stops watchers, closes channel; test asserts `IsStopped`). ✓
- Stale-drop via Gen → Task 5 (every handler checks `msg.Gen != m.gen`). ✓
- Optional Watcher (not in Composite) → Task 3 (deviation documented). ✓
- Testing across k8s/loader/tui layers → Tasks 1–7 each include tests. ✓

**Placeholder scan:** No "TBD"/"handle errors appropriately"; the three "Note for the implementer" blocks point at concrete existing test helpers to reuse and give exact fallback names — they are disambiguation guidance, not missing content. All code steps contain full code.

**Type consistency:** `watchStartedMsg`/`watchTriggeredMsg`/`watchClosedMsg`/`watchUnavailableMsg` carry `{Cat, Gen}` (+`Trigger` on started) consistently across Tasks 4–6. `<-chan struct{}` trigger type is identical across k8s (Task 1/2), loader (Task 3), and TUI (Tasks 4/5). `reloadCategoryCmd(cat, gen)` and `startWatchCmd(ctx, ld, cat, kubeCfg, env, gen)` signatures match their call sites. `DebounceWindow` (exported) named identically in Tasks 1, 2.
