# Live Repo-Backed Categories Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make repo-backed TUI categories update live by watching the local working tree with fsnotify and re-running `LoadDataset` (plus GPU pools) on change, merging the result into the in-memory dataset without disturbing live k8s data, filter, or cursor.

**Architecture:** A single always-on filesystem watcher (`internal/infra/fswatch`) rooted at the repo path, established once at `Init` on the session context (`m.parentCtx`). It emits the same coalesced `<-chan struct{}` trigger the k8s watch produces. Each tick issues quiet background reloads (no spinner, no toast, no `pendingTasks` change): `LoadDataset` → merge repo-owned fields into `m.dataset` → `refreshDisplay`; and, when GPU pools are loaded, a parallel GPU-pool reload through its existing enrichment path.

**Tech Stack:** Go, Bubble Tea, `github.com/fsnotify/fsnotify` v1.9.0 (already a transitive dependency).

## Global Constraints

- Reuse the existing 5 s debounce window: `k8s.DebounceWindow` (do not introduce a second window constant).
- Exclude `.git` and all dot-directories from the watch (their base name starts with `.`).
- The repo watch runs on `m.parentCtx` (session-scoped), **never** `m.loadCtx` — navigation must not cancel it.
- Repo-watch messages are session-scoped: no `Gen`, no `Cat`. They are not subject to the navigation generation token.
- Background reloads must be quiet: no `beginTask`/`endTask`, no loading spinner, no error toast. On reload error, log at warn and keep current data.
- The merge must preserve the lazily-loaded k8s-backed dataset fields: `BaseModels`, `ImportedModelMap`, `GPUNodeMap`, `GPUWorkloadMap`, `DedicatedAIClusterMap`. (`GPUPools` is also preserved by the merge and refreshed by its own command.)
- `fsnotify` must become a direct dependency: run `go mod tidy` after Task 1.
- Follow existing patterns: value-receiver methods on `production.Client`; optional capability interfaces kept out of `Composite`; `logging.FromContext(ctx)` inside infra goroutines, `m.logger` inside the reducer.

## File Structure

| File | Responsibility |
|---|---|
| `internal/infra/fswatch/fswatch.go` (new) | Recursive fsnotify watcher + self-contained debounce; `Watch(ctx, root, window) (<-chan struct{}, error)` |
| `internal/infra/fswatch/fswatch_test.go` (new) | fswatch behavior tests |
| `pkg/models/dataset.go` (modify) | `Dataset.MergeReloadedRepoData(fresh)` |
| `pkg/models/dataset_merge_test.go` (new) | merge preservation test |
| `internal/infra/loader/interfaces.go` (modify) | `RepoWatcher` optional capability interface |
| `internal/infra/loader/production/production.go` (modify) | `Client.WatchRepo` delegating to `fswatch.Watch` |
| `internal/infra/loader/production/repowatch_test.go` (new) | interface-satisfaction + smoke test |
| `internal/ui/tui/messages.go` (modify) | 5 new message types |
| `internal/ui/tui/model_state.go` (modify) | `repoTrigger`, `repoWatching` fields; `sessionCtx()` helper |
| `internal/ui/tui/loader_cmd.go` (modify) | `startRepoWatchCmd`, `waitForRepoTriggerCmd`, `reloadDatasetCmd`, `reloadGPUPoolsCmd` |
| `internal/ui/tui/repowatch_cmd_test.go` (new) | command tests |
| `internal/ui/tui/reducer_repowatch.go` (new) | reducer handlers for the 5 messages |
| `internal/ui/tui/reducer_repowatch_test.go` (new) | handler + indicator tests |
| `internal/ui/tui/model_update.go` (modify) | wire the 5 message cases |
| `internal/ui/tui/model.go` (modify) | establish the repo watch in `Init` |
| `internal/ui/tui/model_view.go` (modify) | show `● LIVE` on repo categories |

