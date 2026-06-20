# Design: Live watches for k8s-backed categories

**Date:** 2026-06-20
**Status:** Approved (pending spec review)

## Goal

Make the five Kubernetes-backed TUI categories update on their own while the
user is viewing them, instead of requiring a manual refresh. The five
categories are those for which `domain.Category.NeedsKubeConfig()` returns
true:

- `BaseModel`
- `ImportedModel`
- `GPUNode`
- `GPUWorkload`
- `DedicatedAICluster`

### Requirements (settled during brainstorming)

- **Scope:** all five `NeedsKubeConfig()` categories.
- **Activation:** on category view — start when the user enters the category,
  stop when they leave.
- **Freshness:** eventual (~5–10s acceptable); lowest practical complexity.
- **Fallback:** on any watch failure or unsupported watch, fall back to the
  existing one-shot load so the category still renders.
- **UI:** a small "live" status indicator while a watch is active, cleared on
  fallback or on leaving the category.

### Non-goals (YAGNI)

- No informer cache or local store.
- No incremental row merge / per-row change highlighting.
- No auto-reconnect with backoff (a dead stream falls back to a one-shot load).
- No watches for non-k8s categories.

## Core principle: watch as a trigger, not a data source

A Kubernetes watch in this design produces **only a debounced "something
changed" signal**. It never carries domain objects. When a signal arrives, the
reducer re-runs the **existing `LoadXxx` loader function** to produce a fresh
snapshot.

Consequences:

- No watch-event → domain-model translation code.
- All existing enrichment / grouping / tenant-keying logic is reused verbatim.
- Snapshot types are unchanged, so the existing `xxxLoadedMsg` handlers update
  table rows with no modification.
- Fallback is trivial: the thing a watch triggers *is* the load, so "fall back
  to one-shot load" is the same code path.

## Per-category watch triggers

| Category             | Trigger resources                                | Client    |
| -------------------- | ------------------------------------------------ | --------- |
| `BaseModel`          | `clusterbasemodels` CRs                          | dynamic   |
| `ImportedModel`      | `basemodels` + `clusterbasemodels` CRs           | dynamic   |
| `GPUNode`            | Nodes **+** GPU pods                             | clientset |
| `GPUWorkload`        | GPU pods                                         | clientset |
| `DedicatedAICluster` | DAC CRs (v1alpha1 + v1beta1) **+** pods          | dynamic   |

All three enriched categories (`GPUNode`, `GPUWorkload`,
`DedicatedAICluster`) watch their primary resource(s) plus pods, so that
pod-derived fields (GPU allocation, node issues, DAC replica stats) reflect
changes without manual refresh.

## Components and boundaries

### k8s layer — `internal/infra/k8s/watch.go` (new)

One function per category:

```go
func WatchBaseModels(ctx context.Context, dyn dynamic.Interface) (<-chan struct{}, error)
func WatchImportedModels(ctx context.Context, dyn dynamic.Interface) (<-chan struct{}, error)
func WatchGPUNodes(ctx context.Context, cs kubernetes.Interface) (<-chan struct{}, error)
func WatchGPUWorkloads(ctx context.Context, cs kubernetes.Interface) (<-chan struct{}, error)
func WatchDedicatedAIClusters(ctx context.Context, dyn dynamic.Interface, cs kubernetes.Interface) (<-chan struct{}, error)
```

Each:

1. Opens `.Watch()` on each of its trigger resources (typed clientset for
   Nodes/Pods, dynamic client for CRs), reusing the same label/field selectors
   the corresponding `List()` already uses.
2. Fans all resource event streams into one goroutine.
3. **Coalesces events over a debounce window** (default 5s) and emits a single
   `struct{}` tick per window. The debounce lives here so the TUI stays dumb.
4. Is bound to `ctx`: on `ctx` cancellation or stream close, the goroutine
   exits and closes the channel.

If **any** required trigger watch fails to open, the function returns an error
and no channel (caller falls back to a one-shot load).

The debounce window is an injectable parameter (or package-level var) so tests
need not sleep for real time.

`DedicatedAICluster` takes both clients because it watches CRs (dynamic) and
pods (clientset).

### loader layer — `internal/infra/loader/interfaces.go` + production impl

New interface, folded into `Composite`:

```go
type Watcher interface {
    WatchBaseModels(ctx context.Context, kubeCfg string, env models.Environment) (<-chan struct{}, error)
    WatchImportedModels(ctx context.Context, kubeCfg string, env models.Environment) (<-chan struct{}, error)
    WatchGPUNodes(ctx context.Context, kubeCfg string, env models.Environment) (<-chan struct{}, error)
    WatchGPUWorkloads(ctx context.Context, kubeCfg string, env models.Environment) (<-chan struct{}, error)
    WatchDedicatedAIClusters(ctx context.Context, kubeCfg string, env models.Environment) (<-chan struct{}, error)
}
```

The production `Client` builds the client(s) with the same
`NewClientsetFromKubeConfig` / `NewDynamicClientFromKubeConfig` helpers the
load methods use, then calls the matching `k8s.WatchXxx`. Errors from client
construction or watch open propagate to the caller.

### TUI layer

**Messages (`messages.go`):**

```go
type watchStartedMsg     struct { Cat domain.Category; Trigger <-chan struct{}; Gen int }
type watchTriggeredMsg   struct { Cat domain.Category; Gen int }
type watchClosedMsg      struct { Cat domain.Category; Gen int }
type watchUnavailableMsg struct { Cat domain.Category; Gen int }
```

All carry `Gen` for stale-drop (same mechanism as the load messages).

