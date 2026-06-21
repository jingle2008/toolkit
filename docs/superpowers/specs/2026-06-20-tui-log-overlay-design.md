# TUI Log Overlay — Design

**Date:** 2026-06-20
**Status:** Approved (pending implementation plan)
**Topic:** A toggle-able, full-screen log overlay in the TUI so the operator can see
what the application is doing during critical operations (mutations, loads, watches).

## Problem

During a TUI session the logger writes only to the rotating file (`cfg.LogFile`):
stdout is reserved for command output and the process's stderr is redirected to a
sibling capture file for the duration of the alt-screen session. As a result, **nothing
the app logs is visible on screen**. When an operator runs a critical operation —
cordon, drain, scale, delete, or watches a live reload — they have no in-app view of
progress, warnings, or failures; they must `tail -f` the log file in another terminal.

## Goal

Add an in-TUI, toggle-able log view that shows the application's log stream live, so
the operator can watch critical operations as they happen without leaving the TUI.

## Decisions (from brainstorming)

- **Layout:** full-screen overlay, toggled on/off — mirrors the existing `HelpView` /
  `DetailsView` pattern. (A bottom split pane was considered and rejected: it creates a
  dual-focus ambiguity where scroll keys are silently owned by the pane while it's open,
  leaving the table not-fully-functional with no visible focus cue.)
- **Content:** capture **everything** (Debug and above) in memory, independent of the
  file logger's configured level. The on-screen view always shows maximum detail.
- **Live behavior:** auto-follow the newest line; if the user scrolls up, follow pauses
  (title shows `PAUSED`); returning to the bottom resumes following.
- **Toggle key:** backtick `` ` `` ("drop-down console" convention; never collides with a
  typed filter/command).
- **Scrolling:** because the overlay owns the whole screen while open, **all** nav keys
  (`↑`/`↓`, `pgup`/`pgdown`, `home`/`end`) scroll the log. No table-keymap conflict, no
  key interception, no focus model.

## Architecture

Two layers, cleanly separated:

1. **Capture (logging package):** an in-memory ring buffer that implements the existing
   `logging.Logger` interface, plus a tee that fans log calls out to both the file logger
   and the ring. This keeps capture as reusable, testable primitives with **zero coupling**
   to Bubble Tea, and leaves the zap/slog file backends untouched.

2. **Presentation (TUI):** a new `LogView` view mode that renders the ring's contents in a
   full-screen viewport, refreshed live while open.

### Capture: `RingSink` + `Tee`

`pkg/infra/logging/ring.go`:

```go
// Level identifies the severity of a captured entry.
type Level int8
const ( LevelDebug Level = iota; LevelInfo; LevelWarn; LevelError )

// Entry is one captured log record.
type Entry struct {
    Time    time.Time
    Level   Level
    Message string
    Fields  []any // alternating key, value (as passed to ...w)
}

// RingSink is a Logger that retains the most recent N entries in a
// fixed-capacity, mutex-guarded ring. Safe for concurrent writes from
// loader/watch goroutines; Snapshot returns a copy for rendering.
type RingSink struct { /* mu, buf []Entry, capacity, start, size, fields []any */ }