> **Design note (deliberate divergence from the spec's preference):** the spec preferred extracting a shared `watchutil.Coalesce` used by both the k8s and fs watchers. This plan instead gives `fswatch` a **self-contained ~30-line debounce** and does **not** touch `internal/infra/k8s/watch.go`. Rationale: the existing `coalesce` is entangled with the k8s multi-watcher `done`/`stopped` shutdown choreography; refactoring it risks the working live-watch path for ~30 lines of saved duplication. The spec explicitly sanctioned this fallback. If a reviewer prefers the shared helper, that is a separate, low-priority refactor.

---

### Task 1: `fswatch` package — recursive working-tree watcher

**Files:**
- Create: `internal/infra/fswatch/fswatch.go`
- Test: `internal/infra/fswatch/fswatch_test.go`

**Interfaces:**
- Consumes: `github.com/fsnotify/fsnotify` (`NewWatcher`, `Watcher.Add`, `Watcher.Events`, `Watcher.Errors`, `Event.Op.Has(fsnotify.Create)`); `logging.FromContext`.
- Produces: `func Watch(ctx context.Context, root string, window time.Duration) (<-chan struct{}, error)` — one value per debounce window in which a non-hidden file under `root` changed; channel closes on ctx cancel or watcher death.

- [ ] **Step 1: Write the failing tests**

Create `internal/infra/fswatch/fswatch_test.go`:

```go
package fswatch

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestWatch_TriggersOnFileChange(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	trig, err := Watch(ctx, dir, 50*time.Millisecond)
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.yaml"), []byte("x"), 0o644))

	select {
	case <-trig:
	case <-time.After(2 * time.Second):
		t.Fatal("expected a trigger after writing a file")
	}
}

func TestWatch_RecursiveNewSubdir(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	trig, err := Watch(ctx, dir, 50*time.Millisecond)
	require.NoError(t, err)

	sub := filepath.Join(dir, "sub")
	require.NoError(t, os.Mkdir(sub, 0o755))
	<-trig // drain the trigger caused by creating the directory

	require.NoError(t, os.WriteFile(filepath.Join(sub, "b.yaml"), []byte("y"), 0o644))
	select {
	case <-trig:
	case <-time.After(2 * time.Second):
		t.Fatal("expected trigger from a file in a newly created subdir")
	}
}

func TestWatch_IgnoresDotGit(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	trig, err := Watch(ctx, dir, 50*time.Millisecond)
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(filepath.Join(dir, ".git", "HEAD"), []byte("ref"), 0o644))
	select {
	case <-trig:
		t.Fatal("changes under .git must not trigger a reload")
	case <-time.After(400 * time.Millisecond):
		// good: no trigger
	}
}

func TestWatch_CancelClosesChannel(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())

	trig, err := Watch(ctx, dir, 50*time.Millisecond)
	require.NoError(t, err)

	cancel()
	select {
	case _, ok := <-trig:
		require.False(t, ok, "channel should be closed after ctx cancel")
	case <-time.After(2 * time.Second):
		t.Fatal("channel was not closed after ctx cancel")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/infra/fswatch/...`
Expected: FAIL — `undefined: Watch` (package does not compile).

- [ ] **Step 3: Implement `fswatch.Watch`**

Create `internal/infra/fswatch/fswatch.go`:

```go
// Package fswatch provides a recursive, debounced filesystem watcher that
// emits a coalesced "something changed" trigger — the same channel shape the
// k8s watch feeds into the TUI reducer. It is used to make repo-backed
// categories live by reloading the dataset when the working tree changes.
package fswatch

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/jingle2008/toolkit/pkg/infra/logging"
)

// Watch establishes a recursive filesystem watch rooted at root and returns a
// coalesced trigger channel: one value per debounce window in which any
// non-hidden file under root changed. Dot-directories (.git, .idea, …) are
// excluded. The caller owns ctx; cancelling it stops the watcher and closes
// the channel. The channel also closes if the watcher backend dies, which the
// caller treats as a fallback signal.
func Watch(ctx context.Context, root string, window time.Duration) (<-chan struct{}, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	if err := addRecursive(w, root); err != nil {
		_ = w.Close()
		return nil, err
	}

	out := make(chan struct{})
	go run(ctx, w, root, window, out)
	return out, nil
}

// isHidden reports whether path's base name marks a dot-entry (.git, .idea),
// excluding the relative "." and ".." entries.
func isHidden(path string) bool {
	base := filepath.Base(path)
	return base != "." && base != ".." && strings.HasPrefix(base, ".")
}

// addRecursive adds root and every non-hidden subdirectory to w.
func addRecursive(w *fsnotify.Watcher, root string) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if path != root && isHidden(path) {
			return fs.SkipDir
		}
		return w.Add(path)
	})
}

// run consumes fsnotify events, adds newly created directories on the fly,
// ignores hidden paths, debounces into out, and tears everything down on ctx
// cancel or backend error.
func run(ctx context.Context, w *fsnotify.Watcher, root string, window time.Duration, out chan<- struct{}) {
	defer close(out)
	defer func() { _ = w.Close() }()

	logging.FromContext(ctx).Infow("fs watch established", "root", root)

	var timerC <-chan time.Time
	for {
		select {
		case <-ctx.Done():
			logging.FromContext(ctx).Debugw("fs watch stopped: context canceled")
			return
		case ev, ok := <-w.Events:
			if !ok {
				return
			}
			if isHidden(ev.Name) {
				continue // ignore events under dot-dirs (e.g. .git churn)
			}
			// A newly created directory must be watched so its contents
			// trigger too.
			if ev.Op.Has(fsnotify.Create) {
				if info, statErr := os.Stat(ev.Name); statErr == nil && info.IsDir() {
					_ = addRecursive(w, ev.Name)
				}
			}
			if timerC == nil {
				timerC = time.After(window)
			}
		case _, ok := <-w.Errors:
			if !ok {
				return
			}
			logging.FromContext(ctx).Warnw("fs watch error; live repo watch will drop")
			return
		case <-timerC:
			timerC = nil
			select {
			case out <- struct{}{}:
			case <-ctx.Done():
				return
			}
		}
	}
}
```

- [ ] **Step 4: Promote fsnotify to a direct dependency**

Run: `go mod tidy`
Expected: `go.mod` line for `github.com/fsnotify/fsnotify v1.9.0` loses its `// indirect` comment. No other module changes.

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/infra/fswatch/...`
Expected: PASS (4 tests).

- [ ] **Step 6: Commit**

```bash
git add internal/infra/fswatch/ go.mod go.sum
git commit -m "feat(fswatch): recursive debounced working-tree watcher"
```

---

### Task 2: `Dataset.MergeReloadedRepoData`

**Files:**
- Modify: `pkg/models/dataset.go`
- Test: `pkg/models/dataset_merge_test.go` (new)

**Interfaces:**
- Produces: `func (d *Dataset) MergeReloadedRepoData(fresh *Dataset)` — copies the repo-owned fields from `fresh` into `d`, preserving `d`'s lazily-loaded k8s fields (`BaseModels`, `ImportedModelMap`, `GPUPools`, `GPUNodeMap`, `GPUWorkloadMap`, `DedicatedAIClusterMap`).

- [ ] **Step 1: Write the failing test**

Create `pkg/models/dataset_merge_test.go`:

```go
package models

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDataset_MergeReloadedRepoData_PreservesK8sFields(t *testing.T) {
	d := &Dataset{
		Tenants:    []Tenant{{Name: "old"}},
		BaseModels: []BaseModel{{Name: "bm1"}},
		GPUPools:   []GPUPool{{Name: "p1"}},
		GPUNodeMap: map[string][]GPUNode{"p1": {{Name: "n1"}}},
	}
	bm := d.BaseModels
	pools := d.GPUPools
	nodes := d.GPUNodeMap

	fresh := &Dataset{
		Tenants:      []Tenant{{Name: "new1"}, {Name: "new2"}},
		Environments: []Environment{{Type: "dev"}},
		// k8s-backed fields left nil, as LoadDataset returns them
	}

	d.MergeReloadedRepoData(fresh)

	// Repo-owned fields are replaced by the freshly loaded values.
	require.Len(t, d.Tenants, 2)
	require.Equal(t, "new1", d.Tenants[0].Name)
	require.Len(t, d.Environments, 1)

	// Lazily-loaded k8s fields are preserved untouched.
	require.Equal(t, bm, d.BaseModels)
	require.Equal(t, pools, d.GPUPools)
	require.Equal(t, nodes, d.GPUNodeMap)
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./pkg/models/ -run TestDataset_MergeReloadedRepoData -v`
Expected: FAIL — `d.MergeReloadedRepoData undefined`.

- [ ] **Step 3: Implement the method**

Add to `pkg/models/dataset.go` (after `ResetRealmScopedFields`):

```go
// MergeReloadedRepoData copies the repo-owned fields from fresh into d while
// preserving the lazily-loaded, k8s-backed fields already present in d
// (BaseModels, ImportedModelMap, GPUPools, GPUNodeMap, GPUWorkloadMap,
// DedicatedAIClusterMap). It is used when a working-tree change triggers a
// dataset reload: LoadDataset repopulates only the repo-owned fields, so a
// wholesale assignment would wipe live k8s data. New repo-owned fields added
// to Dataset are carried across automatically; only the small, stable set of
// k8s fields is enumerated here.
func (d *Dataset) MergeReloadedRepoData(fresh *Dataset) {
	fresh.BaseModels = d.BaseModels
	fresh.ImportedModelMap = d.ImportedModelMap
	fresh.GPUPools = d.GPUPools
	fresh.GPUNodeMap = d.GPUNodeMap
	fresh.GPUWorkloadMap = d.GPUWorkloadMap
	fresh.DedicatedAIClusterMap = d.DedicatedAIClusterMap
	*d = *fresh
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./pkg/models/ -run TestDataset_MergeReloadedRepoData -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/models/dataset.go pkg/models/dataset_merge_test.go
git commit -m "feat(models): MergeReloadedRepoData preserves live k8s fields"
```

---

### Task 3: `RepoWatcher` interface + production `WatchRepo`

**Files:**
- Modify: `internal/infra/loader/interfaces.go` (add after the `Watcher` interface, ~line 131)
- Modify: `internal/infra/loader/production/production.go`
- Test: `internal/infra/loader/production/repowatch_test.go` (new)

**Interfaces:**
- Consumes: `fswatch.Watch` (Task 1); `k8s.DebounceWindow`.
- Produces: `loader.RepoWatcher` interface with `WatchRepo(ctx context.Context, repoPath string) (<-chan struct{}, error)`; `production.Client` implements it.

- [ ] **Step 1: Write the failing test**

Create `internal/infra/loader/production/repowatch_test.go`:

```go
package production

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/infra/loader"
)

// Compile-time guarantee that Client satisfies the optional RepoWatcher
// capability the TUI type-asserts for.
var _ loader.RepoWatcher = Client{}

func TestClient_WatchRepo_TriggersOnChange(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	trig, err := Client{}.WatchRepo(ctx, dir)
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "x.yaml"), []byte("v"), 0o644))
	select {
	case <-trig:
	case <-time.After(7 * time.Second): // > DebounceWindow (5s)
		t.Fatal("expected a trigger from WatchRepo after a file change")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/infra/loader/production/ -run TestClient_WatchRepo -v`
Expected: FAIL — `Client` does not implement `loader.RepoWatcher` (missing `WatchRepo`); package does not compile.

- [ ] **Step 3: Add the `RepoWatcher` interface**

Add to `internal/infra/loader/interfaces.go` after the `Watcher` interface (after line 131):

```go
/*
RepoWatcher is an OPTIONAL capability: establishing a filesystem watch on the
repo working tree that emits a coalesced "reload now" signal, making
repo-backed categories live the way Watcher makes k8s-backed categories live.
Like Watcher and TenantMetadataWriter it is deliberately kept out of Composite
so the many fake loaders used in tests need not implement it. Callers
type-assert a Composite to this interface and fall back to a static load when
the assertion fails or the method returns an error.

The returned channel yields one value whenever any non-hidden file under
repoPath changes (debounced). The caller owns ctx; cancelling it stops the
watch and closes the channel. The channel also closes if the watcher dies,
which the caller treats as a fallback signal.
*/
type RepoWatcher interface {
	WatchRepo(ctx context.Context, repoPath string) (<-chan struct{}, error)
}
```

- [ ] **Step 4: Implement `WatchRepo` on the production client**

Add the import `"github.com/jingle2008/toolkit/internal/infra/fswatch"` to `internal/infra/loader/production/production.go`, then add the method (next to the other `Watch*` methods):

```go
// WatchRepo establishes a debounced filesystem watch on the repo working tree.
// It reuses k8s.DebounceWindow so repo and k8s watches coalesce on the same
// cadence.
func (Client) WatchRepo(ctx context.Context, repoPath string) (<-chan struct{}, error) {
	return fswatch.Watch(ctx, repoPath, k8s.DebounceWindow)
}
```

- [ ] **Step 5: Run the test to verify it passes**

Run: `go test ./internal/infra/loader/production/ -run TestClient_WatchRepo -v`
Expected: PASS (compile-time assertion holds; trigger fires).

- [ ] **Step 6: Commit**

```bash
git add internal/infra/loader/interfaces.go internal/infra/loader/production/production.go internal/infra/loader/production/repowatch_test.go
git commit -m "feat(loader): RepoWatcher capability + production WatchRepo"
```

---

### Task 4: TUI messages, state, and commands

**Files:**
- Modify: `internal/ui/tui/messages.go`
- Modify: `internal/ui/tui/model_state.go`
- Modify: `internal/ui/tui/loader_cmd.go`
- Test: `internal/ui/tui/repowatch_cmd_test.go` (new)

**Interfaces:**
- Consumes: `loader.RepoWatcher`, `loader.Composite`, `models.Environment`, `logging.Logger`, `tea.Cmd`.
- Produces:
  - Messages: `repoWatchStartedMsg{Trigger <-chan struct{}}`, `repoWatchTriggeredMsg{}`, `repoWatchClosedMsg{}`, `datasetReloadedMsg{Dataset *models.Dataset}`, `gpuPoolsReloadedMsg{Items []models.GPUPool}`.
  - State: `Model.repoTrigger <-chan struct{}`, `Model.repoWatching bool`, `Model.sessionCtx() context.Context`.
  - Commands: `startRepoWatchCmd(ctx, ld, repoPath) tea.Cmd`, `waitForRepoTriggerCmd(trigger) tea.Cmd`, `reloadDatasetCmd(ctx, ld, repoPath, env, logger) tea.Cmd`, `reloadGPUPoolsCmd(ctx, ld, repoPath, env, logger) tea.Cmd`.

- [ ] **Step 1: Add the message types**

Append to `internal/ui/tui/messages.go` (the file already imports `models`):

```go
// --- repo (working-tree) watch: session-scoped, no Gen/Cat. ---

// repoWatchStartedMsg signals the working-tree watch is live.
type repoWatchStartedMsg struct{ Trigger <-chan struct{} }

// repoWatchTriggeredMsg signals one debounced working-tree change; the reducer
// issues quiet background reloads and re-arms the listener.
type repoWatchTriggeredMsg struct{}

// repoWatchClosedMsg signals the working-tree watch is unavailable or died;
// the live repo indicator drops. No auto-reconnect.
type repoWatchClosedMsg struct{}

// datasetReloadedMsg carries a freshly loaded dataset to be merged into the
// in-memory one (repo-owned fields only; live k8s fields preserved).
type datasetReloadedMsg struct{ Dataset *models.Dataset }

// gpuPoolsReloadedMsg carries freshly loaded GPU pools (repo-sourced via their
// own loader, not LoadDataset) to refresh the cached pool list.
type gpuPoolsReloadedMsg struct{ Items []models.GPUPool }
```

- [ ] **Step 2: Add state fields and the session-context helper**

In `internal/ui/tui/model_state.go`, add to the `Model` struct (near `watching bool` / `watchTrigger`):

```go
	// repoTrigger is the live working-tree trigger channel; nil when the
	// repo watch is unavailable. repoWatching is true while it is established.
	repoTrigger  <-chan struct{}
	repoWatching bool
```

And add the helper (near `opCtx`):

```go
// sessionCtx returns the session-scoped context (survives navigation, cancels
// on shutdown) used by the always-on repo watch and its background reloads.
func (m *Model) sessionCtx() context.Context {
	if m.parentCtx == nil {
		return context.Background()
	}
	return m.parentCtx
}
```

- [ ] **Step 3: Write the failing command tests**

Create `internal/ui/tui/repowatch_cmd_test.go`:

```go
package tui

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/infra/loader"
	"github.com/jingle2008/toolkit/pkg/infra/logging"
	"github.com/jingle2008/toolkit/pkg/models"
)

