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

- [ ] Content-aware column widths: a column is at least as wide as its
  header and its widest short value; low-priority columns drop on narrow
  terminals instead of every column truncating.
- [ ] Only the active sort column carries an arrow; the `↕` markers go.
- [ ] Booleans as `✓` / blank; empty cells as `—`, matching the preview.
- [ ] One key notation everywhere (the footer's: `C`, `^x`, `/`, `:`):
  help, modal hints, the export picker.
- [ ] Footer keeps `/` and `:` visible before category actions.

## Batch 3 — export and details

- [ ] Export shows the current directory and an editable filename; no
  permission/size columns; hints in the shared style; height fits content.
- [ ] Details view uses the preview's key/value layout with sections,
  JSON behind a toggle; scroll position shown.
- [ ] Preview expands nested objects one level and humanizes labels.
