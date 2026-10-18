# TUI v2 Migration and Redesign — Design

**Date:** 2026-10-17
**Status:** Approved (pending implementation)
**Baseline:** `main` @ `68122b7`
**Plan:** `docs/superpowers/plans/2026-10-17-tui-v2-redesign.md`

## Problem

The TUI looks and feels inconsistent, and three specific defects motivated this work:

1. **Help gets truncated.** `fullHelpView` renders one column at `viewWidth/2`, centered by
   `CenterText`, with no height bound — on a short terminal the bottom sections are lost.
2. **Confirm and row-picker modals look unfinished.** Both are bare text centered on an
   empty screen: no border, no color, no emphasis. The irreversible tier is a
   `⚠ DESTRUCTIVE` text line. The row picker also has no height bound, so a long target
   list truncates like help.
3. **Styling is ad hoc.** `styles.go` holds a partial `Styles` struct; toasts, the log
   view and the table hard-code their own xterm indexes (160, 214, 33, 24, 130, 240…).

Beyond the defects, navigation has three problems:

- **Discoverability** — keys are learned only from the help screen.
- **Key overloading** — one shift+letter means different things per category: `R` is
  Restart on GPU nodes / inference services but *Sort Regions* on overrides; `C` Cordon vs
  *Sort Context*; `U` Scale Up vs *Sort Usage*; `T`, `V`, `S` each sort two different
  columns. Sorts share the key space with mutating actions.
- **Location** — scope, filter, sort, faulty mode and history position are not visible at
  a glance.

## Scope

In scope: a full migration to the Charm v2 stack, then a redesign of every view
(list, details, loading, help, export, edit-tenant, log, confirm, row picker, toasts),
a new keymap built on a prefix engine, a command palette, and a detail-preview split.

Out of scope: mouse support (capture breaks native text selection, which users rely on to
copy IDs — revisit behind a flag later); any CLI/MCP change; v2 keyboard enhancements
(key release events etc.).

## Decisions

| # | Decision |
|---|---|
| Stack | Full v2: `charm.land/bubbletea/v2` v2.1.0, `charm.land/bubbles/v2` v2.2.1, `charm.land/lipgloss/v2` v2.0.6, `charm.land/glamour/v2` v2.0.1. |
| Order | Migration first and behavior-preserving; redesign on top. |
| Safety net | ANSI-stripped golden snapshots of every view, added on v1 before the swap; the swap must leave them byte-identical. |
| Swap parity | v2 defaults that change UX are disabled in the swap: viewport `h`/`l` horizontal scroll unbound (`h` is Help), textinput cursor matches v1, no keyboard enhancements requested. |
| Light/dark | `lipgloss.LightDark(isDark)` fed by `tea.BackgroundColorMsg` from the swap on (`compat.AdaptiveColor` queries the terminal at package init, which every CLI command would pay for). The theme phase builds all roles this way. glamour stays on `DarkStyle` (the auto style costs 5–10 s at startup). |
| Palette | Keep the existing identity (orange `#FF8700`, pink `#FF5F87`, purple `#A550DF`) as named roles. |
| Layout | Single pane with compact chrome; optional right-hand preview pane. |
| Modals | Composited over the dimmed underlying view with lipgloss v2 layers. |
| Keys | Core keys fixed; sorts move under an `s` prefix; every shift+letter means one action everywhere. |
| Mouse | None. |

## Visual System

### Theme roles

`Palette` (in `theme.go`) holds the roles, and `Styles` is built from it. Every view takes colors from roles, never literals.

