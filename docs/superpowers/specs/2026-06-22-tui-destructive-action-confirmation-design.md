# TUI Destructive-Action Confirmation — Design

**Date:** 2026-06-22
**Status:** Approved (pending implementation)
**Finding:** Review finding #1 — TUI destructive actions bypass confirmation/dry-run controls.

## Problem

CLI and MCP mutations are gated: the CLI requires `--yes` (`RequireExplicitYes`) or an
interactive `y/N` prompt, and MCP refuses unless `confirm=true`. The TUI has **no such
gate** — `handleItemActions` dispatches destructive operations directly from a single
hotkey. An accidental or stale-selection keypress immediately cordons, drains, reboots,
terminates a node, deletes a DAC, or scales a pool against the live environment.

This design adds a confirmation step to the TUI's destructive hotkeys, tiered by the
blast radius of each action.

## Scope

In scope: confirmation for the five destructive TUI hotkeys below. Out of scope: any
change to CLI/MCP gating (already correct), and a dry-run mode for the TUI (the
interactive confirmation is the TUI analog of `--dry-run`/`--yes`).

## Action tiers

| Hotkey | Action | Target | Tier |
|--------|--------|--------|------|
| `ctrl+x` | delete DAC | `DedicatedAICluster` | **Irreversible** |
| `ctrl+x` | terminate instance (boot volume destroyed) | `GPUNode` | **Irreversible** |
| `C` | toggle cordon / uncordon | `GPUNode` | Recoverable |
| `D` | drain node | `GPUNode` | Recoverable |
| `R` | reboot (soft reset) | `GPUNode` | Recoverable |
| `U` | scale up GPU pool | `GPUPool` | Recoverable |

- The `Delete` (`ctrl+x`) key is **always** irreversible — both of its targets destroy
  state.
- The cordon toggle is gated in **both** directions; the modal label reflects the
  resulting action ("Cordon" vs "Uncordon"). Uncordon is restorative but gated for a
  consistent, predictable mental model.
- Scale-up is recoverable (the pool can be scaled back down).

## Approach

Use the codebase's established modal pattern (a `common.ViewMode` with a dedicated
update handler and render function, routed by `delegateToActiveView`) — exactly how
`EditTenantView` and `LogView` already work. Rejected alternatives: an `InputMode`
status-bar prompt (too cramped to carry the irreversible warning; overloads edit-input
semantics) and a generic reusable confirm sub-component (no existing precedent; YAGNI
for five call sites).

## Components

### 1. ViewMode

Add `common.ConfirmView` to `internal/ui/tui/common/view_mode.go` (enum value +
`String()` case).

### 2. Confirm overlay state

A `confirmOverlay` struct on the `Model` (grouped like the existing `logOverlay`):

```go
type confirmTier int

const (
    tierRecoverable confirmTier = iota
    tierIrreversible
)

type confirmOverlay struct {
    tier       confirmTier
    action     string         // verb for the label, e.g. "Delete", "Drain", "Terminate"
    kind       string         // e.g. "DAC", "node", "GPU pool"
    target     string         // resource name
    warning    string         // extra line for the irreversible tier
    returnView common.ViewMode
    run        func() tea.Cmd // the deferred destructive command
}
```

Stored as `m.confirm confirmOverlay`. The `run` thunk **re-resolves the item from its
`itemKey` at confirm time** (mirroring how `deleteDedicatedAICluster` already calls
`findItem(m.dataset, …)`), so a background watch reload while the modal is open cannot
act on a stale row pointer.

### 3. Gating flow

`handleItemActions` no longer runs destructive methods directly. Each destructive case
calls a helper:

```go
func (m *Model) requestConfirm(c confirmOverlay) tea.Cmd {
    c.returnView = m.viewMode
    m.confirm = c
    m.viewMode = common.ConfirmView
    return nil
}
```

Non-destructive actions (copy tenant, open metrics, refresh, edit-tenant) are unchanged.

### 4. Key routing (`updateConfirmView`)

`delegateToActiveView` gains `case common.ConfirmView: return m.updateConfirmView(msg)`.
The modal owns all key input while open:

- **Recoverable tier:** `y` or `Y` → confirm; `n` or `esc` → cancel; any other key →
  swallowed (stays in the modal).
- **Irreversible tier:** `Y` (uppercase only) → confirm; `y` (lowercase), `n`, `esc` →
  cancel; any other key → swallowed.
- `ctrl+c` always quits (quitting runs nothing — safe).
- Non-key messages (e.g. `WindowSizeMsg`) fall through to normal handling so resize
  still works.

Confirm: dismiss the modal (`m.viewMode = m.confirm.returnView`, clear `m.confirm`) and
return `run()`. Cancel: dismiss and return `nil`. The existing optimistic UI feedback
(e.g. DAC status → "Deleting") runs only after a confirm.

### 5. Rendering (`confirmView()`)

A centered bordered overlay in the same style as `editTenantView`/`logView`:

- **Recoverable:**
  - title: `Confirm <action>`
  - body: `<Action> <kind>  <target> ?`
  - footer: `[y] confirm   [n/esc] cancel`
- **Irreversible:**
  - title: `⚠ DESTRUCTIVE`
  - body: `<Action> <kind>  <target>`
  - warning line: e.g. `Boot volume destroyed. Cannot undo.` (terminate) /
    `This is irreversible.` (delete DAC)
  - footer: `Press Y to confirm   n/esc cancel`

## Data flow

1. User presses a destructive hotkey in `ListView`.
2. `handleItemActions` builds a `confirmOverlay` (capturing `itemKey`/category, tier,
   labels, and a `run` thunk) and calls `requestConfirm` → `viewMode = ConfirmView`.
3. `confirmView()` renders the modal over the list.
4. Next keypress routes to `updateConfirmView`:
   - confirm → restore `returnView`, return `run()` (which re-resolves the item and
     issues the existing action command);
   - cancel → restore `returnView`, return `nil`.

## Error handling

- The confirmation layer adds no new error paths; it defers existing commands unchanged.
  Action failures continue to surface through the current `*ResultMsg`/`deleteErrMsg`
  reducers.
- If the item can no longer be resolved at confirm time (e.g. it was removed by a
  background reload), `run` follows the existing "item not found" path already present in
  each action method (logs and returns nil) — no panic, no stale action.

## Testing (TDD)

Model-level tests, no real OCI/k8s — assertions are on `viewMode` transitions and
whether a command is returned (a non-nil `tea.Cmd` from confirm vs `nil` from cancel):

1. Each destructive hotkey opens `ConfirmView` with the correct tier/target, **and**
   `handleItemActions` returns no command (the action is deferred, not run).
2. Recoverable tier: `y` → non-nil run cmd + `viewMode` restored; `n`/`esc` → nil cmd +
   overlay cleared; an unrelated key → still `ConfirmView`.
3. Irreversible tier: `Y` → runs; `y` (lowercase) → cancels (does **not** run); `n`/`esc`
   → cancels.
4. `confirmView()` output contains the action and target, and the DESTRUCTIVE warning for
   the irreversible tier.
5. `ctrl+c` during the modal still quits.
6. Non-destructive actions (copy, metrics, refresh, edit-tenant) are never gated.

## Out of scope / future

- A per-session "don't ask again" bypass (no demand; TUI is interactive — always-confirm
  is acceptable). 
- Updating the `HelpView` keybinding hints to note that destructive keys prompt — a
  nice-to-have, not required for this change.
