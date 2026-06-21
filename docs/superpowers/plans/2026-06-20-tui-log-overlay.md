# TUI Log Overlay Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a toggle-able, full-screen log overlay to the TUI (backtick `` ` ``) so the operator can watch the application's log stream live during critical operations.

**Architecture:** An in-memory `RingSink` implements the existing `logging.Logger` interface; a `Tee` fans every log call to both the file logger and the ring. Only the TUI entry path tees. The TUI adds a `LogView` view mode (mirroring `HelpView`) that renders the ring's contents in a `viewport`, refreshed by a ~400 ms tick while open, auto-following the newest line with pause-on-scroll.

**Tech Stack:** Go 1.26, Bubble Tea, Bubbles (`viewport`, `key`), Lipgloss, zap/slog (existing logging), testify (`assert`) for TUI tests.

## Global Constraints

- Module path: `github.com/jingle2008/toolkit`. Go `1.26.1`.
- Logger abstraction is `pkg/infra/logging.Logger` (methods: `Debugw`, `Infow`, `Warnw`, `Errorw`, `WithFields(...) Logger`, `DebugEnabled() bool`, `Sync() error`). New loggers MUST implement it fully.
- `logging` package tests use the standard library `testing` only (match `logging_test.go`). TUI tests use `github.com/stretchr/testify/assert` and `t.Parallel()` (match `model_update_test.go`).
- Run `gofmt` and `golangci-lint run <pkg>` on touched packages before each commit; both must be clean.
- Do NOT change `get`/`mcp`/`mutate` logging — they stay file-only.
- Toggle key is backtick `` ` ``. Overlay is available only from `ListView`/`DetailsView`.

---

### Task 1: `RingSink` — in-memory log capture

**Files:**
- Create: `pkg/infra/logging/ring.go`
- Test: `pkg/infra/logging/ring_test.go`

**Interfaces:**
- Consumes: the existing `Logger` interface (same package).
- Produces: `type Level int8` with `LevelDebug, LevelInfo, LevelWarn, LevelError`; `type Entry struct { Time time.Time; Level Level; Message string; Fields []any }`; `func NewRingSink(capacity int) *RingSink`; `func (r *RingSink) Snapshot() []Entry`; and `*RingSink` satisfies `Logger`.

- [ ] **Step 1: Write the failing test**

Create `pkg/infra/logging/ring_test.go`:

```go
package logging

import (
	"sync"
	"testing"
)

func TestRingSink_AppendAndSnapshot(t *testing.T) {
	r := NewRingSink(10)
	r.Infow("hello", "k", "v")
	r.Warnw("watch out")
	snap := r.Snapshot()
	if len(snap) != 2 {
		t.Fatalf("got %d entries, want 2", len(snap))
	}
	if snap[0].Message != "hello" || snap[0].Level != LevelInfo {
		t.Errorf("entry 0 = %+v", snap[0])
	}
	if snap[1].Level != LevelWarn {
		t.Errorf("entry 1 level = %v, want Warn", snap[1].Level)
	}
}

func TestRingSink_WrapsOnOverflow(t *testing.T) {
	r := NewRingSink(3)
	for i := 0; i < 5; i++ {
		r.Debugw("m")
	}
	snap := r.Snapshot()
	if len(snap) != 3 {
		t.Fatalf("got %d, want 3 (capacity)", len(snap))
	}
}

func TestRingSink_SnapshotIsCopy(t *testing.T) {
	r := NewRingSink(4)
	r.Infow("a")
	snap := r.Snapshot()
	snap[0].Message = "mutated"
	if got := r.Snapshot(); got[0].Message != "a" {
		t.Errorf("snapshot mutation leaked into ring: %q", got[0].Message)
	}
}

func TestRingSink_WithFieldsPrepends(t *testing.T) {
	r := NewRingSink(4)
	r.WithFields("req", "1").Infow("hi", "k", "v")
	snap := r.Snapshot()
	if len(snap) != 1 || len(snap[0].Fields) != 4 {
		t.Fatalf("fields = %+v", snap[0].Fields)
	}
	if snap[0].Fields[0] != "req" || snap[0].Fields[2] != "k" {
		t.Errorf("field order wrong: %+v", snap[0].Fields)
	}
}

func TestRingSink_DebugEnabledAlwaysTrue(t *testing.T) {
	if !NewRingSink(1).DebugEnabled() {
		t.Error("RingSink.DebugEnabled() should be true")
	}
}

