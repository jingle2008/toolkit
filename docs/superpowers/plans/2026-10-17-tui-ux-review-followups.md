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