func NewRingSink(capacity int) *RingSink
func (r *RingSink) Snapshot() []Entry        // oldest→newest, copied
func (r *RingSink) Debugw(msg string, kv ...any)
func (r *RingSink) Infow(msg string, kv ...any)
func (r *RingSink) Warnw(msg string, kv ...any)
func (r *RingSink) Errorw(msg string, kv ...any)
func (r *RingSink) WithFields(kv ...any) logging.Logger // returns a sink view that prepends kv
func (r *RingSink) DebugEnabled() bool        // true — capture everything
func (r *RingSink) Sync() error               // no-op
```

- Capacity is fixed at **1000** entries (bounded memory; ~oldest dropped on overflow).
- `WithFields` returns a lightweight wrapper that records into the same underlying ring
  with the accumulated fields prepended to each entry. This exists primarily to satisfy
  the `Tee.WithFields` fan-out contract. The TUI wiring tees the **raw** ring (see below),
  so on-screen entries omit the always-`cmd=tui` / `version` correlation prefix — less
  per-line noise; the file log still carries it.
- `DebugEnabled()` returns `true` so debug breadcrumbs are captured regardless of the
  file level.

`pkg/infra/logging/tee.go`:

```go
// Tee forwards every call to all wrapped loggers.
type Tee struct { loggers []Logger }
func NewTee(loggers ...Logger) Logger
// Debugw/Infow/Warnw/Errorw → forward to each
// WithFields → NewTee(each.WithFields(kv...))
// DebugEnabled → true if any child is debug-enabled
// Sync → sync all, joining errors
```

### Wiring (TUI entry path only)

`internal/cli/root.go` (`NewRootCmd` RunE → the TUI path) is the **only** place that
builds the tee. `get`, `mcp`, and `mutate` keep the file-only logger.

`NewRootCmd` RunE keeps building the file logger and applying correlation fields exactly
as today. The ring + tee are constructed inside `runToolkit` (the TUI-only function),
which already receives that `logger` and is where `WithContext` + `NewModel` happen:

```go
// RunE (unchanged):
fileLogger, err := initLogger(cfg)
logger := fileLogger.WithFields("cmd", "tui", "version", version)
defer logger.Sync()
// ... runToolkit(ctx, logger, cfg, version)

// inside runToolkit, before the existing WithContext:
ring := logging.NewRingSink(1000)
logger = logging.NewTee(logger, ring)   // file + in-memory ring
ctx = logging.WithContext(ctx, logger)
// ...
model, err := tui.NewModel(
    /* ...existing options... */
    tui.WithLogger(logger),
    tui.WithLogStore(ring),  // NEW
)
```

`logger.Sync()` in RunE syncs the file logger (the ring's `Sync` is a no-op); the tee
created in `runToolkit` is not separately synced, which is fine — only the file backend
buffers.

`internal/ui/tui/options.go`: add `WithLogStore(*logging.RingSink) ModelOption` that sets
`m.logStore`.

Note: `logger.Sync()` already runs on shutdown; the tee's `Sync` covers the file logger
(the ring's `Sync` is a no-op).

### Presentation: `LogView`

`internal/ui/tui/common/view_mode.go`: add `LogView` to the enum and its `String()`
("Log").

`internal/ui/tui/log_view.go`:

- Model fields (added to `model_state.go`): `logStore *logging.RingSink`,
  `logViewport *viewport.Model` (created in `setDefaults`), and
  `logReturnView common.ViewMode` (the view to restore on close — see note below).
- `logView() string` — renders a full-screen bordered box: a title line
  (`LOG — following` / `LOG — PAUSED`, plus a hint `↑↓/pgup/pgdn scroll · end=follow ·
  \` close`) and the viewport body. The body is built from `logStore.Snapshot()`,
  formatted one entry per line:
  `15:04:05 LEVEL message k=v k=v`, truncated to width, with level color:
  DEBUG dim grey, INFO default, WARN yellow (`220`), ERROR red (`196`).
- Follow logic: before rendering, set the viewport content from the snapshot; if the
  viewport was at the bottom (`AtBottom()`), call `GotoBottom()` so it stays pinned to the
  newest line. If the user has scrolled up, leave the offset alone (paused).
- `updateLogView(msg) (tea.Model, tea.Cmd)`:
  - backtick (`ToggleLog`) or `keys.Back` (esc) → `m.viewMode = m.logReturnView` (close).
  - `keys.Quit` → quit (same as `updateHelpView`).
  - otherwise forward the message to `m.logViewport.Update` (handles `↑`/`↓`/`pgup`/
    `pgdown`/`home`/`end`).
  - returns the live-refresh tick command (below) so the overlay keeps updating.

**Dedicated return field.** LogView records the view to return to in its own
`m.logReturnView`, **not** the shared `m.lastViewMode`. `lastViewMode` is also written by
the loading flow (`reducer`: save→LoadingView→restore) and the help toggle; reusing it
risks a background load's save/restore clobbering the Log overlay's return target. A
dedicated field removes that coupling entirely.