func TestRingSink_ConcurrentWrites(t *testing.T) {
	r := NewRingSink(1000)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); r.Infow("x") }()
	}
	wg.Wait()
	if len(r.Snapshot()) != 50 {
		t.Errorf("got %d, want 50", len(r.Snapshot()))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/infra/logging/ -run TestRingSink -v`
Expected: FAIL — `undefined: NewRingSink` (and related).

- [ ] **Step 3: Write the implementation**

Create `pkg/infra/logging/ring.go`:

```go
package logging

import (
	"sync"
	"time"
)

// Level identifies the severity of a captured log entry.
type Level int8

// Severity levels for captured entries, ascending.
const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

// Entry is a single captured log record.
type Entry struct {
	Time    time.Time
	Level   Level
	Message string
	Fields  []any // alternating key, value — as passed to the ...w methods
}

// RingSink is a Logger that retains the most recent entries in a
// fixed-capacity, mutex-guarded ring buffer. It is safe for concurrent
// writes from loader / watch goroutines. Snapshot returns a copy for
// rendering. DebugEnabled is always true so it captures every level
// regardless of the file logger's configured level.
type RingSink struct {
	mu    sync.Mutex
	buf   []Entry
	cap   int
	start int
	size  int
}

// NewRingSink returns a RingSink retaining the last capacity entries.
func NewRingSink(capacity int) *RingSink {
	if capacity < 1 {
		capacity = 1
	}
	return &RingSink{buf: make([]Entry, capacity), cap: capacity}
}

func (r *RingSink) record(level Level, msg string, fields []any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.buf[(r.start+r.size)%r.cap] = Entry{
		Time: time.Now(), Level: level, Message: msg, Fields: fields,
	}
	if r.size < r.cap {
		r.size++
	} else {
		r.start = (r.start + 1) % r.cap
	}
}

// Snapshot returns the retained entries oldest-first as a fresh slice.
func (r *RingSink) Snapshot() []Entry {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Entry, r.size)
	for i := 0; i < r.size; i++ {
		out[i] = r.buf[(r.start+i)%r.cap]
	}
	return out
}

func (r *RingSink) Debugw(msg string, kv ...any) { r.record(LevelDebug, msg, kv) }
func (r *RingSink) Infow(msg string, kv ...any)  { r.record(LevelInfo, msg, kv) }
func (r *RingSink) Warnw(msg string, kv ...any)  { r.record(LevelWarn, msg, kv) }
func (r *RingSink) Errorw(msg string, kv ...any) { r.record(LevelError, msg, kv) }

// WithFields returns a Logger that records into the same ring with kv
// prepended to every entry's fields.
func (r *RingSink) WithFields(kv ...any) Logger {
	return &ringView{sink: r, fields: append([]any(nil), kv...)}
}

// DebugEnabled reports true: the ring always captures debug entries.
func (r *RingSink) DebugEnabled() bool { return true }

// Sync is a no-op (in-memory).
func (r *RingSink) Sync() error { return nil }

// ringView is a RingSink wrapper that prepends accumulated fields.
type ringView struct {
	sink   *RingSink
	fields []any
}

func (v *ringView) join(kv []any) []any {
	if len(v.fields) == 0 {
		return kv
	}
	out := make([]any, 0, len(v.fields)+len(kv))
	out = append(out, v.fields...)
	out = append(out, kv...)
	return out
}

func (v *ringView) Debugw(msg string, kv ...any) { v.sink.record(LevelDebug, msg, v.join(kv)) }
func (v *ringView) Infow(msg string, kv ...any)  { v.sink.record(LevelInfo, msg, v.join(kv)) }
func (v *ringView) Warnw(msg string, kv ...any)  { v.sink.record(LevelWarn, msg, v.join(kv)) }
func (v *ringView) Errorw(msg string, kv ...any) { v.sink.record(LevelError, msg, v.join(kv)) }
func (v *ringView) WithFields(kv ...any) Logger {
	return &ringView{sink: v.sink, fields: v.join(kv)}
}
func (v *ringView) DebugEnabled() bool { return true }
func (v *ringView) Sync() error        { return nil }
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./pkg/infra/logging/ -run TestRingSink -race -v`
Expected: PASS (all 6).

- [ ] **Step 5: Lint + commit**

```bash
gofmt -w pkg/infra/logging/ring.go pkg/infra/logging/ring_test.go
golangci-lint run ./pkg/infra/logging/...
git add pkg/infra/logging/ring.go pkg/infra/logging/ring_test.go
git commit -m "feat(logging): add in-memory RingSink Logger"
```

---

### Task 2: `Tee` — fan-out logger

**Files:**
- Create: `pkg/infra/logging/tee.go`
- Test: `pkg/infra/logging/tee_test.go`

**Interfaces:**
- Consumes: `Logger`, `*RingSink` (Task 1).
- Produces: `func NewTee(loggers ...Logger) Logger`.

- [ ] **Step 1: Write the failing test**

Create `pkg/infra/logging/tee_test.go`:

```go
package logging

