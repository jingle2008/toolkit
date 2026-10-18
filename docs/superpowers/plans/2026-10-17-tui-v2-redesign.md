# TUI v2 Migration and Redesign Implementation Plan

> **For agentic workers:** Work task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Migrate the TUI to the Charm v2 stack without behavior change, then implement
`docs/superpowers/specs/2026-10-17-tui-v2-redesign-design.md`. Each phase is one PR.

**Baseline:** `main` @ `68122b7`.

**Approach:** Lock the current rendering in goldens, shrink the swap with v1-side prep, swap
atomically, then redesign phase by phase, updating goldens on purpose.

## Global Constraints

- **Lint:** `make ci` before every push. That covers gofumpt, golangci-lint (cyclop ≤ 10),
  and `goimports -local` grouping, where local imports go last.
- **TDD:** write a failing test first, then implement.
- **Repo `CLAUDE.md` rules:**
  - Run `gitnexus_impact` before touching shared helpers. Here that means `frame`,
    `renderActiveView`, `ResolveKeys`, `handleItemActions`, `requestConfirm` and
    `openRowPicker`.
  - Run `gitnexus_detect_changes` before commits that touch 3+ behavioral files.
  - For Phase 3, expect HIGH or CRITICAL labels. They reflect position in the call graph,
    not danger, so report them with that framing.
- **Branching:** don't stack PRs. Start each phase from a fresh `main` once the previous
  phase has merged.
- **Goldens:** after Phase 1, any golden diff in a PR must be intended and called out in
  the PR description.

---

## Phase 1: Golden snapshots (v1)

### Task 1.1: Golden harness
**Files:** Create `internal/ui/tui/golden_test.go` and `internal/ui/tui/testdata/golden/`.
- [x] Add a helper `assertGolden(t, name, s string)` that runs `ansi.Strip`, normalizes
  trailing spaces, and compares the result against `testdata/golden/<name>.txt`. Running
  with `-update` rewrites the file.
- [x] Add a fixture builder that produces a deterministic dataset: one row per category,
  including one faulty row and one row with wide runes. Pin the clock, version and
  k8sVersion. Disable the spinner.

### Task 1.2: Snapshot every view
- [x] Render List (DAC, GPU node, tenant), Details, Loading, Help (from List and from
  Details), Export, EditTenant, Log (following and paused), Confirm (both tiers),
  RowPicker, and a toast.
- [x] Render each at 80×24, 120×40 and 200×50. Also add help at 80×15, which documents the
  current truncation.
- [x] Run `go test ./internal/ui/tui/... -run Golden -update`, then a second time without
  `-update` to confirm the output is stable.

## Phase 2: v1 prep (shrink the swap)

### Task 2.1: Test key helpers
**Files:** `internal/ui/tui/keymsg_test.go` (new) and the 17 test files that build `tea.KeyMsg`.
- [x] Add `keyPress(s string) tea.Msg`, which parses `"y"`, `"esc"`, `"ctrl+c"`,
  `"shift+tab"`, `"space"`, `"up"` and similar. It should be the only place that builds
  `tea.KeyMsg{...}`.
- [x] Replace every `tea.KeyMsg{...}` literal (about 56) and the `keyMsg` helper in
  `confirm_test.go:82` with calls to `keyPress`.
- [x] Check with `grep -rn 'tea.KeyMsg{' internal/` that the only hit is the helper.

### Task 2.2: `.Type` comparisons → `String()`
**Files:** `edit_tenant.go:214-235` and `update_list.go:177,238`.
- [x] Switch on `msg.String()` (`"tab"`, `"shift+tab"`, `"up"`, `"down"`, `"left"`,
  `"right"`, `"ctrl+c"`). For space, accept both `" "` and `"space"` so the same code
  works on v1 and v2.
- [x] `update_list.go:177` currently fakes a backspace key to drive the dirPicker. Replace
  that with a direct call to the dirPicker's back navigation if one is exposed. Otherwise,
  route it through the `keyPress("backspace")` constructor moved into non-test code
  (`keymsg.go`).

