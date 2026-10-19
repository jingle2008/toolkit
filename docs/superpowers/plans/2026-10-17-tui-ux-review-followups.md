# TUI UX review follow-ups

The 2026-10-17 UX review of every screen (after the v2 redesign, PRs
#158–#170) ranked its findings into three batches. Each batch ships as its
own PR from fresh main.

## Batch 1 — safety and feedback

- [x] Production is unmistakable: a red `PROD` badge in the env bar and a
  `PRODUCTION` badge on the confirm modal's context line
  (`Environment.IsProduction`: type `prod`; shep_targets declares dev,
  preprod and prod).
- [x] Recoverable confirmations say what they do: cordon/uncordon, drain
  (its steps, per `k8s.DrainNode`), node restart (soft reset) and
  scale-up (current → configured size, per `oci.IncreasePoolSize`).
- [x] Error toasts stay until `Esc` in the list or details view, wrap to
  three lines, and point at the log.
- [x] An empty table says why: loading, filter, faulty-only, scope, or no
  rows in the context.

Dropped from the review: "Enter on an irreversible Delete button does
nothing". It already redraws the hint as "press capital Y to delete"
(`confirmOverlay.nudge`); the snapshot only showed the modal before a key.

## Batch 2 — table columns and key notation

- [x] Content-aware column widths (`columnWidths`): a column needs its
  header or its widest value; spare width goes by ratio. Short of room,
  long values shrink to 12, Status never shrinks, and columns drop from
  the right (sparing the first and Status), counted in the panel title.
- [x] Only the active sort column carries an arrow; the `↕` markers go.
- [x] Empty cells as a muted `—`, matching the preview.
- [ ] ~~Booleans as `✓` / blank~~ — dropped: with room, a column is as
  wide as its header, so `✓` saves no width; short of room, `true`/`false`
  already keep their width while long columns give way. And blank would
  hide `Ready false`, the value that matters.
- [x] One key notation everywhere: the registry's help labels are the
  footer's (`C`, `^x`, `/`, `:`), so `footerKey` is gone; help gets a Sort
  section and sentence case; the edit-tenant and export hints match.
- [x] Footer keeps `/` and `:` visible: category actions drop first.

## Batch 3 — export, details, preview, goldens

- [x] Export shows the folder the file goes to and an editable name
  (`tab`); no permission/size columns; `▸` cursor; height fits content.
  `enter` exports into the folder shown (the filepicker's Select is off,
  `l`/`→` open), fixing "can't export into the current or an empty
  folder"; success toasts the path.
- [x] Details view: fields layout (`renderFields` over the item's JSON,
  so it matches `o`), JSON on `tab`; breadcrumb shows layout and scroll
  position instead of list stats.
- [x] Preview shares the layout at depth 1 (nested objects open one
  level) with humanized labels.
- [x] Goldens trimmed to 80×24 and 120×40; only the preview keeps
  200×50 (`wideGoldenViews`).

## Batch 4 — log view

- [x] The log renders in the frame (env bar, breadcrumb with state,
  filter chips and `[shown/total]`, titled panel, key footer).
- [x] `/` text filter and `l` minimum-level cycle; `esc` clears the
  filters before closing.

Dropped from the review: "the help logo costs space on short terminals".
Help already draws it only when it fits without making help scroll.

## Batch 5 — the rest of the review

- [x] Palette: recent items first, group named once, short aliases, open
  category marked.
- [x] Row picker: badges left-aligned; "type to filter" hint. (It already
  showed `a–b of n` when scrolling.)
- [x] Edit tenant: placeholders, inline "required", enter moves fields
  (saves on the last), ctrl+s saves anywhere.
- [x] Confirm: irreversible hint says lowercase y cancels.
- [x] Loading: q quit hint. Load errors: r retry where Refresh is bound.
- [x] Panel title is the row count; details breadcrumb drops the row.
- [ ] ~~Type the name to confirm deletes in prod~~ — dropped by request.

## Batch 6 — leftovers

- [x] Help filter on `/` (key or description; a matching section title
  keeps the section whole).
- [x] A modal within 4 rows of the screen's height draws on a blank
  screen, not over view fragments.
- [x] Confirm Kind row names the category.
- [x] Edit tenant: `^o` explained on its own line (DevOps portal).
- [x] Next-step hints for errors other than loads: retry keys for
  drain (resumes), restart, delete, tenant save, export, details render;
  safety-check refusals (not an orphan, deferred orphan delete via
  `k8s.ErrDeleteDeferred`, no metrics dashboard) are warnings; a failed
  DAC delete or node termination now toasts at all.
- [ ] Browser-launch failures (portal, metrics) — a hint needs the URL
  to be copyable first.
- [x] Colors and contrast: text roles ≥4.5:1 on six common backgrounds
  and fills ≥4.5:1 under their text (`TestPalette_Contrast`); dark fills
  split from dark text (`*Fill` roles); `NO_COLOR` draws focus in reverse
  video (`tea.ColorProfileMsg`). Verified in tmux: 256-color downsampling
  and NO_COLOR stripping both work; true 16-color untested.

## Batch 7 — one count

- [x] The cursor position moves to the panel's bottom edge (`1 of 3`),
  with the total when a filter or faulty-only mode hides rows; the panel
  title stops repeating the count; details and log positions move there
  too. Closes the last of "the same information appears several times".

## Batch 8 — visual review and palette redesign

Rendered every screen (live data and the golden model, dark in the user's
WezTerm colors, light) on a cell grid and reviewed the images.

- [x] Selected row: quiet Selection tint with no foreground and a `▌`
  Accent marker in its left padding; statuses keep their colors on it.
  Fixes light-mode cells turning black after a status, and statuses at
  1.0–3.2:1 on the pink fill.
- [x] One meaning per hue: Primary = keys, Accent = focus, status trio,
  DangerFill = production/irreversible only. Removed Secondary, Focus,
  Info and the Accent/Secondary/Success/Info fills.
- [x] Chrome: category, stats, LIVE and log state are text; PROD is the
  one fill.
- [x] Export picker themed; JSON view follows light/dark and drops
  glamour's top margin; help never ends a page on a title.

## Batch 9 — 2026-10-18 review (navigation, safety, narrow terminals)

A heuristic review (Nielsen; k9s/lazygit conventions) over the goldens and
a live tmux session on preprod.

- [x] `esc` in a drilled-in list returns to the parent with the cursor on
  the row drilled in from (as the manual already said), stepping back in
  history so `]` re-enters the child. It used to drop the scope and stay
  in the child, listing every row.
- [x] `Enter` on a scope drills into the first child with rows in what's
  loaded (a tenant without limit overrides no longer opens an empty list).
- [x] Irreversible confirm: lowercase `y` nudges ("press capital Y…")
  instead of cancelling — a "yes" key no longer means no. Reverses
  Batch 5's hint.
- [x] Footer: blanked under modals (the modal has its own hint line);
  an empty list shows only list-level actions.
- [x] Narrow terminals: help clips to the screen (it overflowed at 50
  columns); a resize re-truncates OCIDs in the middle (they were
  end-cut to identical `ocid1.tenanc…`); the panel title names hidden
  columns and points at `y`.
- [ ] ~~Status column shrinks on narrow terminals~~ — kept: Batch 2 chose
  never to cut Status (`WARN: CORDONED` → `WARN: C…` loses the meaning).