import (
	"errors"
	"testing"
)

type countingLogger struct {
	infos int
	debug bool
	syncErr error
}

func (c *countingLogger) Debugw(string, ...any) {}
func (c *countingLogger) Infow(string, ...any)  { c.infos++ }
func (c *countingLogger) Warnw(string, ...any)  {}
func (c *countingLogger) Errorw(string, ...any) {}
func (c *countingLogger) WithFields(...any) Logger { return c }
func (c *countingLogger) DebugEnabled() bool        { return c.debug }
func (c *countingLogger) Sync() error               { return c.syncErr }

func TestTee_ForwardsToAll(t *testing.T) {
	a, b := &countingLogger{}, &countingLogger{}
	NewTee(a, b).Infow("hi")
	if a.infos != 1 || b.infos != 1 {
		t.Errorf("infos a=%d b=%d, want 1,1", a.infos, b.infos)
	}
}

func TestTee_WithFieldsFansOut(t *testing.T) {
	r := NewRingSink(4)
	c := &countingLogger{}
	NewTee(c, r).WithFields("k", "v").Infow("hi")
	if c.infos != 1 {
		t.Errorf("child not called via WithFields tee")
	}
	if snap := r.Snapshot(); len(snap) != 1 || len(snap[0].Fields) != 2 {
		t.Errorf("ring did not receive fielded entry: %+v", snap)
	}
}

func TestTee_DebugEnabledIsOr(t *testing.T) {
	if !NewTee(&countingLogger{debug: false}, &countingLogger{debug: true}).DebugEnabled() {
		t.Error("DebugEnabled should be true if any child is debug-enabled")
	}
	if NewTee(&countingLogger{}, &countingLogger{}).DebugEnabled() {
		t.Error("DebugEnabled should be false if no child is debug-enabled")
	}
}

