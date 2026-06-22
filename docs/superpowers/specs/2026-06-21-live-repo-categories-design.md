# Live Repo-Backed Categories — Design

**Date:** 2026-06-21
**Status:** Approved (pending spec review)

## Goal

Make repo-backed TUI categories update live, the way the k8s-backed
categories already do, by watching the local working tree with fsnotify and
re-running `LoadDataset` on change. The reload feeds the same coalesced
trigger → reducer path the k8s watch uses and preserves the active filter and
selected-row cursor.

## Background

The TUI's data has two sources:

| Source | Categories | Load arg |
|---|---|---|
| **k8s** (dynamic/clientset) | BaseModel, ImportedModel, GPUNode, GPUWorkload, DedicatedAICluster | `kubeCfg` |
| **git repo** (`configloader.LoadDataset` + a few repo loaders) | Tenant, all Definitions, all Tenancy/Regional Overrides, GPUPool, ModelArtifact, Environment, ServiceTenancy, Alias | `repo` |

Every k8s-backed category is already live (`internal/infra/k8s/watch.go`,
per-category `WatchX` methods, wired through `startWatchCmd`). No un-watched
k8s category remains. Everything else is repo-backed and never updates after
the initial load.

Repo data has a **single source**: `configloader.LoadDataset(repoPath, env)`,
run once at `Init` into the in-memory `models.Dataset`. The k8s watch, by
contrast, is per-category. The consumer side (`Trigger <-chan struct{}` →
reducer re-runs a loader) is source-agnostic, so a filesystem producer drops
in cleanly.

`github.com/fsnotify/fsnotify v1.9.0` is already a transitive dependency.

## Key decisions

1. **Single dataset-level watch**, not per-category. One fsnotify watcher; on
   any working-tree change, re-run the full `LoadDataset` and refresh the
   current view. Matches repo data's single source and avoids a fragile
   category→path map.
2. **Always-on for the session.** The watch starts once at `Init`, tied to
   `m.parentCtx` (the session context that survives navigation and cancels on
   shutdown). It is not torn down or re-established on navigation.
3. **Reuse the 5 s debounce window** (`k8s.DebounceWindow`). Repo files change
   infrequently; no need for a snappier window.
4. **Include GPUPool.** GPUPool is repo-sourced but loaded via its own lazy
   path, not `LoadDataset`. On the repo trigger we also re-run the GPUPool
   loader so "every repo category" genuinely holds.
5. **Exclude `.git` and dot-directories** from the watch. Git operations alone
   would otherwise cause constant spurious reloads.

## Architecture

```
fswatch.Watch(parentCtx, repoPath)                  [new package]
    └── recursive fsnotify add (skip .git/dotdirs)
    └── shared debounce/coalesce (5s)  ── <-chan struct{} ──┐
                                                            │
production.Client.WatchRepo(ctx, repoPath)  [RepoWatcher]   │
    └── delegates to fswatch.Watch                          │
                                                            ▼
Init → startRepoWatchCmd(parentCtx) ── repoWatchStartedMsg{Trigger}
                                            │ m.repoWatching = true
                                            │ arm waitForRepoTriggerCmd
                                            ▼
        repoWatchTriggeredMsg ── reloadDatasetCmd (LoadDataset on parentCtx)
                              └─ if GPUPools loaded: reload GPUPools too
                              └─ re-arm waitForRepoTriggerCmd
                                            ▼
        datasetReloadedMsg{Dataset} ── m.dataset.MergeReloadedRepoData(fresh)
                                     └─ refreshDisplay (preserve filter+cursor)
```

## Components

### 1. `internal/infra/fswatch` (new package)

```go
// Watch establishes a recursive filesystem watch rooted at root and returns a
// coalesced trigger channel: one value per debounce window in which any file
// under root (excluding .git and dot-directories) changed. The caller owns
// ctx; cancelling it stops the watcher and closes the channel. The channel
// also closes if the watcher dies, which the caller treats as a fallback.
func Watch(ctx context.Context, root string, window time.Duration) (<-chan struct{}, error)
```

- Walks `root` with `filepath.WalkDir`, adding every directory to an
  `*fsnotify.Watcher`. Skips any directory whose base name starts with `.`
  (covers `.git`, `.idea`, etc.) via `fs.SkipDir`.
