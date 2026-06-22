# TUI Destructive-Action Confirmation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Gate the TUI's five destructive hotkeys behind a risk-tiered confirmation modal so an accidental keypress can no longer perform a live destructive operation.

**Architecture:** A new `common.ConfirmView` ViewMode with a `confirmOverlay` state struct on the `Model`, following the existing `EditTenantView`/`LogView` modal pattern. Destructive hotkeys in `handleItemActions` defer their command into the overlay instead of running it; `updateConfirmView` resolves the keypress (y/N for recoverable, capital-Y for irreversible) and `confirmView()` renders the modal.

**Tech Stack:** Go, Bubble Tea (`github.com/charmbracelet/bubbletea`), `github.com/charmbracelet/bubbles/key`, testify.

## Global Constraints

- Code must pass `gofumpt -l` (zero diff) and `golangci-lint run` (zero issues). Note `cyclop` max cyclomatic complexity is 10 — keep functions small.
- TDD: write the failing test first, watch it fail, then implement.
- Tests live in package `tui` and reuse the existing harness in `internal/ui/tui/model_test.go` (`NewModel(...)` with `fakeLoader{}` and `logging.NewNoOpLogger()`).
- New confirmation code goes in `internal/ui/tui/confirm.go` (logic) and `internal/ui/tui/confirm_test.go` (tests), keeping the feature in one focused file.
- Destructive actions and their tiers (from the spec):
  - Irreversible: delete DAC, terminate GPU node (both via the `ctrl+x` Delete key).
  - Recoverable: toggle cordon (`C`), drain (`D`), reboot (`R`), scale up (`U`).

---

### Task 1: ViewMode, tier type, and overlay state

**Files:**
- Modify: `internal/ui/tui/common/view_mode.go` (add `ConfirmView` enum value + `String()` case)
- Create: `internal/ui/tui/confirm.go` (tier type + overlay struct)
- Modify: `internal/ui/tui/model_state.go:94` (add `confirm confirmOverlay` field to the `Model` struct)
- Test: `internal/ui/tui/confirm_test.go`

**Interfaces:**
- Produces: `common.ConfirmView` (a `common.ViewMode`); `confirmTier` with constants `tierRecoverable`, `tierIrreversible`; `confirmOverlay` struct with fields `tier confirmTier`, `action string`, `kind string`, `target string`, `warning string`, `returnView common.ViewMode`, `run func() tea.Cmd`; `Model.confirm confirmOverlay`.

- [ ] **Step 1: Write the failing test**

Create `internal/ui/tui/confirm_test.go`:

```go
package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/jingle2008/toolkit/internal/ui/tui/common"
)

func TestConfirmView_String(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "Confirm", common.ConfirmView.String())
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ui/tui/ -run TestConfirmView_String`
Expected: FAIL — build error, `common.ConfirmView` undefined.

- [ ] **Step 3: Implement the types**

In `internal/ui/tui/common/view_mode.go`, add `ConfirmView` after `LogView` in the `const` block:

```go
	// LogView is the full-screen log overlay.
	LogView
	// ConfirmView is the modal that gates destructive actions.
	ConfirmView
```

and add its `String()` case after the `LogView` case:

```go
	case LogView:
		return "Log"
	case ConfirmView:
		return "Confirm"
```

Create `internal/ui/tui/confirm.go`:

```go
package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jingle2008/toolkit/internal/ui/tui/common"
)

// confirmTier classifies a destructive action by blast radius, which
// determines how the confirmation modal gates it.
type confirmTier int

const (
	// tierRecoverable actions can be undone or retried (cordon, drain,
	// reboot, scale); a single y/N confirms.
	tierRecoverable confirmTier = iota
	// tierIrreversible actions destroy state (delete DAC, terminate node);
	// they require an explicit capital Y behind a DESTRUCTIVE warning.
	tierIrreversible
)

// confirmOverlay holds the state of the destructive-action confirmation
// modal. run is the deferred command; it is invoked only on confirm and
// re-resolves its target at that time so a background reload cannot leave
// it acting on a stale row. returnView restores the prior view on dismiss.
type confirmOverlay struct {
	tier       confirmTier
	action     string
	kind       string
	target     string
	warning    string
	returnView common.ViewMode
	run        func() tea.Cmd
}
```