### Task 2.3: Logo gradient takes RGB
**Files:** `view/logo.go`, `styles.go:20-21`, and `view/logo_test.go`.
- [x] Change `LogoTop` and `LogoBottom` to the type `struct{R, G, B uint8}`. Interpolate on
  the numbers and build the per-line color as a `#RRGGBB` string at render time. That
  removes the `string(lipgloss.Color)` + `Sscanf` parsing.
- [x] Goldens must stay unchanged.

## Phase 3: v2 swap (behavior-preserving)

### Task 3.1: Dependencies and imports
- [x] Run `go get charm.land/bubbletea/v2@v2.1.0 charm.land/bubbles/v2@v2.2.1
  charm.land/lipgloss/v2@v2.0.6 charm.land/glamour/v2@v2.0.1`.
- [x] Rewrite the imports in the 42 non-test and 38 test files, keeping local imports last.
- [x] Run `go mod tidy`. The v1 modules will still appear because the golangci-lint tools
  pull them in. That is expected.

### Task 3.2: API breaks
- [x] Root `View()` returns `tea.View` and sets `v.AltScreen = true`. Remove
  `tea.WithAltScreen()` from `internal/cli/root.go:222`.
- [x] Change `tea.KeyMsg` type assertions to `tea.KeyPressMsg` (18 sites). Update the
  `keyPress` helper to build `tea.KeyPressMsg{Code, Text, Mod}`.
- [x] viewport: replace `New(20, 20)` with `New(viewport.WithWidth(20),
  viewport.WithHeight(20))` (`model_state.go:305,312`). Replace field assignments with
  `SetWidth`/`SetHeight` (`model_reducer.go:334-335`, `log_view.go:94-95`).
- [x] textinput: `ti.SetWidth(...)` (`model_view.go:110`).
- [x] help: `Width()`/`SetWidth()` (`model_view.go:233,239,240`).
- [x] stopwatch: `stopwatch.New(stopwatch.WithInterval(time.Second))` (`model_state.go:339`).
- [x] filepicker: switch from reading `Height` as a field to the method.
- [x] lipgloss: replace the 2 `AdaptiveColor` uses with `lipgloss.LightDark(isDark)`, with
  `DefaultStyles(isDark)` rebuilt on `tea.BackgroundColorMsg`, which `Init` requests.
  *(Deviation: `compat.AdaptiveColor` was dropped. It queries the terminal at package
  init, which every CLI command would pay for, since `internal/cli` imports the TUI.)*
- [x] lipgloss: `Style.Width` now counts the border. Bordered boxes go through
  `boxWidth(style, w)` to keep their v1 outer width.
- [x] glamour: swap the import path in `view/render.go`. `WithStandardStyle(styles.DarkStyle)`
  stays.

### Task 3.3: Keep v1 behavior
- [x] Unbind the viewport `Left`/`Right` keys (`h`/`l`) in both viewports (details and log).
  Add a test that `h` in DetailsView opens Help.
- [x] Configure the textinput cursor to look the same as v1. Goldens catch any drift.
- [x] Don't set `KeyboardEnhancements`.
- [x] Goldens must stay **byte-identical**. Run `go test ./...` and `make ci`, and run the
  app by hand at 80×24.

## Phase 4: Theme roles

**Files:** `styles.go` → `theme.go`, plus every view that uses color literals
(`toast.go`, `log_view.go`, `model_state.go`, `edit_tenant.go`, `update_export.go`).
- [x] Add `Palette` with the roles from the spec, built by `NewPalette(isDark bool)` using
  `lipgloss.LightDark`. `DefaultStyles(isDark)` builds every style from it, including the
  component styles that `setStyles` pushes into the table, viewport, help and spinner.
- [x] Handle `tea.BackgroundColorMsg` by rebuilding the theme. Request the background color
  in `Init`. Default to dark until the reply arrives. *(Already wired in Phase 3; now it
  restyles the components too.)*
- [x] Replace every `lipgloss.Color("…")` literal outside `theme.go`. Check with
  `grep -rn 'lipgloss.Color(' internal/ui/tui | grep -v theme.go`, which should print nothing.