- Handles `fsnotify.Create` on a directory by adding the new directory to the
  watcher (so a freshly-created config subtree is covered) — unless it is a
  dot-directory.
- Forwards every event into a `raw` channel, then debounces into the output
  channel using the **shared coalesce helper** (below).
- On `Errors` channel activity or watcher close, closes the output channel
  (fallback signal).

### 2. Shared debounce helper

`watch.go` already contains `coalesce` and the `done/stopped` shutdown
choreography. To stay DRY, extract the generic debounce into a small reusable
function used by **both** the k8s watcher and `fswatch`:

```go
// package watchutil  (internal/infra/watchutil)
// Coalesce debounces raw signals into out using window, exiting when ctx is
// cancelled or done is closed.
func Coalesce(ctx context.Context, window time.Duration,
    raw <-chan struct{}, done <-chan struct{}, out chan<- struct{})
```

`k8s.watchTrigger` is refactored to call `watchutil.Coalesce`; its existing
behavior and tests are unchanged. `fswatch.Watch` calls the same helper.

> If extraction proves to entangle the k8s shutdown sequencing during
> implementation, fall back to a self-contained ~30-line debounce inside
> `fswatch` and note the duplication. DRY is the goal, not a mandate that
> risks the working k8s path.

### 3. `loader.RepoWatcher` (new optional capability interface)

```go
// RepoWatcher is an OPTIONAL capability: establishing a filesystem watch on
// the repo working tree that emits a coalesced "reload now" signal. Like
// Watcher and TenantMetadataWriter it is kept out of Composite so fake
// loaders need not implement it. Callers type-assert and degrade gracefully.
type RepoWatcher interface {
    WatchRepo(ctx context.Context, repoPath string) (<-chan struct{}, error)
}
```

Implemented on `production.Client`:

```go
func (Client) WatchRepo(ctx context.Context, repoPath string) (<-chan struct{}, error) {
    return fswatch.Watch(ctx, repoPath, k8s.DebounceWindow)
}
```

Kept separate from `Watcher` (whose doc is explicitly k8s).

### 4. Messages (`messages.go`)

Session-scoped — no `Cat`, no `Gen` (the watch is not per-category and not
subject to the navigation generation token):

```go
type repoWatchStartedMsg struct{ Trigger <-chan struct{} }
type repoWatchTriggeredMsg struct{}
type repoWatchClosedMsg struct{}
type datasetReloadedMsg struct{ Dataset *models.Dataset }
```

### 5. Commands (`loader_cmd.go`)

```go
// startRepoWatchCmd type-asserts the loader to loader.RepoWatcher and starts
// the working-tree watch. On success returns repoWatchStartedMsg; if the
// loader doesn't support it or setup fails, returns repoWatchClosedMsg so the
// app runs static with no live indicator.
func startRepoWatchCmd(ctx context.Context, ld loader.Composite, repoPath string) tea.Cmd

// waitForRepoTriggerCmd blocks on one value from the trigger: a tick →
// repoWatchTriggeredMsg, a close → repoWatchClosedMsg.
func waitForRepoTriggerCmd(trigger <-chan struct{}) tea.Cmd

// reloadDatasetCmd re-runs LoadDataset on the session context and returns
// datasetReloadedMsg. On error it logs at warn and returns nil (no toast, no
// state change) — a background refresh must not nag.
func reloadDatasetCmd(ctx context.Context, ld loader.Composite, repoPath string, env models.Environment, logger logging.Logger) tea.Cmd
```

### 6. Init wiring (`model.go`)

After `loadData()` and the existing lazy-category/initial-filter commands,
append `startRepoWatchCmd(m.parentCtx, m.loader, m.repoPath)`. The repo watch
uses `m.parentCtx`, **not** `m.loadCtx`, so navigation never cancels it.

### 7. Reducer handlers (`model_update.go` + a new `reducer_repowatch.go`)

- `repoWatchStartedMsg` → store `m.repoTrigger`, set `m.repoWatching = true`,
  log, return `waitForRepoTriggerCmd`.