In `internal/ui/tui/model_state.go`, add the field to the `Model` struct (next to the other view state, after `lastViewMode common.ViewMode` at line 115):

```go
	viewMode      common.ViewMode
	lastViewMode  common.ViewMode // for toggling help view
	confirm       confirmOverlay  // destructive-action confirmation modal state
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/ui/tui/ -run TestConfirmView_String`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/ui/tui/common/view_mode.go internal/ui/tui/confirm.go internal/ui/tui/model_state.go internal/ui/tui/confirm_test.go
git commit -m "feat(tui): add ConfirmView mode and confirmOverlay state"
```

---

### Task 2: requestConfirm / dismissConfirm helpers

**Files:**
- Modify: `internal/ui/tui/confirm.go`
- Test: `internal/ui/tui/confirm_test.go`

**Interfaces:**
- Consumes: `confirmOverlay`, `common.ConfirmView` (Task 1).
- Produces: `func (m *Model) requestConfirm(c confirmOverlay) tea.Cmd` (stores `c`, captures `returnView`, switches to `ConfirmView`, returns `nil`); `func (m *Model) dismissConfirm()` (restores `returnView`, clears `m.confirm`).

- [ ] **Step 1: Write the failing test**

Append to `internal/ui/tui/confirm_test.go`:

```go
func TestRequestConfirm_OpensModalAndCapturesReturnView(t *testing.T) {
	t.Parallel()
	m := newConfirmTestModel(t)
	m.viewMode = common.ListView

	cmd := m.requestConfirm(confirmOverlay{
		tier:   tierRecoverable,
		action: "Drain",
		kind:   "node",
		target: "gpu-1",
		run:    func() tea.Cmd { return nil },
	})

	assert.Nil(t, cmd, "opening the modal issues no command")
	assert.Equal(t, common.ConfirmView, m.viewMode)
	assert.Equal(t, common.ListView, m.confirm.returnView)
	assert.Equal(t, "Drain", m.confirm.action)
}

func TestDismissConfirm_RestoresViewAndClears(t *testing.T) {
	t.Parallel()
	m := newConfirmTestModel(t)
	m.viewMode = common.ListView
	m.requestConfirm(confirmOverlay{tier: tierRecoverable, action: "Drain", run: func() tea.Cmd { return nil }})

	m.dismissConfirm()

	assert.Equal(t, common.ListView, m.viewMode)
	assert.Equal(t, confirmOverlay{}, m.confirm, "overlay must be cleared")
}
```

Add this test helper to `internal/ui/tui/confirm_test.go` (imports `models` and `logging`):

```go
func newConfirmTestModel(t *testing.T) *Model {
	t.Helper()
	m, err := NewModel(
		WithRepoPath("repo"),
		WithEnvironment(models.Environment{Type: "dev", Region: "us-phx-1", Realm: "oc1"}),
		WithLoader(fakeLoader{}),
		WithLogger(logging.NewNoOpLogger()),
	)
	require.NoError(t, err)
	return m
}
```

Update the import block of `confirm_test.go` to add:

```go
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/pkg/infra/logging"
	"github.com/jingle2008/toolkit/pkg/models"
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ui/tui/ -run 'TestRequestConfirm|TestDismissConfirm'`
Expected: FAIL — build error, `requestConfirm`/`dismissConfirm` undefined.

- [ ] **Step 3: Implement the helpers**

Append to `internal/ui/tui/confirm.go`:

```go
// requestConfirm opens the confirmation modal for a destructive action,
// capturing the current view so dismissConfirm can restore it. It returns
// nil: opening the modal issues no command.
func (m *Model) requestConfirm(c confirmOverlay) tea.Cmd {
	c.returnView = m.viewMode
	m.confirm = c
	m.viewMode = common.ConfirmView
	return nil
}