// repoWatchLoader is a fake Composite that also implements RepoWatcher.
type repoWatchLoader struct {
	loader.Composite
	trigger <-chan struct{}
	err     error
}

func (f repoWatchLoader) WatchRepo(context.Context, string) (<-chan struct{}, error) {
	return f.trigger, f.err
}

func TestStartRepoWatchCmd_Started(t *testing.T) {
	ch := make(chan struct{})
	cmd := startRepoWatchCmd(context.Background(), repoWatchLoader{trigger: ch}, "/repo")
	msg := cmd()
	started, ok := msg.(repoWatchStartedMsg)
	require.True(t, ok, "expected repoWatchStartedMsg, got %T", msg)
	require.NotNil(t, started.Trigger)
}

func TestStartRepoWatchCmd_NotAWatcher(t *testing.T) {
	// fakeLoader (defined in the package's existing tests) does not implement
	// RepoWatcher.
	cmd := startRepoWatchCmd(context.Background(), fakeLoader{}, "/repo")
	_, ok := cmd().(repoWatchClosedMsg)
	require.True(t, ok, "a loader without RepoWatcher must yield repoWatchClosedMsg")
}

func TestStartRepoWatchCmd_SetupError(t *testing.T) {
	cmd := startRepoWatchCmd(context.Background(), repoWatchLoader{err: errors.New("nope")}, "/repo")
	_, ok := cmd().(repoWatchClosedMsg)
	require.True(t, ok, "a WatchRepo error must yield repoWatchClosedMsg")
}