- [x] Goldens are unchanged (they're stripped of ANSI). Add one ANSI-preserving test
  asserting that the Danger role reaches the error toast.

## Phase 5: Overlay and modal chrome (fixes help and modals)

### Task 5.1: Overlay infrastructure
**Files:** `view/overlay.go` (new).
- [x] Add `Overlay(base string, modal string, w, h int) string`. It re-renders the base in
  Muted (dimmed), centers the modal as a `lipgloss.Layer` with Z=1, and composites with
  `lipgloss.NewCompositor(...).Render()`.
- [x] Add `Corner(base, box string, w, h int)`, which places the box at the bottom right.
  Toasts use it.
- [x] Add `Modal(theme, title, body, hints string, maxW, maxH int)`, the shared chrome:
  rounded border with the title in the top edge.
- [x] Test wide runes and ANSI on both sides of the modal edge.

### Task 5.2: Route modal views through the overlay
- [x] In `renderActiveView`, Confirm, RowPicker, Help, Export and EditTenant render
  `Overlay(m.renderBaseView(), modal)`. `renderBaseView` renders `returnView` (or
  `lastViewMode` for help).
- [x] Toasts move from `JoinVertical` to `Corner`, stacking up to 3. Delete the
  clipped-border comment in `model_view.go`.

### Task 5.3: Confirm redesign
- [x] Danger or Warn chrome by tier, the target as key/value rows, buttons with Cancel as
  the default, `←`/`→`/`tab` to move between buttons and `enter` to activate one.
  Irreversible actions still need `Y`: `enter` on Delete only pulses the hint.
- [x] Existing confirm and audit tests must stay green. Add tests for button focus and for
  "enter on Delete does not run when the action is irreversible".

### Task 5.4: Row picker redesign
- [x] Category badges, a scrolling window bounded by the modal's max height, and a
  `a–b/n` indicator.
- [x] Above 8 rows, add a type-to-filter textinput. The cursor clamps to the filtered set.
  Tests cover filtering, the scroll window following the cursor, and an empty result.

### Task 5.5: Help redesign
- [x] Pack sections into balanced columns column by column (1–3 columns from the width).
- [x] When the packed height exceeds the max, use an inner viewport with a `↓ more`
  indicator and `j`/`k`/`pgup`/`pgdn`/`home`/`end` keys. `g`/`G` come in Phase 6.
- [x] The 80×15 help golden now shows a scroll indicator and no truncation. Add a unit
  test that every binding is reachable by scrolling.

### Task 5.6: Export and edit tenant chrome
- [x] Use the shared `Modal`, a Primary focus marker, and `( )`/`(•)` toggles.
- [x] Update the goldens deliberately and list the views whose goldens changed in the PR.

**Phase 5 deviations:**
- Help may use up to 90% of the terminal width, not the 80-column modal cap, so three columns fit on wide terminals.
- The logo in help's title is deferred to Phase 7, where the header drops it.
- Toasts float one row and two columns inside the bottom-right corner, so they don't cover the table frame. `toasts.active` stays the newest toast, and `older` holds up to two more.
- The export directory list takes its height from the modal body (up to 15 rows), so the export modal fits on 80×24.
- `view.Box` with width 0 widens to fit its title.

## Phase 6: Prefix engine and new keymap

### Task 6.1: Sort prefix
**Files:** `prefix.go` (new), plus `update_list.go` dispatch.
- [x] Model state `prefix` holds the pending prefix (`s`). The next key completes the
  sequence; `esc` or a non-matching continuation cancels, and the key is not replayed.
- [x] Text-entry modes bypass the prefix.
- [x] Tests: sort by key, cancel, esc keeps the filter, bypass, status marker.

### Task 6.2: Keymap migration
**Files:** `keys/registry.go` and `keys/key_map.go`, plus their tests.
- [x] Replace the 14 `Sort*` bindings with one derivation: `SortKeys(cat) map[col]rune`.
  It takes each sortable column's first letter and falls back to the next free letter
  of the column name within that category. The `s ⟨key⟩` sequences are built from it.
  Make `G`/`end` go to the bottom and `g`/`home` go to the top for the table and help.
- [x] Update `SortableColumns` and `FullHelp` to read the derived keys.
- [x] Tests:
  - Every category's derived keys equal first letters today (table-driven against the
    spec table).
  - The fallback picks the next free letter (synthetic clashing columns).
  - No two sortable columns in one category share a key.
  - Across all of `catContext`, a shift+letter key never appears with two different
    descriptions.
  - No sequence prefix collides with a single-key binding.