// dismissConfirm closes the modal, restoring the prior view and clearing
// the pending overlay.
func (m *Model) dismissConfirm() {
	m.viewMode = m.confirm.returnView
	m.confirm = confirmOverlay{}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/ui/tui/ -run 'TestRequestConfirm|TestDismissConfirm'`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/ui/tui/confirm.go internal/ui/tui/confirm_test.go
git commit -m "feat(tui): add requestConfirm/dismissConfirm modal helpers"
```

---

### Task 3: updateConfirmView key handling + routing

**Files:**
- Modify: `internal/ui/tui/confirm.go`
- Modify: `internal/ui/tui/model_update.go:134-149` (add `ConfirmView` case to `delegateToActiveView`)
- Test: `internal/ui/tui/confirm_test.go`

**Interfaces:**
- Consumes: `confirmOverlay`, `dismissConfirm`, `keys.Quit` (`ctrl+c`).
- Produces: `func (m *Model) updateConfirmView(msg tea.Msg) (tea.Model, tea.Cmd)`.

Key semantics: recoverable — `y`/`Y` confirm, `n`/`esc` cancel, other keys swallowed. Irreversible — `Y` confirms, `y`/`n`/`esc` cancel, other keys swallowed. `ctrl+c` always quits. Non-key messages are ignored (modal stays).

- [ ] **Step 1: Write the failing test**

Append to `internal/ui/tui/confirm_test.go`:

```go
// armConfirm puts the model into ConfirmView with a run thunk that records
// whether it fired, returning a pointer to that flag.
func armConfirm(m *Model, tier confirmTier) *bool {
	ran := false
	m.confirm = confirmOverlay{
		tier:       tier,
		action:     "Delete",
		kind:       "DAC",
		target:     "dac-1",
		returnView: common.ListView,
		run:        func() tea.Cmd { ran = true; return nil },
	}
	m.viewMode = common.ConfirmView
	return &ran
}

func keyMsg(s string) tea.KeyMsg {
	if s == "esc" {
		return tea.KeyMsg{Type: tea.KeyEsc}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func TestUpdateConfirmView_Recoverable(t *testing.T) {
	t.Parallel()
	t.Run("y confirms and dismisses", func(t *testing.T) {
		t.Parallel()
		m := newConfirmTestModel(t)
		ran := armConfirm(m, tierRecoverable)
		_, _ = m.updateConfirmView(keyMsg("y"))
		assert.True(t, *ran, "y must run the action")
		assert.Equal(t, common.ListView, m.viewMode)
	})
	t.Run("n cancels without running", func(t *testing.T) {
		t.Parallel()
		m := newConfirmTestModel(t)
		ran := armConfirm(m, tierRecoverable)
		_, _ = m.updateConfirmView(keyMsg("n"))
		assert.False(t, *ran, "n must not run the action")
		assert.Equal(t, common.ListView, m.viewMode)
	})
	t.Run("esc cancels", func(t *testing.T) {
		t.Parallel()
		m := newConfirmTestModel(t)
		ran := armConfirm(m, tierRecoverable)
		_, _ = m.updateConfirmView(keyMsg("esc"))
		assert.False(t, *ran)
		assert.Equal(t, common.ListView, m.viewMode)
	})
	t.Run("unrelated key is swallowed", func(t *testing.T) {
		t.Parallel()
		m := newConfirmTestModel(t)
		ran := armConfirm(m, tierRecoverable)
		_, _ = m.updateConfirmView(keyMsg("x"))
		assert.False(t, *ran)
		assert.Equal(t, common.ConfirmView, m.viewMode, "stays in modal")
	})
}

func TestUpdateConfirmView_Irreversible(t *testing.T) {
	t.Parallel()
	t.Run("capital Y confirms", func(t *testing.T) {
		t.Parallel()
		m := newConfirmTestModel(t)
		ran := armConfirm(m, tierIrreversible)
		_, _ = m.updateConfirmView(keyMsg("Y"))
		assert.True(t, *ran)
		assert.Equal(t, common.ListView, m.viewMode)
	})
	t.Run("lowercase y cancels (does not run)", func(t *testing.T) {
		t.Parallel()
		m := newConfirmTestModel(t)
		ran := armConfirm(m, tierIrreversible)
		_, _ = m.updateConfirmView(keyMsg("y"))
		assert.False(t, *ran, "lowercase y must not run an irreversible action")
		assert.Equal(t, common.ListView, m.viewMode, "lowercase y cancels the modal")
	})
}

func TestUpdateConfirmView_CtrlCQuits(t *testing.T) {
	t.Parallel()
	m := newConfirmTestModel(t)
	armConfirm(m, tierIrreversible)
	_, cmd := m.updateConfirmView(tea.KeyMsg{Type: tea.KeyCtrlC})
	require.NotNil(t, cmd, "ctrl+c must return a command (tea.Quit)")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ui/tui/ -run TestUpdateConfirmView`
Expected: FAIL — build error, `updateConfirmView` undefined.

- [ ] **Step 3: Implement updateConfirmView and wire routing**

Append to `internal/ui/tui/confirm.go` (add imports `"github.com/charmbracelet/bubbles/key"` and `keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"`):

```go
// updateConfirmView resolves a keypress while the confirmation modal is
// open. Recoverable actions confirm on y/Y; irreversible actions require an
// explicit capital Y. n/esc cancel; for irreversible, a lowercase y also
// cancels (so muscle-memory never destroys state). Any other key is
// swallowed so the modal stays put. ctrl+c always quits.
func (m *Model) updateConfirmView(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	if key.Matches(km, keys.Quit) {
		m.cancelInFlight()
		return m, tea.Quit
	}

	s := km.String()
	confirmed := s == "Y" || (m.confirm.tier == tierRecoverable && s == "y")
	if confirmed {
		run := m.confirm.run
		m.dismissConfirm()
		return m, run()
	}

	cancelled := s == "n" || s == "N" || s == "esc" ||
		(m.confirm.tier == tierIrreversible && s == "y")
	if cancelled {
		m.dismissConfirm()
	}
	return m, nil
}
```

In `internal/ui/tui/model_update.go`, add a case to `delegateToActiveView` after the `LogView` case (line 147-148):

```go
	case common.LogView:
		return m.updateLogView(msg)
	case common.ConfirmView:
		return m.updateConfirmView(msg)
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/ui/tui/ -run TestUpdateConfirmView`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/ui/tui/confirm.go internal/ui/tui/model_update.go internal/ui/tui/confirm_test.go
git commit -m "feat(tui): handle confirm-modal keys and route ConfirmView"
```

---

### Task 4: Per-action confirm builders + gate handleItemActions

**Files:**
- Modify: `internal/ui/tui/confirm.go` (builders)
- Modify: `internal/ui/tui/reducer_actions.go:44-67` (`handleItemActions` destructive cases)
- Test: `internal/ui/tui/confirm_test.go`

**Interfaces:**
- Consumes: `confirmOverlay`, `requestConfirm`, `itemKeyString`, `findItem`, action methods `deleteItem(itemKey)`, `cordonNode(item, itemKey)`, `drainNode(item, itemKey)`, `rebootNode(item, itemKey)`, `scaleUpGPUPool(item, itemKey)`.
- Produces: builder methods on `*Model` returning a `confirmOverlay`:
  - `confirmDelete(itemKey models.ItemKey) confirmOverlay`
  - `confirmCordon(itemKey models.ItemKey) confirmOverlay`
  - `confirmDrain(itemKey models.ItemKey) confirmOverlay`
  - `confirmReboot(itemKey models.ItemKey) confirmOverlay`
  - `confirmScale(itemKey models.ItemKey) confirmOverlay`

- [ ] **Step 1: Write the failing test**

Append to `internal/ui/tui/confirm_test.go` (add import `"github.com/jingle2008/toolkit/internal/domain"`):

```go
func TestConfirmDelete_TierByCategory(t *testing.T) {
	t.Parallel()
	key := models.ItemKey{Name: "dac-1"}

	m := newConfirmTestModel(t)
	m.category = domain.DedicatedAICluster
	dac := m.confirmDelete(key)
	assert.Equal(t, tierIrreversible, dac.tier)
	assert.Equal(t, "Delete", dac.action)
	assert.Equal(t, "DAC", dac.kind)
	assert.Equal(t, itemKeyString(key), dac.target)
	assert.NotEmpty(t, dac.warning)
	assert.NotNil(t, dac.run)

	m.category = domain.GPUNode
	node := m.confirmDelete(key)
	assert.Equal(t, tierIrreversible, node.tier)
	assert.Equal(t, "Terminate", node.action)
	assert.Equal(t, "node", node.kind)
}

func TestConfirmRecoverableBuilders(t *testing.T) {
	t.Parallel()
	key := models.ItemKey{Name: "gpu-1"}
	m := newConfirmTestModel(t)

	for _, tc := range []struct {
		name   string
		got    confirmOverlay
		action string
		kind   string
	}{
		{"cordon", m.confirmCordon(key), "Toggle cordon", "node"},
		{"drain", m.confirmDrain(key), "Drain", "node"},
		{"reboot", m.confirmReboot(key), "Reboot", "node"},
		{"scale", m.confirmScale(key), "Scale up", "GPU pool"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tierRecoverable, tc.got.tier)
			assert.Equal(t, tc.action, tc.got.action)
			assert.Equal(t, tc.kind, tc.got.kind)
			assert.Empty(t, tc.got.warning, "recoverable actions carry no warning line")
			assert.NotNil(t, tc.got.run)
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ui/tui/ -run 'TestConfirmDelete|TestConfirmRecoverable'`
Expected: FAIL — build error, builder methods undefined.

- [ ] **Step 3: Implement builders and gate the dispatch**

Append to `internal/ui/tui/confirm.go` (add import `"github.com/jingle2008/toolkit/internal/domain"` and `"github.com/jingle2008/toolkit/pkg/models"`):

```go
// confirmDelete builds the irreversible overlay for the Delete key. The
// action label and warning depend on the category: a DAC is deleted, a GPU
// node's backing instance is terminated. run re-resolves nothing extra —
// deleteItem already re-finds its target by key at execution time.
func (m *Model) confirmDelete(itemKey models.ItemKey) confirmOverlay {
	c := confirmOverlay{
		tier:   tierIrreversible,
		target: itemKeyString(itemKey),
		run:    func() tea.Cmd { return m.deleteItem(itemKey) },
	}
	switch m.category {
	case domain.GPUNode:
		c.action, c.kind = "Terminate", "node"
		c.warning = "Boot volume destroyed. Cannot undo."
	default: // DedicatedAICluster
		c.action, c.kind = "Delete", "DAC"
		c.warning = "This is irreversible."
	}
	return c
}

// confirmCordon builds the recoverable overlay for the cordon toggle. The
// run thunk re-resolves the item by key at confirm time so a background
// reload cannot leave it acting on a stale row.
func (m *Model) confirmCordon(itemKey models.ItemKey) confirmOverlay {
	return confirmOverlay{
		tier:   tierRecoverable,
		action: "Toggle cordon",
		kind:   "node",
		target: itemKeyString(itemKey),
		run:    func() tea.Cmd { return m.cordonNode(findItem(m.dataset, m.category, itemKey), itemKey) },
	}
}

func (m *Model) confirmDrain(itemKey models.ItemKey) confirmOverlay {
	return confirmOverlay{
		tier:   tierRecoverable,
		action: "Drain",
		kind:   "node",
		target: itemKeyString(itemKey),
		run:    func() tea.Cmd { return m.drainNode(findItem(m.dataset, m.category, itemKey), itemKey) },
	}
}

func (m *Model) confirmReboot(itemKey models.ItemKey) confirmOverlay {
	return confirmOverlay{
		tier:   tierRecoverable,
		action: "Reboot",
		kind:   "node",
		target: itemKeyString(itemKey),
		run:    func() tea.Cmd { return m.rebootNode(findItem(m.dataset, m.category, itemKey), itemKey) },
	}
}

func (m *Model) confirmScale(itemKey models.ItemKey) confirmOverlay {
	return confirmOverlay{
		tier:   tierRecoverable,
		action: "Scale up",
		kind:   "GPU pool",
		target: itemKeyString(itemKey),
		run:    func() tea.Cmd { return m.scaleUpGPUPool(findItem(m.dataset, m.category, itemKey), itemKey) },
	}
}
```

In `internal/ui/tui/reducer_actions.go`, change the destructive cases in `handleItemActions` (lines 57-65) to defer through `requestConfirm`:

```go
	case key.Matches(msg, keys.ToggleCordon):
		return m.requestConfirm(m.confirmCordon(itemKey))
	case key.Matches(msg, keys.DrainNode):
		return m.requestConfirm(m.confirmDrain(itemKey))
	case key.Matches(msg, keys.Delete):
		return m.requestConfirm(m.confirmDelete(itemKey))
	case key.Matches(msg, keys.RebootNode):
		return m.requestConfirm(m.confirmReboot(itemKey))
	case key.Matches(msg, keys.ScaleUp):
		return m.requestConfirm(m.confirmScale(itemKey))
```

(The non-destructive cases — `CopyTenant`, `EditTenant`, `OpenMetrics`, `Refresh` — are unchanged. The local `item` variable is no longer read by these cases; if the compiler reports `item` unused, drop its assignment and keep `itemKey := itemKeyFrom(m.category, m.selectedRawRow())`.)

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/ui/tui/ -run 'TestConfirmDelete|TestConfirmRecoverable'`
Expected: PASS

- [ ] **Step 5: Run the full package to confirm the dispatch still compiles and passes**

Run: `go test ./internal/ui/tui/`
Expected: PASS (`ok`)

- [ ] **Step 6: Commit**

```bash
git add internal/ui/tui/confirm.go internal/ui/tui/reducer_actions.go internal/ui/tui/confirm_test.go
git commit -m "feat(tui): gate destructive hotkeys behind confirmation"
```

---

### Task 5: Render the confirmation modal

**Files:**
- Modify: `internal/ui/tui/confirm.go` (`confirmView()`)
- Modify: `internal/ui/tui/model_view.go:186-190` (add `ConfirmView` case to `renderActiveView`)
- Test: `internal/ui/tui/confirm_test.go`

**Interfaces:**
- Consumes: `m.confirm`, `m.centered` (existing).
- Produces: `func (m *Model) confirmView() string`.

- [ ] **Step 1: Write the failing test**

Append to `internal/ui/tui/confirm_test.go` (add import `"strings"`):

```go
func TestConfirmView_RecoverableRender(t *testing.T) {
	t.Parallel()
	m := newConfirmTestModel(t)
	m.confirm = confirmOverlay{tier: tierRecoverable, action: "Drain", kind: "node", target: "gpu-1"}
	out := m.confirmView()
	assert.Contains(t, out, "Drain")
	assert.Contains(t, out, "gpu-1")
	assert.Contains(t, out, "[y]")
	assert.NotContains(t, out, "DESTRUCTIVE")
}

func TestConfirmView_IrreversibleRender(t *testing.T) {
	t.Parallel()
	m := newConfirmTestModel(t)
	m.confirm = confirmOverlay{
		tier: tierIrreversible, action: "Terminate", kind: "node",
		target: "gpu-1", warning: "Boot volume destroyed. Cannot undo.",
	}
	out := m.confirmView()
	assert.Contains(t, out, "DESTRUCTIVE")
	assert.Contains(t, out, "Terminate")
	assert.Contains(t, out, "gpu-1")
	assert.Contains(t, out, "Boot volume destroyed. Cannot undo.")
	assert.Contains(t, out, "Press Y")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ui/tui/ -run TestConfirmView_.*Render`
Expected: FAIL — build error, `confirmView` undefined.

- [ ] **Step 3: Implement confirmView and wire the render dispatch**

Append to `internal/ui/tui/confirm.go` (add imports `"fmt"` and `"strings"`):

```go
// confirmView renders the confirmation modal body. The irreversible tier
// leads with a DESTRUCTIVE banner, shows the warning line, and asks for a
// capital Y; the recoverable tier is a plain y/N prompt.
func (m *Model) confirmView() string {
	c := m.confirm
	var b strings.Builder
	if c.tier == tierIrreversible {
		b.WriteString("⚠ DESTRUCTIVE\n\n")
		fmt.Fprintf(&b, "%s %s  %s\n", c.action, c.kind, c.target)
		if c.warning != "" {
			fmt.Fprintf(&b, "%s\n", c.warning)
		}
		b.WriteString("\nPress Y to confirm   n/esc cancel")
		return b.String()
	}
	fmt.Fprintf(&b, "Confirm %s\n\n", strings.ToLower(c.action))
	fmt.Fprintf(&b, "%s %s  %s ?\n", c.action, c.kind, c.target)
	b.WriteString("\n[y] confirm   [n/esc] cancel")
	return b.String()
}
```

In `internal/ui/tui/model_view.go`, add a case to `renderActiveView` after the `LogView` case (line 188-189):

```go
	case common.LogView:
		return m.logView()
	case common.ConfirmView:
		return m.centered(m.confirmView())
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/ui/tui/ -run TestConfirmView_.*Render`
Expected: PASS

- [ ] **Step 5: Full verification**

Run: `go test ./internal/ui/tui/ && gofumpt -l internal/ui/tui/ && golangci-lint run ./internal/ui/tui/`
Expected: tests `ok`, `gofumpt` prints nothing, `golangci-lint` reports `0 issues`.

- [ ] **Step 6: Commit**

```bash
git add internal/ui/tui/confirm.go internal/ui/tui/model_view.go internal/ui/tui/confirm_test.go
git commit -m "feat(tui): render the destructive-action confirmation modal"
```

---

## Self-Review

**Spec coverage:**
- Action tiers (irreversible delete/terminate; recoverable cordon/drain/reboot/scale) → Task 4 builders. ✓
- ViewMode modal pattern → Task 1 (`ConfirmView`) + Task 3 routing + Task 5 render. ✓
- `confirmOverlay` state with `run` re-resolving by key → Task 1 struct + Task 4 builders (recoverable thunks call `findItem`; `deleteItem` already re-resolves). ✓
- Gating flow via `requestConfirm` → Task 2 + Task 4. ✓
- Key semantics (y/N recoverable; capital-Y irreversible; lowercase-y cancels irreversible; ctrl+c quits; other swallowed; non-key ignored) → Task 3 tests + impl. ✓
- Rendering both tiers with DESTRUCTIVE warning → Task 5. ✓
- Non-destructive actions never gated → Task 4 leaves those cases unchanged (noted). ✓
- Testing list from spec → covered across Tasks 1-5; the "each destructive key opens ConfirmView and does not run" requirement is covered structurally (handleItemActions returns `requestConfirm(...)` which opens the modal and returns nil) and by the builder tests.

**Placeholder scan:** No TBD/TODO; all steps contain runnable code and exact commands.

**Type consistency:** `confirmOverlay`, `confirmTier`, `tierRecoverable`/`tierIrreversible`, `requestConfirm`, `dismissConfirm`, `updateConfirmView`, `confirmView`, and the five builder names are used identically across tasks. Builder signatures take `models.ItemKey` and return `confirmOverlay` consistently.

**Note for the implementer:** the cordon confirmation uses the label "Toggle cordon" (not "Cordon"/"Uncordon") because the action is a toggle and its direction depends on live node state not available at build time — this is intentional and matches the spec's toggle handling.