func TestWaitForRepoTriggerCmd_TickAndClose(t *testing.T) {
	ch := make(chan struct{}, 1)
	ch <- struct{}{}
	_, ok := waitForRepoTriggerCmd(ch)().(repoWatchTriggeredMsg)
	require.True(t, ok, "a value on the channel must yield repoWatchTriggeredMsg")

	closed := make(chan struct{})
	close(closed)
	_, ok = waitForRepoTriggerCmd(closed)().(repoWatchClosedMsg)
	require.True(t, ok, "a closed channel must yield repoWatchClosedMsg")
}

func TestReloadDatasetCmd_SuccessAndError(t *testing.T) {
	ds := &models.Dataset{Tenants: []models.Tenant{{Name: "t"}}}
	okLoader := fakeLoader{dataset: ds}
	msg := reloadDatasetCmd(context.Background(), okLoader, "/repo", models.Environment{}, logging.NewNoOpLogger())()
	reloaded, ok := msg.(datasetReloadedMsg)
	require.True(t, ok, "expected datasetReloadedMsg, got %T", msg)
	require.Same(t, ds, reloaded.Dataset)

	errLoader := fakeLoader{datasetErr: errors.New("boom")}
	require.Nil(t, reloadDatasetCmd(context.Background(), errLoader, "/repo", models.Environment{}, logging.NewNoOpLogger())(),
		"a reload error must produce a nil (no-op) message, not a toast")
}
```

> **Implementer note:** `fakeLoader` already exists in the tui test package. Inspect its current definition (e.g. `grep -n "type fakeLoader" internal/ui/tui/*_test.go`). If it does not already let a test supply a dataset / dataset error for `LoadDataset`, add the fields `dataset *models.Dataset` and `datasetErr error` and have its `LoadDataset` return them (falling back to its current behavior when both are zero). Do not break existing usages — only extend.

- [ ] **Step 4: Run the tests to verify they fail**

Run: `go test ./internal/ui/tui/ -run 'RepoWatch|RepoTrigger|ReloadDataset' -v`
Expected: FAIL — `undefined: startRepoWatchCmd` / `waitForRepoTriggerCmd` / `reloadDatasetCmd`.

- [ ] **Step 5: Implement the commands**

Append to `internal/ui/tui/loader_cmd.go` (it already imports `context`, `fmt`, `tea`, `loader`, `models`, `domain`; add `logging "github.com/jingle2008/toolkit/pkg/infra/logging"` if not already imported):

```go
// startRepoWatchCmd type-asserts the loader to loader.RepoWatcher and starts
// the working-tree watch on the session context. On success it returns
// repoWatchStartedMsg; if the loader doesn't support watching or setup fails,
// it returns repoWatchClosedMsg so the app runs static with no live indicator.
func startRepoWatchCmd(ctx context.Context, ld loader.Composite, repoPath string) tea.Cmd {
	return func() tea.Msg {
		rw, ok := ld.(loader.RepoWatcher)
		if !ok {
			return repoWatchClosedMsg{}
		}
		trigger, err := rw.WatchRepo(ctx, repoPath)
		if err != nil {
			return repoWatchClosedMsg{}
		}
		return repoWatchStartedMsg{Trigger: trigger}
	}
}

// waitForRepoTriggerCmd blocks on one value from the repo trigger: a tick →
// repoWatchTriggeredMsg, a close → repoWatchClosedMsg.
func waitForRepoTriggerCmd(trigger <-chan struct{}) tea.Cmd {
	return func() tea.Msg {
		if _, ok := <-trigger; !ok {
			return repoWatchClosedMsg{}
		}
		return repoWatchTriggeredMsg{}
	}
}

// reloadDatasetCmd re-runs LoadDataset on the session context and returns
// datasetReloadedMsg. On error it logs at warn and returns nil — a background
// refresh must not raise a toast or disturb the loading state.
func reloadDatasetCmd(ctx context.Context, ld loader.Composite, repoPath string, env models.Environment, logger logging.Logger) tea.Cmd {
	return func() tea.Msg {
		ds, err := ld.LoadDataset(ctx, repoPath, env)
		if err != nil {
			logger.Warnw("background dataset reload failed; keeping current data", "error", err)
			return nil
		}
		return datasetReloadedMsg{Dataset: ds}
	}
}

// reloadGPUPoolsCmd re-runs LoadGPUPools on the session context and returns
// gpuPoolsReloadedMsg. Like reloadDatasetCmd it is quiet on error.
func reloadGPUPoolsCmd(ctx context.Context, ld loader.Composite, repoPath string, env models.Environment, logger logging.Logger) tea.Cmd {
	return func() tea.Msg {
		items, err := ld.LoadGPUPools(ctx, repoPath, env)
		if err != nil {
			logger.Warnw("background GPU pool reload failed; keeping current data", "error", err)
			return nil
		}
		return gpuPoolsReloadedMsg{Items: items}
	}
}
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `go test ./internal/ui/tui/ -run 'RepoWatch|RepoTrigger|ReloadDataset' -v`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/ui/tui/messages.go internal/ui/tui/model_state.go internal/ui/tui/loader_cmd.go internal/ui/tui/repowatch_cmd_test.go
git commit -m "feat(tui): repo-watch messages, state, and background-reload commands"
```

---

### Task 5: Reducer handlers, Init wiring, and live indicator

**Files:**
- Create: `internal/ui/tui/reducer_repowatch.go`
- Modify: `internal/ui/tui/model_update.go`
- Modify: `internal/ui/tui/model.go`
- Modify: `internal/ui/tui/model_view.go`
- Test: `internal/ui/tui/reducer_repowatch_test.go` (new)

**Interfaces:**
- Consumes: the messages/commands from Task 4; `Dataset.MergeReloadedRepoData` (Task 2); `m.updateGPUPoolState()`, `m.refreshDisplay()`, `m.sessionCtx()`; `domain.GPUPool`, `domain.Category.NeedsKubeConfig()`.
- Produces: handlers `handleRepoWatchStarted`, `handleRepoWatchTriggered`, `handleRepoWatchClosed`, `handleDatasetReloaded`, `handleGPUPoolsReloaded`; the `Init` repo-watch wiring; the repo-aware live indicator.

- [ ] **Step 1: Write the failing tests**

Create `internal/ui/tui/reducer_repowatch_test.go`:

```go
package tui

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/pkg/models"
)

func TestHandleRepoWatchStarted_SetsWatchingAndArms(t *testing.T) {
	m := newTestModel(t)
	ch := make(chan struct{})
	cmd := m.handleRepoWatchStarted(repoWatchStartedMsg{Trigger: ch})
	require.True(t, m.repoWatching)
	require.NotNil(t, cmd, "must return a re-arm command")
}

func TestHandleRepoWatchClosed_ClearsWatching(t *testing.T) {
	m := newTestModel(t)
	m.repoWatching = true
	m.handleRepoWatchClosed()
	require.False(t, m.repoWatching)
}

func TestHandleRepoWatchTriggered_ReturnsBatch(t *testing.T) {
	m := newTestModel(t)
	m.dataset = &models.Dataset{GPUPools: []models.GPUPool{{Name: "p1"}}}
	m.repoTrigger = make(chan struct{})
	cmd := m.handleRepoWatchTriggered()
	require.NotNil(t, cmd, "trigger must produce reload + re-arm commands")
}

func TestHandleDatasetReloaded_MergesPreservingK8s(t *testing.T) {
	m := newTestModel(t)
	m.dataset = &models.Dataset{
		Tenants:    []models.Tenant{{Name: "old"}},
		BaseModels: []models.BaseModel{{Name: "bm1"}},
	}
	m.handleDatasetReloaded(datasetReloadedMsg{Dataset: &models.Dataset{
		Tenants: []models.Tenant{{Name: "new"}},
	}})
	require.Equal(t, "new", m.dataset.Tenants[0].Name, "repo field updated")
	require.Len(t, m.dataset.BaseModels, 1, "k8s field preserved")
}

func TestHandleDatasetReloaded_NilDatasetIgnored(t *testing.T) {
	m := newTestModel(t)
	before := m.dataset
	m.handleDatasetReloaded(datasetReloadedMsg{Dataset: nil})
	require.Same(t, before, m.dataset, "a nil reload must be ignored")
}

func TestLiveIndicator_ShowsOnRepoCategory(t *testing.T) {
	m := newTestModel(t)
	m.viewWidth, m.viewHeight = 100, 20
	m.category = domain.Tenant // repo-backed: NeedsKubeConfig() == false
	m.repoWatching = true
	require.True(t, strings.Contains(m.View(), "LIVE"),
		"a repo-backed category with repoWatching must show the live indicator")

	m.repoWatching = false
	require.False(t, strings.Contains(m.View(), "LIVE"),
		"no indicator when the repo watch is not established")
}
```

> **Implementer note:** `newTestModel(t)` exists in the package's tests. If it does not default to a usable view size or a non-nil dataset, set `m.viewWidth`/`m.viewHeight` (as above) and assign `m.dataset` within each test as needed. Confirm `domain.Tenant` returns `false` from `NeedsKubeConfig()` (it is repo-backed); if the helper model defaults to a different category, the indicator test already sets `m.category` explicitly.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/ui/tui/ -run 'HandleRepoWatch|HandleDatasetReloaded|LiveIndicator_ShowsOnRepo' -v`
Expected: FAIL — handlers undefined.

- [ ] **Step 3: Implement the reducer handlers**

Create `internal/ui/tui/reducer_repowatch.go`:

```go
package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jingle2008/toolkit/internal/domain"
)

// handleRepoWatchStarted records the live working-tree watch and arms the
// listener. Session-scoped: not gen-gated.
func (m *Model) handleRepoWatchStarted(msg repoWatchStartedMsg) tea.Cmd {
	m.repoWatching = true
	m.repoTrigger = msg.Trigger
	m.logger.Infow("repo watch started")
	return waitForRepoTriggerCmd(msg.Trigger)
}

// handleRepoWatchTriggered issues quiet background reloads (dataset, plus GPU
// pools when loaded) and re-arms the listener. No beginTask: a working-tree
// change must not flash the loading spinner.
func (m *Model) handleRepoWatchTriggered() tea.Cmd {
	m.logger.Debugw("repo watch triggered; reloading dataset")
	cmds := []tea.Cmd{
		reloadDatasetCmd(m.sessionCtx(), m.loader, m.repoPath, m.environment, m.logger),
	}
	if m.dataset != nil && m.dataset.GPUPools != nil {
		cmds = append(cmds, reloadGPUPoolsCmd(m.sessionCtx(), m.loader, m.repoPath, m.environment, m.logger))
	}
	if m.repoTrigger != nil {
		cmds = append(cmds, waitForRepoTriggerCmd(m.repoTrigger))
	}
	return tea.Batch(cmds...)
}

// handleRepoWatchClosed clears the live repo indicator. No auto-reconnect.
func (m *Model) handleRepoWatchClosed() {
	m.repoWatching = false
	m.logger.Warnw("repo watch closed; live repo indicator dropped")
}

// handleDatasetReloaded merges the freshly loaded repo-owned data into the
// in-memory dataset (preserving live k8s fields) and refreshes the view,
// preserving the active filter and selected-row cursor.
func (m *Model) handleDatasetReloaded(msg datasetReloadedMsg) {
	if msg.Dataset == nil {
		return
	}
	if m.dataset == nil {
		m.dataset = msg.Dataset
	} else {
		m.dataset.MergeReloadedRepoData(msg.Dataset)
	}
	m.refreshDisplay()
}

// handleGPUPoolsReloaded refreshes the cached GPU pool list and re-runs the
// pool enrichment (task-neutral). The view is rebuilt only when GPU pools are
// on screen.
func (m *Model) handleGPUPoolsReloaded(msg gpuPoolsReloadedMsg) tea.Cmd {
	if m.dataset == nil {
		return nil
	}
	m.dataset.GPUPools = msg.Items
	if m.category == domain.GPUPool {
		m.refreshDisplay()
	}
	return m.updateGPUPoolState()
}
```

- [ ] **Step 4: Wire the message cases**

In `internal/ui/tui/model_update.go`, add these cases alongside the existing `watchStartedMsg` … `watchUnavailableMsg` cases (around lines 79–86):

```go
	case repoWatchStartedMsg:
		return m, m.handleRepoWatchStarted(msg)
	case repoWatchTriggeredMsg:
		return m, m.handleRepoWatchTriggered()
	case repoWatchClosedMsg:
		m.handleRepoWatchClosed()
		return m, nil
	case datasetReloadedMsg:
		m.handleDatasetReloaded(msg)
		return m, nil
	case gpuPoolsReloadedMsg:
		return m, m.handleGPUPoolsReloaded(msg)
```

- [ ] **Step 5: Establish the watch in `Init`**

In `internal/ui/tui/model.go`, change the final return of `Init` from:

```go
	cmds = append(cmds, setFilter(m.initialFilter))
	return tea.Sequence(cmds...)
```

to:

```go
	cmds = append(cmds, setFilter(m.initialFilter))
	// Establish the always-on working-tree watch in parallel with the initial
	// load, on the session context so navigation never cancels it.
	return tea.Batch(
		tea.Sequence(cmds...),
		startRepoWatchCmd(m.sessionCtx(), m.loader, m.repoPath),
	)
```

- [ ] **Step 6: Make the live indicator repo-aware**

In `internal/ui/tui/model_view.go`, replace (lines ~90–93):

```go
	liveCell := ""
	if m.watching {
		liveCell = m.liveStyle.Render("● LIVE")
	}
```

with:

```go
	liveCell := ""
	// k8s categories: live while their watch is established (m.watching).
	// repo categories: live while the always-on working-tree watch runs.
	if m.watching || (m.repoWatching && !m.category.NeedsKubeConfig()) {
		liveCell = m.liveStyle.Render("● LIVE")
	}
```

- [ ] **Step 7: Run the tests to verify they pass**

Run: `go test ./internal/ui/tui/ -run 'HandleRepoWatch|HandleDatasetReloaded|LiveIndicator_ShowsOnRepo' -v`
Expected: PASS.

- [ ] **Step 8: Run the full affected suites**

Run: `go test ./internal/ui/tui/... ./internal/infra/... ./pkg/models/...`
Expected: PASS (no regressions in the existing watch / dataset / view tests).

- [ ] **Step 9: Lint**

Run: `golangci-lint run ./internal/ui/tui/... ./internal/infra/fswatch/... ./internal/infra/loader/... ./pkg/models/...`
Expected: 0 issues.

- [ ] **Step 10: Commit**

```bash
git add internal/ui/tui/reducer_repowatch.go internal/ui/tui/reducer_repowatch_test.go internal/ui/tui/model_update.go internal/ui/tui/model.go internal/ui/tui/model_view.go
git commit -m "feat(tui): live repo categories via always-on working-tree watch"
```

---

## Self-Review

**Spec coverage:**
- Single dataset-level watch → Task 1 (`fswatch`), Task 5 (`reloadDatasetCmd` re-runs full `LoadDataset`). ✓
- Always-on for the session → Task 5 Step 5 (`Init` on `sessionCtx`), no teardown on navigation. ✓
- Reuse 5 s `DebounceWindow` → Task 3 Step 4. ✓
- Include GPUPool → Task 4 `reloadGPUPoolsCmd`, Task 5 `handleGPUPoolsReloaded` + trigger batch. ✓
- Exclude `.git`/dot-dirs → Task 1 `isHidden` + `addRecursive` (`SkipDir`) + event guard. ✓
- `RepoWatcher` optional capability → Task 3. ✓
- Merge preserving k8s fields → Task 2. ✓
- Live indicator on repo categories → Task 5 Step 6. ✓
- Quiet error handling (warn, no toast) → Task 4 `reloadDatasetCmd`/`reloadGPUPoolsCmd`. ✓
- Watcher death drops indicator → Task 5 `handleRepoWatchClosed`. ✓

**Type consistency:** message names, `repoTrigger`/`repoWatching`, `sessionCtx()`, and the four command signatures are identical across Tasks 4 and 5. `MergeReloadedRepoData(fresh *Dataset)` matches between Task 2 and its caller in Task 5. ✓

**Deviation from spec:** the shared-`watchutil` extraction is replaced by a self-contained debounce in `fswatch` (documented in the File Structure note), per the spec's sanctioned fallback. ✓