| Role | Dark | Light | Use |
|---|---|---|---|
| `Primary` | `#FF8700` | `#D75F00` | info labels, section headers (keys from Phase 5) |
| `Accent` | `#FF5F87` | `#D7005F` | context chip, spinner, logo bottom, selected row (Phase 7) |
| `Secondary` | `#A550DF` | `#8700AF` | counters, stats chip |
| `Text` | `#E4E4E4` | `#262626` | body |
| `OnAccent` | `#FFFDF5` | `#FFFDF5` | text on Accent/Secondary/Success/Info/Danger fills |
| `Muted` | `#8A8A8A` | `#6C6C6C` | hints, debug lines, dim backdrop |
| `Border` | `#585858` | `#BCBCBC` | unfocused borders, header rule |
| `Focus` | `#5F5FD7` | `#5F5FAF` | modal and details borders |
| `Selection` / `OnSelection` | `#5F00FF` / `#FFFFAF` | same | selected row (until Phase 7) |
| `Surface` / `OnSurface` | `#353533` / `#C1C6B2` | `#D9DCCF` / `#343433` | bars, modal background |
| `Success` | `#2EA043` | `#1A7F37` | LIVE, ready |
| `Warn` / `OnWarn` | `#FFAF00` / `#000000` | `#9A6700` / `#FFFDF5` | recoverable confirm, pending/draining, paused log, log warnings |
| `Danger` | `#D70000` | `#CF222E` | irreversible confirm, faulty, errors |
| `Info` | `#0087FF` | `#0969DA` | info toasts, following log, help keys |

Phase 4 picked these values to keep the current look. The roles and their uses are fixed; the values can be tuned later.

### Frame (list view)

```
 prod › OC1 › us-ashburn-1 · ctx gpu-prod-iad · k8s v1.31           ⠋ 3s  ● LIVE  v1.42.0
 DACs › dac-foo   ⟨/gpu⟩  ⟨sort: age ▼⟩  ⟨faulty⟩  ⟨◀ 2/5⟩                         12/124
╭─ DACs (124) ─────────────────────────────────────────────────────────────────────────╮
│ Name ▲            Tenant          Size   Usage   Status     Age                      │
│ ──────────────────────────────────────────────────────────────────────────────────── │
│ dac-a             acme            4      87%     Active     12d                      │
│▌dac-b             globex          2      12%     Failed     3d                       │
│ dac-c             initech         8      54%     Active     41d                      │
╰──────────────────────────────────────────────────────────────────────────────────────╯
 o go-to  m metrics  E edit tenant  ^x delete  s sort  : palette  / filter  ? more
```

- **Env bar** (line 1): realm › type › region · context · k8s; spinner+timer, LIVE, version
  right-aligned. Replaces the 6-line info block. Truncates from the middle segments first.
- **Breadcrumb** (line 2): category › scope, then chips that appear only when active
  (filter, sort, faulty, history position), row counter right-aligned. Replaces the status
  bar; the filter/command text input renders in place of the chips while typing.
- **Table**: rounded border, title in the top edge, header underline, `▲`/`▼` on the
  sorted column, full-width accent selected row, no zebra. Status, faulty and usage cells
  get semantic colors; faulty rows get a `Danger` gutter marker (`▌`).
- **Footer**: 6–8 context hints for the category/view (from the keymap), then `? more`.
  Truncates from the right; `? more` always survives.
- **Logo**: removed from the frame; shown on the loading screen and in the help overlay.

Chrome is 3 rows plus the table border (was ~8).

### Toasts

A bordered box in the bottom-right corner, composited as a layer (no layout shift, no
clipping of the table border). Border and title color by severity (`Info`/`Warn`/`Danger`).
Up to 3 stack upward; existing 8 s lifetime unchanged.

### Modals

All modals render as a layer over the current frame, which is redrawn in `Muted` (dimmed).
Shared chrome: rounded border, title in the top edge, a key-hint line at the bottom, max
width `min(80, 90% viewWidth)`, max height `90% viewHeight` with scrolling inside.

**Confirm — irreversible:**