### Toggle + dispatch (mirror HelpView)

- `keys/registry.go`: add a `Global` binding `ToggleLog` with keys `` ` `` and help
  `` <`> Toggle Log ``.
- `delegateToActiveView` (`model_update.go`): before the per-view switch, if the message is
  a `tea.KeyMsg` matching `ToggleLog` **and** `m.viewMode` is `ListView` or `DetailsView`,
  save `m.logReturnView = m.viewMode`, set `m.viewMode = common.LogView`, and return the
  refresh tick command. This opens the overlay uniformly from both content views; in the
  modal views (Help, Export, EditTenant, Loading) backtick falls through to those handlers
  and is ignored, consistent with how those modes capture input.
- `delegateToActiveView`: add `case common.LogView: return m.updateLogView(msg)` to the view
  dispatch switch.
- `model_view.go` `View()`: add `case common.LogView: return m.logView()`.

### Live refresh

While in `LogView`, the model schedules a `tea.Tick` (~400 ms) that emits a
`logTickMsg`. The handler, if still in `LogView`, re-issues the tick; `View()` reads the
latest snapshot each render. The tick **re-arms only while in LogView**, so it self-stops
on close/quit. When the overlay is closed, no tick runs — the ring keeps filling in the
background (the logger writes regardless), so reopening shows full history.

~400 ms latency is acceptable for "watch an operation." A push-based channel/`program.Send`
design was rejected as over-coupled for the benefit.

## Data flow

```
loader/watch/mutation goroutines
        │  logger.Infow/Warnw/Errorw/Debugw (logger == Tee in TUI)
        ▼
   Tee.fan-out ──► fileLogger (zap/slog → rotating file)   [unchanged]
        └────────► RingSink (append to ring, mutex-guarded)
                          ▲
        logTickMsg (~400ms while LogView) ──► View() ──► RingSink.Snapshot() ──► logViewport
```

## Error handling

- Ring writes never fail (in-memory, mutex-guarded); no error path to surface.
- A nil `logStore` (e.g., a test model built without `WithLogStore`) must be tolerated:
  the backtick toggle is a no-op and `logView()` renders an empty/"no logs" body. The
  TUI entry path always wires it, so this only affects tests.
- `Tee.Sync` joins child errors (the existing `zapLogger.Sync` already ignores the benign
  Windows `os.ErrInvalid`).

## Testing strategy

- **`logging` unit tests:** `RingSink` append/wrap-on-overflow, `Snapshot` returns a copy
  (mutating it doesn't affect the ring), concurrent writes are race-free (`-race`),
  `WithFields` prepends fields, `DebugEnabled()==true`. `Tee` forwards to all children,
  `WithFields`/`Sync` fan out, `DebugEnabled` is the OR of children.
- **TUI tests:** backtick from `ListView` opens `LogView` and closes back to `ListView`
  (via `logReturnView`); opening from `DetailsView` returns to `DetailsView`; `pgup` moves
  the viewport off-bottom so the title reads `PAUSED`; `end` returns to following. A render check of `logView()` from a seeded ring
  (asserts entries appear, newest last, level labels present). A nil-`logStore` model
  toggles without panicking.

## Out of scope (YAGNI)

In-overlay search/filter, drag-resize, per-entry expansion, persisting overlay state
across sessions, configurable ring capacity, color themes. The file log already persists
the full stream for after-the-fact inspection.

## Affected files

New: `pkg/infra/logging/ring.go`, `pkg/infra/logging/tee.go`,
`internal/ui/tui/log_view.go` (+ test files).
Changed: `internal/cli/root.go`, `internal/ui/tui/options.go`,
`internal/ui/tui/model_state.go`, `internal/ui/tui/common/view_mode.go`,
`internal/ui/tui/keys/registry.go`,
`internal/ui/tui/model_update.go` (toggle in `delegateToActiveView` + `LogView` dispatch),
`internal/ui/tui/model_view.go` (`View()` dispatch).