func TestTee_SyncJoinsErrors(t *testing.T) {
	boom := errors.New("boom")
	if err := NewTee(&countingLogger{syncErr: boom}, &countingLogger{}).Sync(); !errors.Is(err, boom) {
		t.Errorf("Sync did not propagate child error: %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/infra/logging/ -run TestTee -v`
Expected: FAIL — `undefined: NewTee`.

- [ ] **Step 3: Write the implementation**

Create `pkg/infra/logging/tee.go`:

```go
package logging

import "errors"

// tee forwards every log call to all wrapped loggers.
type tee struct {
	loggers []Logger
}

// NewTee returns a Logger that fans every call out to all loggers.
func NewTee(loggers ...Logger) Logger {
	return &tee{loggers: loggers}
}

func (t *tee) Debugw(msg string, kv ...any) {
	for _, l := range t.loggers {
		l.Debugw(msg, kv...)
	}
}

func (t *tee) Infow(msg string, kv ...any) {
	for _, l := range t.loggers {
		l.Infow(msg, kv...)
	}
}

func (t *tee) Warnw(msg string, kv ...any) {
	for _, l := range t.loggers {
		l.Warnw(msg, kv...)
	}
}

func (t *tee) Errorw(msg string, kv ...any) {
	for _, l := range t.loggers {
		l.Errorw(msg, kv...)
	}
}

func (t *tee) WithFields(kv ...any) Logger {
	next := make([]Logger, len(t.loggers))
	for i, l := range t.loggers {
		next[i] = l.WithFields(kv...)
	}
	return &tee{loggers: next}
}

func (t *tee) DebugEnabled() bool {
	for _, l := range t.loggers {
		if l.DebugEnabled() {
			return true
		}
	}
	return false
}

func (t *tee) Sync() error {
	var errs []error
	for _, l := range t.loggers {
		if err := l.Sync(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./pkg/infra/logging/ -run TestTee -v`
Expected: PASS (all 4).

- [ ] **Step 5: Lint + commit**

```bash
gofmt -w pkg/infra/logging/tee.go pkg/infra/logging/tee_test.go
golangci-lint run ./pkg/infra/logging/...
git add pkg/infra/logging/tee.go pkg/infra/logging/tee_test.go
git commit -m "feat(logging): add Tee fan-out Logger"
```

---

### Task 3: `LogView` mode, model fields, `WithLogStore` option

**Files:**
- Modify: `internal/ui/tui/common/view_mode.go` (add `LogView` enum + `String()` case)
- Modify: `internal/ui/tui/common/common_test.go` (add `LogView` row to the String test)
- Modify: `internal/ui/tui/model_state.go` (add fields + viewport default)
- Modify: `internal/ui/tui/options.go` (add `WithLogStore`)
- Test: `internal/ui/tui/options_test.go` (add `WithLogStore` test)

**Interfaces:**
- Consumes: `*logging.RingSink` (Task 1).
- Produces: `common.LogView` enum value; model fields `logStore *logging.RingSink`, `logViewport *viewport.Model`, `logReturnView common.ViewMode`; `func WithLogStore(s *logging.RingSink) ModelOption`.

- [ ] **Step 1: Add the enum value + String case**

In `internal/ui/tui/common/view_mode.go`, add `LogView` after `EditTenantView` in the `const` block:

```go
	// EditTenantView is the view mode for the tenant-metadata entry form.
	EditTenantView
	// LogView is the full-screen log overlay.
	LogView
```

And add to `String()` before `default`:

```go
	case EditTenantView:
		return "EditTenant"
	case LogView:
		return "Log"
```

- [ ] **Step 2: Update the String test (failing first)**

In `internal/ui/tui/common/common_test.go`, add to the `TestViewModeString` cases table:

```go
		{LogView, "Log"},
```

Run: `go test ./internal/ui/tui/common/ -run TestViewModeString -v`
Expected: PASS (enum + case added together compile and pass).

- [ ] **Step 3: Add model fields + viewport default**

In `internal/ui/tui/model_state.go`, add to the `Model` struct (near the other view fields):

```go
	// Log overlay state.
	logStore      *logging.RingSink
	logViewport   *viewport.Model
	logReturnView common.ViewMode // view to restore when the log overlay closes
```

In `setDefaults`, after the `m.viewport` block, add:

```go
	if m.logViewport == nil {
		lvp := viewport.New(20, 20)
		m.logViewport = &lvp
	}
```

(`viewport` and `logging` are already imported in `model_state.go`.)

- [ ] **Step 4: Add the `WithLogStore` option**

In `internal/ui/tui/options.go`, after `WithLogger`:

```go
// WithLogStore sets the in-memory log ring the log overlay renders from.
func WithLogStore(s *logging.RingSink) ModelOption {
	return func(m *Model) {
		m.logStore = s
	}
}
```

- [ ] **Step 5: Write the option test**

In `internal/ui/tui/options_test.go`, add (imports `logging` as needed):

```go
func TestWithLogStore(t *testing.T) {
	t.Parallel()
	ring := logging.NewRingSink(8)
	m := &Model{}
	WithLogStore(ring)(m)
	assert.Same(t, ring, m.logStore)
}
```

Run: `go test ./internal/ui/tui/ -run TestWithLogStore -v`
Expected: PASS.

- [ ] **Step 6: Build, lint, commit**

```bash
go build ./...
gofmt -w internal/ui/tui/common/view_mode.go internal/ui/tui/common/common_test.go internal/ui/tui/model_state.go internal/ui/tui/options.go internal/ui/tui/options_test.go
golangci-lint run ./internal/ui/tui/... ./internal/ui/tui/common/...
git add -A
git commit -m "feat(tui): add LogView mode, log-store model fields, WithLogStore option"
```

---

### Task 4: `ToggleLog` keybinding

**Files:**
- Modify: `internal/ui/tui/keys/registry.go` (add `ToggleLog` var + append to `globalKeys`)
- Test: `internal/ui/tui/keys/registry_test.go` (assert the binding resolves)

**Interfaces:**
- Produces: package var `keys.ToggleLog key.Binding` (keys `` ` ``).

- [ ] **Step 1: Write the failing test**

In `internal/ui/tui/keys/registry_test.go`, add:

```go
func TestToggleLogInGlobalKeys(t *testing.T) {
	t.Parallel()
	km := ResolveKeys(domain.Tenant, common.ListView)
	found := false
	for _, b := range km.Global {
		if key.Matches(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'`'}}, b) {
			found = true
		}
	}
	if !found {
		t.Error("backtick (ToggleLog) not present in global keys")
	}
}
```

Ensure the test file imports `tea "github.com/charmbracelet/bubbletea"`, `"github.com/charmbracelet/bubbles/key"`, `"github.com/jingle2008/toolkit/internal/domain"`, and `"github.com/jingle2008/toolkit/internal/ui/tui/common"` (add any missing).

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ui/tui/keys/ -run TestToggleLogInGlobalKeys -v`
Expected: FAIL — backtick not present.

- [ ] **Step 3: Add the binding**

In `internal/ui/tui/keys/registry.go`, add to the global `var (...)` block (with `Quit`, `Help`, etc.):

```go
	ToggleLog = key.NewBinding(
		key.WithKeys("`"),
		key.WithHelp("<`>", "Toggle Log"),
	)
```

And add `ToggleLog` to the `globalKeys` slice (before `Quit`):

```go
var globalKeys = []key.Binding{
	Help,
	ViewDetails,
	CopyName,
	ToggleLog,
	Back,
	Quit,
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/ui/tui/keys/ -v`
Expected: PASS (new test + existing registry-conflict test still green — backtick collides with nothing).

- [ ] **Step 5: Lint + commit**

```bash
gofmt -w internal/ui/tui/keys/registry.go internal/ui/tui/keys/registry_test.go
golangci-lint run ./internal/ui/tui/keys/...
git add internal/ui/tui/keys/registry.go internal/ui/tui/keys/registry_test.go
git commit -m "feat(tui): add backtick ToggleLog global keybinding"
```

---

### Task 5: Log entry formatting + overlay render

**Files:**
- Create: `internal/ui/tui/log_view.go` (formatting + `logView()` render; `updateLogView`/tick added in Task 6)
- Test: `internal/ui/tui/log_view_test.go`

**Interfaces:**
- Consumes: `logging.Entry`, `logging.Level*` (Task 1); model fields (Task 3); existing `truncateString` (in `model_view.go`).
- Produces: `func formatLogEntry(e logging.Entry) string`; `func (m *Model) logView() string`; `func (m *Model) renderLogEntries(width int) string`.

- [ ] **Step 1: Write the failing test**

Create `internal/ui/tui/log_view_test.go`:

```go
package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	logging "github.com/jingle2008/toolkit/pkg/infra/logging"
)

func TestFormatLogEntry(t *testing.T) {
	t.Parallel()
	e := logging.Entry{
		Time:    time.Date(2026, 6, 20, 15, 4, 5, 0, time.UTC),
		Level:   logging.LevelInfo,
		Message: "mutation begin",
		Fields:  []any{"action", "drain"},
	}
	assert.Equal(t, "15:04:05 INFO  mutation begin action=drain", formatLogEntry(e))
}

func TestRenderLogEntries_Empty(t *testing.T) {
	t.Parallel()
	m := &Model{logStore: logging.NewRingSink(4)}
	assert.Contains(t, m.renderLogEntries(80), "no log entries")
}

func TestRenderLogEntries_NilStore(t *testing.T) {
	t.Parallel()
	m := &Model{}
	assert.NotPanics(t, func() { _ = m.renderLogEntries(80) })
}

func TestRenderLogEntries_OrdersOldestToNewest(t *testing.T) {
	t.Parallel()
	ring := logging.NewRingSink(8)
	ring.Infow("first")
	ring.Errorw("second")
	m := &Model{logStore: ring}
	out := m.renderLogEntries(120)
	assert.Less(t, strings.Index(out, "first"), strings.Index(out, "second"))
	assert.Contains(t, out, "ERROR")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ui/tui/ -run 'TestFormatLogEntry|TestRenderLogEntries' -v`
Expected: FAIL — `formatLogEntry` / `renderLogEntries` undefined.

- [ ] **Step 3: Write the implementation**

Create `internal/ui/tui/log_view.go`:

```go
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	logging "github.com/jingle2008/toolkit/pkg/infra/logging"
)

// levelLabel returns the fixed-width display label for a log level.
func levelLabel(l logging.Level) string {
	switch l {
	case logging.LevelDebug:
		return "DEBUG"
	case logging.LevelInfo:
		return "INFO"
	case logging.LevelWarn:
		return "WARN"
	case logging.LevelError:
		return "ERROR"
	default:
		return "?"
	}
}

// levelStyle returns the color style for a log level.
func levelStyle(l logging.Level) lipgloss.Style {
	switch l {
	case logging.LevelWarn:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	case logging.LevelError:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	case logging.LevelDebug:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	default:
		return lipgloss.NewStyle()
	}
}

// formatLogEntry renders one entry as a single uncolored line:
// "15:04:05 LEVEL message k=v k=v".
func formatLogEntry(e logging.Entry) string {
	var b strings.Builder
	b.WriteString(e.Time.Format("15:04:05"))
	b.WriteByte(' ')
	fmt.Fprintf(&b, "%-5s ", levelLabel(e.Level))
	b.WriteString(e.Message)
	for i := 0; i+1 < len(e.Fields); i += 2 {
		fmt.Fprintf(&b, " %v=%v", e.Fields[i], e.Fields[i+1])
	}
	return b.String()
}

// renderLogEntries builds the overlay body: one color-coded, width-
// truncated line per entry, oldest first.
func (m *Model) renderLogEntries(width int) string {
	if m.logStore == nil {
		return "(log store unavailable)"
	}
	entries := m.logStore.Snapshot()
	if len(entries) == 0 {
		return "(no log entries yet)"
	}
	lines := make([]string, len(entries))
	for i, e := range entries {
		lines[i] = levelStyle(e.Level).Render(truncateString(formatLogEntry(e), width))
	}
	return strings.Join(lines, "\n")
}

// logView renders the full-screen log overlay: a title line showing the
// follow/pause state, the scrollable body, and a key hint footer. It
// refreshes the viewport from the latest ring snapshot each render and,
// while the user is at the bottom, keeps the newest line in view
// (auto-follow). Scrolling up leaves the offset alone (pause).
func (m *Model) logView() string {
	width := m.viewWidth
	bodyHeight := m.viewHeight - 2 // title + hint lines
	if bodyHeight < 1 {
		bodyHeight = 1
	}
	m.logViewport.Width = width
	m.logViewport.Height = bodyHeight

	follow := m.logViewport.AtBottom()
	m.logViewport.SetContent(m.renderLogEntries(width))
	if follow {
		m.logViewport.GotoBottom()
	}

	state := "following"
	if !m.logViewport.AtBottom() {
		state = "PAUSED"
	}
	title := lipgloss.NewStyle().Bold(true).Render(fmt.Sprintf("LOG — %s", state))
	hint := lipgloss.NewStyle().Foreground(lipgloss.Color("245")).
		Render("↑↓/pgup/pgdn scroll · end follow · home top · ` close")
	return lipgloss.JoinVertical(lipgloss.Left, title, m.logViewport.View(), hint)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/ui/tui/ -run 'TestFormatLogEntry|TestRenderLogEntries' -v`
Expected: PASS (all 4).

- [ ] **Step 5: Build, lint, commit**

```bash
go build ./...
gofmt -w internal/ui/tui/log_view.go internal/ui/tui/log_view_test.go
golangci-lint run ./internal/ui/tui/...
git add internal/ui/tui/log_view.go internal/ui/tui/log_view_test.go
git commit -m "feat(tui): render log overlay entries with level colors and follow state"
```

---

### Task 6: Toggle, dispatch, scroll handling, and live tick

**Files:**
- Modify: `internal/ui/tui/log_view.go` (add `logTickMsg`, `logTickCmd`, `updateLogView`)
- Modify: `internal/ui/tui/model_update.go` (toggle in `delegateToActiveView`, `LogView` dispatch, top-level `logTickMsg` case)
- Modify: `internal/ui/tui/model_view.go` (`renderActiveView` `LogView` case)
- Test: `internal/ui/tui/log_view_test.go` (add behavior tests)

**Interfaces:**
- Consumes: `keys.ToggleLog`/`keys.Back`/`keys.Quit`, `common.LogView`, `m.logReturnView`, `m.logViewport`, `logView()` (Tasks 3–5).
- Produces: `type logTickMsg struct{}`; `func logTickCmd() tea.Cmd`; `func (m *Model) updateLogView(msg tea.Msg) (tea.Model, tea.Cmd)`.

- [ ] **Step 1: Write the failing behavior tests**

Append to `internal/ui/tui/log_view_test.go` (add imports: `tea "github.com/charmbracelet/bubbletea"`, `"github.com/jingle2008/toolkit/internal/domain"`, `"github.com/jingle2008/toolkit/internal/ui/tui/common"`, `"github.com/jingle2008/toolkit/pkg/models"`):

```go
func newLogModel(t *testing.T, ring *logging.RingSink) *Model {
	t.Helper()
	m, err := NewModel(
		WithRepoPath("repo"),
		WithEnvironment(models.Environment{Type: "dev", Region: "us-phx-1", Realm: "oc1"}),
		WithCategory(domain.Tenant),
		WithLoader(fakeLoader{}),
		WithLogger(logging.NewNoOpLogger()),
		WithLogStore(ring),
	)
	if err != nil {
		t.Fatalf("NewModel: %v", err)
	}
	m.viewWidth, m.viewHeight = 80, 12
	return m
}

var backtick = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'`'}}