```
╭─ ⚠  Delete DAC ──────────────────────────────╮   border + title: Danger
│                                              │
│   Kind     DedicatedAICluster                │
│   Name     dac-b                             │
│   Env      prod / us-ashburn-1               │
│                                              │
│   This permanently deletes the cluster.      │   warning: Danger
│                                              │
│          [ Cancel ]   [ Delete ]             │   Cancel highlighted (default)
│                                              │
│   Y confirm · n/esc cancel · ←/→ select      │
╰──────────────────────────────────────────────╯
```

- Recoverable tier: `Warn` border, no ⚠, `y` confirms.
- Buttons are a visual cue *and* selectable (`←`/`→`/`tab`, `enter` activates the
  highlighted one; Cancel is the default). The irreversible tier keeps the capital-`Y`
  requirement: `enter` on `[ Delete ]` is ignored there and the hint pulses
  "press Y". Audit semantics (cancel = refused) unchanged.

**Row picker:**

```
╭─ Go to related ──────────────────────────────╮
│ filter: run█                                 │   shown when > 8 rows
│                                              │
│ ▸ [DAC]       dac-b                          │   category badge: Secondary bg
│   [Runtime]   vllm-0.6                       │
│   [Tenant]    globex                         │
│                                       1–3/3  │   scroll indicator
│ ↑/↓ select · enter go · esc cancel           │
╰──────────────────────────────────────────────╯
```

**Help:**

- Sections flow into columns: `cols = clamp((innerWidth+gap) / (sectionWidth+gap), 1, 3)`;
  sections are packed column-major, balanced by line count.
- If the packed height still exceeds the modal's max height, content goes in a viewport
  (`j`/`k`, `pgup`/`pgdn`, `g`/`G`) with a `↓ more (n%)` indicator.
- Logo in the title row when width allows.

**Export, edit tenant:** shared modal chrome; focused field gets a `Primary` left marker
(replacing `> `), toggles render as `( ) internal  (•) external`.

**Log view:** stays full-screen; title bar recolored from roles (`Info` following,
`Warn` paused).

**Loading view:** centered logo, spinner + "Loading DACs … 3s" under it.

### Preview split

- Toggle with `v` in list view; off by default; the choice persists for the session.
- Available only at ≥160 columns; below that `v` shows a toast ("needs ≥160 columns").
- Right pane, 40% width, bordered, titled with the row name; content is a condensed
  key/value summary of the selected row (top-level scalar fields, nested objects collapsed
  to `{…}`/`[n]`), not glamour JSON. Updates on cursor move (debounced like filtering).
- `y` full-screen details is unchanged.

## Navigation

### Prefix engine

The list view has one prefix key, `s` (sort). A small state machine in the model
(`prefix.go`) handles it:

- On `s` the model records the pending prefix and shows it in the status bar (`s…`).
  The next key completes the sequence (`s n`).
- A non-matching continuation cancels the prefix (and is *not* replayed). `esc` cancels.
- Text-entry modes (filter, palette, edit tenant, row-picker filter) bypass the engine.
- *(`gg` and a which-key popup were considered and dropped: `g`/`G` stay single keys
  for top/bottom.)*

### Keymap