- [x] Update `registry_conflict_test.go` and `category_drift_test.go`.

### Task 6.3: Pending-prefix display
- [x] Show the pending prefix in the breadcrumb area.
- [x] `s` pending: the table header underlines each sortable column's key letter (the
  derived key's position in the column name). No popup. Test that the underlined index
  matches `SortKeys` in an ANSI-preserving test.

**Phase 6 notes:**
- `gg` and the which-key popup were dropped after review. `gg` began as a question, not a requirement, and the popup only ever had one entry. `g`/`G` are single keys for top/bottom of the table and help. The details view is unchanged.
- The sort prefix lives in the model (`prefix.go`) rather than a generic `keys/sequence.go` matcher, since it is the only prefix. Sort declarations stay `key.Binding`s, which `ResolveKeys` binds to `s ⟨key⟩` per category. That lets help show the sequence and the existing per-category conflict test check sort-key uniqueness.
- `v` (preview toggle) is added with Phase 9, not here.

## Phase 7: Frame

**Files:** `model_view.go` (`frame`, `statusView`, `infoView`), `keys/key_map.go`
(`HeaderHelp` is removed), `log_view.go`, and the loading view.
- [ ] Env bar (one line, truncating from the middle) and a breadcrumb with conditional
  chips and the row counter. The filter and command input render in place of the chips.
- [ ] Footer hints. Add `KeyMap.FooterHints(width)`, which returns the context actions
  first, then the mode keys, and always keeps `? more`. Delete `HeaderHelp` and its fit
  test, replacing them with a footer-fit test.
- [ ] Table chrome: rounded border with the title in the top edge, a header underline,
  sort arrows, the accent selected row, semantic cell colors, and the faulty gutter.
  `updateLayout` budgets 3 chrome rows plus the border.
- [ ] Log view title bar takes its colors from roles. The loading view gets the logo,
  spinner and label.
- [ ] Update the goldens deliberately. Before/after screenshots go in the PR.

## Phase 8: Command palette

**Files:** `palette.go` (new), `update_list.go` (CommandMode entry), `go.mod`
(`github.com/sahilm/fuzzy`).
- [ ] Palette items come from categories with their aliases, recent environments followed
  by everything in `m.dataset.Environments`, the current context actions, and the toggles.
  Each item carries a `run func() tea.Cmd`.
- [ ] Fuzzy ranking with highlighted matches. An exact alias match always wins on `enter`.
  Show at most 12 rows, scrolling.
- [ ] Tests: an exact alias reaches the same `updateCategory` path as today, ranking is
  stable, environment selection calls `switchEnvironment`, and actions dispatch through
  `handleItemActions`.

## Phase 9: Preview split

**Files:** `preview.go` (new), `model_reducer.go` (`updateLayout`), `model_view.go`.
- [ ] `v` toggles `m.preview.on`. Below 160 columns it shows a toast instead. Turning it on
  splits the table area 60/40.
- [ ] A condensed key/value summary of the selected item: top-level scalars, with nested
  values collapsed. Debounce updates on cursor moves using the existing generation
  pattern.
- [ ] Tests: the width gate, the layout widths, and stale summary messages being dropped.

## Phase 10: Keys-changed hint and docs

**Files:** `internal/uistate/` (new, beside `recentenv`), `internal/cli/root.go`,
`USER_MANUAL.md`, `docs/guide/`, `CHANGELOG.md`, and `README.md`.
- [ ] Add `uistate.Load/Save` with `{"keymapVersion": N}`, stored next to the recent-envs
  path and written atomically (`fileutil.WriteFileAtomic`).
- [ ] When the saved version is older than the binary's `KeymapVersion`, show a dismissible
  hint overlay on the first frame. Dismissing it saves the version. A missing or corrupt
  file means show the hint once.
- [ ] Docs: the keymap table, the palette, the preview, and screenshots.

---

## Verification (every phase)

- [ ] `make ci` is green.
- [ ] `go test ./...` passes, and every golden diff is intended and listed in the PR.
- [ ] Run by hand with `go run ./cmd/toolkit` at 80×24 and at ≥160 columns. From Phase 4 on, test in
  both a light and a dark terminal.
- [ ] Run `gitnexus_detect_changes` before committing.