func TestLogOverlay_ToggleFromList(t *testing.T) {
	t.Parallel()
	m := newLogModel(t, logging.NewRingSink(16))
	m.viewMode = common.ListView
	_, cmd := m.Update(backtick)
	assert.Equal(t, common.LogView, m.viewMode)
	assert.Equal(t, common.ListView, m.logReturnView)
	assert.NotNil(t, cmd) // live-refresh tick started
	// Toggle again closes back to the originating view.
	_, _ = m.Update(backtick)
	assert.Equal(t, common.ListView, m.viewMode)
}

func TestLogOverlay_ReturnsToDetails(t *testing.T) {
	t.Parallel()
	m := newLogModel(t, logging.NewRingSink(16))
	m.viewMode = common.DetailsView
	_, _ = m.Update(backtick)
	assert.Equal(t, common.LogView, m.viewMode)
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	assert.Equal(t, common.DetailsView, m.viewMode)
}

func TestLogOverlay_PauseAndResume(t *testing.T) {
	t.Parallel()
	ring := logging.NewRingSink(200)
	for i := 0; i < 100; i++ {
		ring.Infow("line")
	}
	m := newLogModel(t, ring)
	m.viewMode = common.ListView
	_, _ = m.Update(backtick) // open → follows
	assert.Contains(t, m.View(), "following")

	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgUp}) // scroll up → pause
	assert.Contains(t, m.View(), "PAUSED")

	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnd}) // back to bottom → follow
	assert.Contains(t, m.View(), "following")
}