**Fixed (unchanged):** `:` `/` `enter` `esc` `?`/`h` `q`/`ctrl+c` `tab`/`shift+tab`
`[`/`]` `0`–`5` `` ` `` `y` `c` `p` `e` `r` `t` `o` (go to related; copy object in
details) `m` `ctrl+x` `ctrl+o` `ctrl+a` `ctrl+z` `E` `C` `D` `U`.

**Changed:**

| Before | After | Action |
|---|---|---|
| shift+⟨first letter⟩ | `s` ⟨first letter⟩ | Sort by column (all 14 sorts: `N` → `s n`, `T` → `s t`, …) |
| `G` (Sort GPUs) | `s g` | Sort GPUs |
| `end` | `G` / `end` (`g` / `home` unchanged) | Bottom (top) of table and help |
| — | `v` | Toggle preview split |

**Sort keys.** A sort key is the column name's **first letter, unique within a
category**, the same mnemonic as today's shift+letter scheme without the shift. Today
first letters never collide within a category, so every key is the first letter:

| Category | Sortable columns → key |
|---|---|
| DAC | Name, Tenant, Internal, Usage, Size, Age → `n t i u s a` |
| ImportedModel | Name, Tenant, Size, Context, Vendor → `n t s c v` |
| GPUNode | Name, Free, Type, Age → `n f t a` |
| GPUPool | Name, Size, GPUs → `n s g` |
| GPUWorkload | Name, Tenant, GPUs, Age → `n t g a` |
| Tenant | Name, Internal, Status → `n i s` |
| BaseModel | Name, Size, Context → `n s c` |
| Tenancy/regional overrides | Name, Tenant, Regions, Value → `n t r v` (subset per category) |
| Environment, ServiceTenancy | Name, Type → `n t` |

- **Fallback:** if a future column's first letter is taken within its category, it
  gets the next letter of its name that is free there. The key is derived from
  the category's column list, not hand-assigned.
- **Display:** while `s` is pending, the table header underlines each sortable column's
  key letter (<u>N</u>ame, <u>T</u>enant, <u>S</u>ize…). The header becomes the menu.
  Outside the prefix the header is plain.
- **Guard:** a registry test fails if two sortable columns in one category resolve to
  the same key.
- Re-pressing a sort sequence toggles direction, as today.

**Resulting invariant:** shift+letter = act on the row, one meaning in every category
(`R` Restart, `C` Cordon, `D` Drain, `U` Scale Up, `E` Edit Tenant). A registry test
locks it: no shift+letter binding may appear with two different descriptions across
`catContext`.

### Command palette

`:` opens a palette overlay (was a status-bar command input):

- Sections: **Categories** (name + aliases), **Environments** (recent first, then all from
  `m.dataset.Environments`), **Actions** for the current row/category (with their key
  shown), **Toggles** (alias, faulty, preview).
- Fuzzy match via `github.com/sahilm/fuzzy`; match runs highlighted.
- **Compatibility:** if the input exactly equals a category alias, `enter` behaves exactly
  as `:alias⏎` does today, regardless of ranking.
- ≤12 visible rows, scrolling; `↑`/`↓`/`ctrl+n`/`ctrl+p` move, `enter` runs, `esc` closes.

### Keys-changed hint

A UI-state file next to the recent-envs file stores `{"keymapVersion": N}`. When the
stored version is older than the binary's, the first frame shows a dismissible toast-style
overlay: "Sorting moved to `s`+letter, `G` is now bottom — `?` for all keys". Dismissing
writes the current version. Missing/corrupt file = show once, then write.

## Testing

- **Goldens:** `internal/ui/tui/testdata/golden/*.txt`, ANSI-stripped renders of each
  view at 80×24, 120×40 and 200×50 from a fixed fixture dataset; `-update` flag
  regenerates. The swap must not change them; redesign phases update them deliberately and
  the diffs are reviewed as the visual change.
- **Unit:** help column packing and scroll bounds; row picker filter/scroll; prefix engine
  (match, timeout, stale tick, cancel, text-mode bypass); keymap invariant; sort-key derivation (first letter, fallback, per-category uniqueness); palette
  ranking + exact-alias override; preview width gate; keymap-version file.
- **Manual:** `go run ./cmd/toolkit` at 80×24 and ≥160 columns, light and dark terminal, each view.

## Risks

- **Compositing v1-era content strings in v2:** glamour output and pre-rendered table
  cells must survive layer compositing (wide runes, ANSI). Covered by goldens + a
  wide-rune fixture row.
- **Background-color query latency:** `LightDark` needs `BackgroundColorMsg`; render with
  the dark theme until it arrives so the first frame is never blank.
- **Prefix keys vs. existing single keys:** `s` and `g` are free today (`g` freed by
  #154); the keymap invariant test plus `registry_conflict_test.go` guard regressions.
- **Muscle memory:** mitigated by the keys-changed hint and CHANGELOG.