**Commands (`loader_cmd.go`):**

- `startWatchCmd(ctx, ld, cat, kubeCfg, env, gen)` — calls the matching
  `Watcher.WatchXxx`. Returns `watchStartedMsg{trigger, gen}` on success or
  `watchUnavailableMsg{gen}` on setup failure.
- `waitForTriggerCmd(cat, trigger, gen)` — reads one value from the trigger
  channel. Returns `watchTriggeredMsg{gen}` on a tick, or `watchClosedMsg{gen}`
  if the channel is closed.

**Reducer (`reducer_category.go`):** the five `handleXxxCategory` functions
return `tea.Batch(loadXxxCmd(...), startWatchCmd(...))` instead of just the
load. The load keeps its existing cache gate (paints cached rows instantly on
re-entry, does a fresh `List()` on cache-miss or refresh). The watch starts
unconditionally for these five categories.

**Reducer message handling:**

- `watchStartedMsg` (gen matches): set `m.watching = true`, store nothing
  beyond issuing `waitForTriggerCmd(cat, trigger, gen)`.
- `watchTriggeredMsg` (gen matches): re-issue **both** `loadXxxCmd(...)` (fresh
  snapshot) and `waitForTriggerCmd(...)` (keep listening).
- `watchClosedMsg` (gen matches): fallback — issue one final `loadXxxCmd(...)`
  and set `m.watching = false` (no auto-reconnect).
- `watchUnavailableMsg` (gen matches): leave `m.watching = false`; the static
  load is already (or will be) on screen.
- Any watch message whose `Gen` does not match the current gen is ignored.

**State (`model_state.go`):** add `watching bool`.

**View (`model_view.go`):** add a small live cell next to `loadingCell`
(around line 85), rendered only when `m.watching` is true, included in the
status-bar width budget.

## Lifecycle and staleness (reuses existing machinery)

- **Teardown is free.** `updateCategoryCore` already calls
  `m.newLoadContext()`, which cancels the previous `loadCtx`. The watch
  goroutine is bound to `loadCtx`, so it exits and closes its channel on any
  category switch or refresh. `m.watching` is reset to false when the category
  changes.
- **Stale-drop is free.** Every watch message carries `Gen`. The reducer
  compares against the current gen (bumped by `m.bumpGen()` on each category
  change) exactly as it does for loads. Late ticks from a torn-down watch are
  dropped.

## Data flow

```
enter k8s category  [gen = N]
  → newLoadContext()                      # cancels old loadCtx, makes fresh one
  → handleXxxCategory returns
        tea.Batch( loadXxxCmd(gen=N), startWatchCmd(gen=N) )
  → loadXxxCmd  → xxxLoadedMsg(gen=N)     → rows render (initial paint)
  → startWatchCmd:
        success → watchStartedMsg{trigger, gen=N}
                    → m.watching = true; waitForTriggerCmd(trigger, gen=N)
        failure → watchUnavailableMsg{gen=N}
                    → m.watching stays false (static data already shown)

  … cluster changes …
  → k8s goroutine coalesces events over 5s → one tick on trigger
  → waitForTriggerCmd reads tick → watchTriggeredMsg{gen=N}
        → loadXxxCmd(gen=N) + waitForTriggerCmd(trigger, gen=N)
  → loadXxxCmd → xxxLoadedMsg(gen=N) → rows update

  … user leaves category …
  → newLoadContext() cancels loadCtx → goroutine exits, trigger closes
  → m.watching = false (set on category switch)
  → any in-flight watch msg with gen != current is ignored
```

## Error handling

- **Setup failure** (client build error, `watch` verb forbidden by RBAC, any
  trigger resource fails to open): `watchUnavailableMsg` → no live indicator;
  the one-shot load result remains on screen.
- **Mid-stream death** (API server closes the stream, network blip): the k8s
  goroutine closes the channel → `watchClosedMsg` → one final `loadXxxCmd` to
  capture the latest state, then `m.watching = false`. No auto-reconnect; the
  user can manual-refresh (re-enter / refresh key) to re-establish a watch.
- **Partial trigger failure** for a multi-resource category (e.g. Nodes watch
  opens but pods watch is forbidden): treated as setup failure — the category
  degrades to the static load with no live indicator, rather than running a
  half-watch that misses pod-driven changes.

## Testing

- **k8s layer:** use `k8s.io/client-go/kubernetes/fake` and
  `k8s.io/client-go/dynamic/fake`, whose fake clients support `.Watch()` via
  injectable fake watchers. Drive Add/Modify/Delete events and assert a
  coalesced tick fires; assert a burst of events within the window collapses to
  a single tick. Use the injectable debounce window so tests do not sleep for
  real seconds. Assert ctx cancellation closes the channel and stops the
  goroutine (no leak).
- **loader layer:** a mock `Watcher`; assert the setup-error path returns an
  error and no channel.
- **TUI layer:** reducer tests feed `watchStartedMsg`, `watchTriggeredMsg`,
  `watchClosedMsg`, and `watchUnavailableMsg` into `Update` and assert:
  `m.watching` transitions correctly; a trigger re-issues both load and wait
  commands; a `watchClosedMsg` issues a final load and clears `watching`; a gen
  mismatch drops stale watch messages.

## Pre-implementation safety check

Per project convention (`CLAUDE.md`), before editing run impact analysis on the
two high-fan-in edit sites and report the blast radius:

- `updateCategoryCore` (reducer entry point for every category change)
- the `loader.Composite` interface (adding `Watcher` widens it; all
  implementers and mocks must satisfy it)