- `repoWatchTriggeredMsg` → return a batch of:
  - `reloadDatasetCmd(m.parentCtx, …)`
  - if `m.dataset != nil && m.dataset.GPUPools != nil`, a GPUPool reload
    command targeting the dataset cache (re-run `LoadGPUPools`, apply via the
    existing `[]models.GPUPool` path in `handleDataMsg`/`applyDataset`)
  - `waitForRepoTriggerCmd(m.repoTrigger)` to re-arm
- `repoWatchClosedMsg` → set `m.repoWatching = false`, log warn. (No restart;
  rare for a local fs watch.)
- `datasetReloadedMsg` → `m.dataset.MergeReloadedRepoData(msg.Dataset)`, then
  `m.refreshDisplay()` (preserves filter + cursor by item key).

If `msg.Dataset` is nil (first-ever load not yet complete) or `m.dataset` is
nil, fall back to assigning the fresh dataset directly.

### 8. Merge apply (`pkg/models`)

The reload must preserve the lazily-loaded k8s fields already in `m.dataset`.
A method on `Dataset` keeps field-ownership maintained alongside the struct:

```go
// MergeReloadedRepoData copies the repo-owned fields from fresh into d,
// leaving the lazily-loaded k8s-backed fields (BaseModels, ImportedModelMap,
// GPUNodeMap, GPUWorkloadMap, DedicatedAIClusterMap) untouched. Used when a
// working-tree change triggers a dataset reload: LoadDataset repopulates only
// the repo-owned fields, so a wholesale assignment would wipe live k8s data.
func (d *Dataset) MergeReloadedRepoData(fresh *Dataset)
```

Repo-owned fields copied (from `configloader.LoadDataset`'s output): the three
DefinitionGroups, the three TenancyOverrideMaps, the three RegionalOverride
slices, `Tenants`, `Environments`, `ServiceTenancies`, `ModelArtifactMap`.
GPUPools is refreshed by its own reload command (decision 4), not here.

### 9. Live indicator (`model_view.go`)

```go
liveCell := ""
switch {
case m.watching:                                       // k8s category, live
    liveCell = m.liveStyle.Render("● LIVE")
case m.repoWatching && !m.category.NeedsKubeConfig():  // repo category, live
    liveCell = m.liveStyle.Render("● LIVE")
}
```

A repo-backed category shows `● LIVE` whenever the repo watch is established.

## Error handling

| Failure | Behavior |
|---|---|
| `WatchRepo` setup fails (missing path, OS watch limit) | `repoWatchClosedMsg`; no indicator; app runs static; log warn |
| Background `LoadDataset` reload errors | log warn; keep current data; **no toast**; re-arm listener still happens |
| GPUPool reload errors | existing error path, but downgraded to warn (background) |
| Watcher dies (channel close) | `repoWatchClosedMsg` → `m.repoWatching = false`; indicator drops |

## State additions (`model_state.go`)

```go
repoTrigger  <-chan struct{} // live working-tree trigger; nil when unavailable
repoWatching bool            // true while the repo watch is established
```

## Testing

**`fswatch` (unit):**
- Temp dir, `Watch`, touch a file → trigger arrives within ~2×window.
- Create a subdir then a file in it → trigger arrives (recursive add works).
- Write under `<root>/.git/…` → **no** trigger (exclusion works).
- Cancel ctx → output channel closes.

**`Dataset.MergeReloadedRepoData` (unit):**
- Populate a dataset with both k8s fields and repo fields; merge a repo-only
  `fresh` → repo fields updated, k8s fields preserved byte-for-byte.

**TUI (unit):**
- `repoWatchStartedMsg` → `m.repoWatching` true, returns a non-nil (re-arm) cmd.
- `repoWatchTriggeredMsg` → returns a batch including a reload cmd and re-arm.
- `datasetReloadedMsg` → dataset merged, filter + cursor preserved (extend the
  existing cursor-preservation tests).
- Live indicator shows on a repo category when `repoWatching`, hidden when not.
- Nil-store / nil-dataset paths do not panic.

## Out of scope

- Upstream/remote tracking (git fetch/poll). "Live" here means local
  working-tree changes only, per the agreed scope.
- Per-file granular reload. The whole dataset is re-read on any change;
  debounce makes this cheap enough.