func TestLogOverlay_TickStopsWhenClosed(t *testing.T) {
	t.Parallel()
	m := newLogModel(t, logging.NewRingSink(8))
	m.viewMode = common.ListView // not in LogView
	_, cmd := m.Update(logTickMsg{})
	assert.Nil(t, cmd) // tick does not re-arm outside LogView
}

func TestLogOverlay_NilStoreDoesNotPanic(t *testing.T) {
	t.Parallel()
	m, _ := NewModel(
		WithRepoPath("repo"),
		WithEnvironment(models.Environment{Type: "dev", Region: "us-phx-1", Realm: "oc1"}),
		WithLoader(fakeLoader{}),
		WithLogger(logging.NewNoOpLogger()),
	)
	m.viewWidth, m.viewHeight = 80, 12
	m.viewMode = common.ListView
	assert.NotPanics(t, func() {
		_, _ = m.Update(backtick)
		_ = m.View()
	})
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/ui/tui/ -run TestLogOverlay -v`
Expected: FAIL — `logTickMsg`/`updateLogView` undefined and toggle not wired (viewMode stays `ListView`).

- [ ] **Step 3: Add tick + `updateLogView` to `log_view.go`**

Append to `internal/ui/tui/log_view.go` (add imports `time`, `tea "github.com/charmbracelet/bubbletea"`, `"github.com/charmbracelet/bubbles/key"`, `keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"`):

```go
// logTickMsg drives periodic re-renders of the log overlay so the live
// tail updates even while the app is otherwise idle.
type logTickMsg struct{}

const logRefreshInterval = 400 * time.Millisecond

// logTickCmd schedules the next log-overlay refresh.
func logTickCmd() tea.Cmd {
	return tea.Tick(logRefreshInterval, func(time.Time) tea.Msg { return logTickMsg{} })
}

// updateLogView handles input while the log overlay is open: close keys,
// quit, the home/end follow controls (the viewport keymap lacks them),
// and otherwise forwards scrolling to the viewport.
func (m *Model) updateLogView(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(km, keys.ToggleLog, keys.Back):
			m.viewMode = m.logReturnView
			return m, nil
		case key.Matches(km, keys.Quit):
			m.cancelInFlight()
			return m, tea.Quit
		}
		switch km.String() {
		case "end":
			m.logViewport.GotoBottom()
			return m, nil
		case "home":
			m.logViewport.SetYOffset(0)
			return m, nil
		}
	}
	vp, cmd := m.logViewport.Update(msg)
	m.logViewport = &vp
	return m, cmd
}
```

- [ ] **Step 4: Wire toggle + dispatch in `model_update.go`**

In `internal/ui/tui/model_update.go`, add a top-level case in the `Update` switch (next to `spinner.TickMsg`):

```go
	case logTickMsg:
		if m.viewMode == common.LogView {
			return m, logTickCmd()
		}
		return m, nil
```

Replace `delegateToActiveView` so it intercepts the toggle before the per-view switch and adds the `LogView` case:

```go
func (m *Model) delegateToActiveView(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok && key.Matches(km, keys.ToggleLog) &&
		(m.viewMode == common.ListView || m.viewMode == common.DetailsView) {
		m.logReturnView = m.viewMode
		m.viewMode = common.LogView
		return m, logTickCmd()
	}
	switch m.viewMode {
	case common.HelpView:
		return m.updateHelpView(msg)
	case common.ListView:
		return m.updateListView(msg)
	case common.DetailsView:
		return m.updateDetailView(msg)
	case common.LoadingView:
		return m.updateLoadingView(msg)
	case common.ExportView:
		return m.updateExportView(msg)
	case common.LogView:
		return m.updateLogView(msg)
```

(Keep the remaining existing cases — `EditTenantView` and `default` — unchanged.) Add imports to `model_update.go`: `"github.com/charmbracelet/bubbles/key"` and `keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"`.

- [ ] **Step 5: Add the render case in `model_view.go`**

In `renderActiveView`'s switch, add before `default`:

```go
	case common.LogView:
		return m.logView()
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./internal/ui/tui/ -run TestLogOverlay -v`
Expected: PASS (all 5).

- [ ] **Step 7: Build, lint, commit**

```bash
go build ./...
gofmt -w internal/ui/tui/log_view.go internal/ui/tui/model_update.go internal/ui/tui/model_view.go internal/ui/tui/log_view_test.go
golangci-lint run ./internal/ui/tui/...
git add -A
git commit -m "feat(tui): toggle log overlay, scroll handling, and live refresh tick"
```

---

### Task 7: Wire the tee + ring into the TUI entry path

**Files:**
- Modify: `internal/cli/root.go` (`runToolkit`: build ring + tee, pass `WithLogStore`)

**Interfaces:**
- Consumes: `logging.NewRingSink`, `logging.NewTee` (Tasks 1–2); `tui.WithLogStore` (Task 3).

- [ ] **Step 1: Build the ring + tee in `runToolkit`**

In `internal/cli/root.go`, inside `runToolkit`, replace the existing context-attach line:

```go
	ctx = logging.WithContext(ctx, logger)
```

with:

```go
	// Capture the log stream in memory (all levels) so the TUI's log
	// overlay can render it live, while the file logger keeps writing.
	// The ring is teed raw, so on-screen lines omit the cmd/version
	// correlation prefix the file log carries.
	ring := logging.NewRingSink(1000)
	logger = logging.NewTee(logger, ring)
	ctx = logging.WithContext(ctx, logger)
```

And add `tui.WithLogStore(ring)` to the `tui.NewModel(...)` option list (next to `tui.WithLogger(logger)`):

```go
		tui.WithLogger(logger),
		tui.WithLogStore(ring),
```

- [ ] **Step 2: Build + vet**

Run: `go build ./... && go vet ./...`
Expected: clean (no output).

- [ ] **Step 3: Full test suite + lint**

Run:
```bash
go test ./... 2>&1 | grep -vE '^ok|no test files'
golangci-lint run ./...
```
Expected: no failures printed; `0 issues`.

- [ ] **Step 4: Manual smoke check**

Run the TUI against a configured environment, press `` ` `` to open the overlay, confirm: log lines appear and tail live during a refresh (`r`); PgUp shows `PAUSED`; `end` resumes `following`; `` ` `` / `esc` closes back to the prior view. (If no live environment is available, note this step as skipped — the behavior is covered by Task 6 tests.)

- [ ] **Step 5: Commit**

```bash
git add internal/cli/root.go
git commit -m "feat(tui): wire in-memory log ring into the TUI for the log overlay"
```

---

## Self-Review

**Spec coverage:**
- RingSink (all levels, 1000, Snapshot, WithFields, DebugEnabled=true, concurrency) → Task 1. ✔
- Tee (fan-out, WithFields, DebugEnabled OR, Sync join) → Task 2. ✔
- LogView enum + model fields + WithLogStore → Task 3. ✔
- Backtick ToggleLog (Global) → Task 4. ✔
- Render (newest-at-bottom, level colors, follow/PAUSED title, truncation) → Task 5. ✔
- Toggle from List/Details via `delegateToActiveView`, dedicated `logReturnView`, dispatch, `View()` case, ~400 ms tick re-arming only in LogView, home/end follow controls, nil-store safety → Task 6. ✔
- Wiring tee+ring in `runToolkit` (TUI only; `get`/`mcp`/`mutate` untouched) → Task 7. ✔
- Out-of-scope items (search/resize/persist) → not implemented, by design. ✔

**Placeholder scan:** none — every code/test step contains complete code and exact commands.

**Type consistency:** `RingSink`/`Entry`/`Level*`/`NewRingSink`/`Snapshot` (Task 1) used identically in Tasks 2/5/6; `NewTee` (Task 2) used in Task 7; `WithLogStore`/`logStore`/`logViewport`/`logReturnView`/`common.LogView` (Task 3) used consistently in Tasks 5/6/7; `formatLogEntry`/`renderLogEntries`/`logView`/`updateLogView`/`logTickMsg`/`logTickCmd` names match across Tasks 5/6. `keys.ToggleLog` (Task 4) used in Task 6.
