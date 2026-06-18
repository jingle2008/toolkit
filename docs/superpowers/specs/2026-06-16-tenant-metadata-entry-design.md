# Tenant Metadata Entry (TUI) — Design

**Date:** 2026-06-16
**Status:** Implemented (branch `feat/tui-tenant-metadata-entry`)

## Accepted deviations (recorded at final review)

- **Gating toast is combined.** §1 designed two distinct rejection messages
  ("tenant already resolved" vs "no tenancy id to identify"). The implementation
  emits a single combined message ("tenant already resolved or has no tenancy id")
  because the `tenantEditTarget` helper returns a plain `ok` bool; distinguishing
  would require signature churn (and test ripple) for a marginal wording nuance on
  a rarely-hit error path. Behavior (the gating itself) matches the spec.
- **In-memory apply coverage.** `TestApplyTenantSave_ResolvesBothMapsInMemory` asserts
  a saved entry appends the `Tenant` and re-keys BOTH the DAC and ImportedModel maps
  (raw-suffix key → tenant-name key, `Owner` populated) with no loader/cluster call;
  `TestApplyTenantSave_NilMapsSafe` covers not-yet-loaded maps; and
  `TestHandleTenantSavedMsg_AfterFormDismissed` confirms the apply runs even if the
  form was dismissed first.
- **Lossy rewrite.** Saving through the TUI rewrites the metadata file from the
  parsed `Metadata`/`TenantMetadata` struct. Consequently YAML comments and any
  fields not modeled by `TenantMetadata` (only Name/ID/IsInternal/Note are
  modeled) are NOT preserved across a save. As a data-loss guard, a metadata file
  that exists but fails to parse is NOT overwritten: the save is refused with an
  error (the form stays open and a toast surfaces) rather than clobbering the
  user's hand-authored entries. A missing file is still created on first save.

## Follow-up enhancements

- **Open in console portal (`ctrl+o`).** While the entry form is open, `ctrl+o`
  opens the tenant in the OCI console at
  `https://devops.oci.oraclecorp.com/account/admin/detail/metadata/<tenancy-ocid>?realm=<realm>`
  via the platform browser launcher (`actions.OpenURL`: `open`/`xdg-open`/
  `rundll32`). A control key is used so it never collides with typing into the
  Name/Note fields; launch failures surface as an error toast (intercepted at the
  top of `Update`, so the toast fires even if the form was dismissed first).

## Problem

On the **DedicatedAICluster** (DAC) and **ImportedModel** pages, each row is
grouped by its owning tenant. A tenant is shown one of two ways:

- **Resolved** — the row's tenancy OCID matched a known tenant, so we display the
  friendly **Name** and **internal/external** status (`Owner *Tenant` is non-nil).
- **Unresolved** — no match, so we display the raw `TenantID` suffix and the owner
  is unknown (`Owner == nil`).

There is no way, from the TUI, to attach a friendly name to an unresolved tenant.
The only source of friendly tenant info is the external metadata file, which today
can only be edited by hand outside the app.

## Goal

Let the user select an unresolved DAC/ImportedModel row, key in tenant info
(**Name**, **Internal**, **Note**), persist it to the metadata file (creating the
file if it does not exist), and auto-refresh so the row immediately renders the
friendly name + internal status.

## Background (verified in code)

- Unresolved ⟺ selected item's `Owner == nil` (`pkg/models/dataset.go:48`
  `resolveTenantOwnedMap`).
- The full tenancy OCID is reconstructable per row:
  `item.TenancyOCID(realm)` → `ocid1.tenancy.<realm>..<TenantID>`
  (`pkg/models/dedicated_ai_cluster.go:96`, `pkg/models/imported_model.go:53`).
  `realm = m.environment.Realm`.
- Metadata file: `cfg.MetadataFile` (default `~/.config/toolkit/metadata.yaml`),
  shape `Metadata{ Tenants []TenantMetadata{ID, Name, IsInternal, Note} }`
  (`pkg/models/metadata.go`). `ID` is the full OCID.
- `getTenants` (`internal/configloader/configloader.go:129`) merges discovered
  tenancy IDs with metadata entries by full OCID. A standalone metadata entry
  resolves **only if both `Name` and `IsInternal` are non-nil** — which matches the
  fields we capture.
- `Tenants` load eagerly in `LoadDataset` (`configloader.go:307`); DAC/ImportedModel
  load lazily from k8s and are re-keyed against `d.Tenants`.
- `production.Client` (`internal/infra/loader/production/production.go`) holds both
  `metadataFile` and in-memory `metadata`. Neither `loader.Composite` nor the TUI
  has any write path today.
- Existing TUI input is a single-line text input; popups (export) render as a
  bordered overlay with a dedicated `common.ViewMode` and `esc` dismisses them
  (commit c28f1f1).

## Approach (persistence + refresh seam)

**Optional `TenantMetadataWriter` on the loader.**

1. Add `configloader.SaveMetadata(path string, m *models.Metadata) error`, mirroring
   `LoadMetadata`: choose JSON vs YAML by file extension; create parent dir + file if
   absent.
2. Add an upsert/merge helper (replace the entry with a matching `ID`, else append).
3. Give `production.Client` a method:
   `UpsertTenantMetadata(entry models.TenantMetadata) error` that merges `entry` into
   `l.metadata` **and** persists via `SaveMetadata`, keeping in-memory and on-disk
   state consistent.
4. Expose it through a small **optional** interface, type-asserted on `m.loader` in
   the TUI — **not** added to `loader.Composite`, so the existing fake loaders in
   tests are unaffected.

Rejected: having the TUI own merge + rebuild the loader (TUI would need the file path
it doesn't hold today, and would duplicate `production.New` error handling).

**Caveat — pointer receiver / method set.** `UpsertTenantMetadata` mutates
`l.metadata`, so it must be declared with a **pointer receiver**
(`func (l *Client) UpsertTenantMetadata(...)`). The runtime type assertion
`m.loader.(TenantMetadataWriter)` therefore succeeds only when the dynamic type
behind `m.loader` is `*production.Client` — which holds today, since
`production.New` returns `&Client{...}`. If a future refactor made `New` return a
value (`Client`), the assertion would silently start returning `ok == false` and
every save would no-op into an error toast rather than failing to compile. Guard
this with a compile-time/round-trip test asserting that the value returned by
`production.New(...)` satisfies `TenantMetadataWriter` (e.g.
`var _ TenantMetadataWriter = production.New(ctx, "")` or an explicit type
assertion in a test).

## Components

### 1. Key binding & gating
- New key `E` (`shift+e`, help "Edit Tenant") in `internal/ui/tui/keys/registry.go`,
  added to `catContext` for `DedicatedAICluster` and `ImportedModel` **list view only**.
- On press, fetch the selected typed item via `m.selectedItem()` and branch:
  - `Owner != nil` → toast "tenant already resolved", no form.
  - `TenantID == ""` or `TenantID == "UNKNOWN_TENANCY"` → toast "no tenancy id to
    identify this tenant", no form (orphan group has no real tenancy OCID).
  - else → open the form, seeded with the row's `TenantID` for display context.

### 2. Form overlay
- New `common.ViewMode` `EditTenantView`.
- New file `internal/ui/tui/edit_tenant.go`: form state, `View()`, and key handling,
  rendered as a bordered overlay like `exportView()`.
- Fields, `tab`/`shift+tab` to cycle focus:
  - **Name** — `textinput` (required; empty Name blocks save with an inline hint).
  - **Internal** — bool toggle (`space`/`←`/`→` flips; default `true`, matching the
    discovered-tenant default in `getTenants`).
  - **Note** — `textinput` (optional).
- `enter` → save; `esc` → cancel and return to the list.
- Wire the `EditTenantView` branch into the main update switch and view switch.

### 3. Persist
- On save, build:
  `TenantMetadata{ ID: item.TenancyOCID(realm), Name: &name, IsInternal: &internal,
  Note: <&note or nil when blank> }`.
- Dispatch an async `tea.Cmd` that type-asserts `m.loader` to `TenantMetadataWriter`
  and calls `UpsertTenantMetadata(entry)`. If the loader doesn't implement the
  interface (shouldn't happen in production) or the file path is empty → error toast.
- Errors surface as an error toast; the form stays open so input isn't lost.

### 4. Refresh (write + apply in memory)
- On save success, return to `ListView`, show a success toast with the metadata file
  path, and run `applyTenantSave(entry)` — **fully synchronous, no reload, no I/O**.
  The persisted `TenantMetadata` rides along on `tenantSavedMsg` so the reducer has it.
- `applyTenantSave`:
  - Appends a standalone `Tenant{IDs:[entry.ID], Name, IsInternal, Note}` to
    `ds.Tenants` (`upsertTenantByID` — replace-by-ID, else append).
  - Re-keys **both** `DedicatedAIClusterMap` and `ImportedModelMap` (whichever are
    loaded) in memory via `SetDedicatedAIClusterMap` / `SetImportedModelMap`,
    reconstructing the raw suffix-keyed map from each item's retained `TenantID`
    (`rawByTenantID`). This re-resolves `Owner` against the updated `Tenants`.
  - Calls `refreshDisplay`.
- **Why this is correct without re-running `getTenants` or reloading:** the form only
  edits **unresolved** rows (`Owner == nil`). An unresolved tenancy matched no existing
  `Tenant`, which (since any tenant with terraform overrides would already have a
  `Tenant` built from the discovered `tenantMap`) also means it has **no tenancy-
  override entries**. So the edit is always a *standalone* tenant (getTenants' second
  pass), built directly from the entry — no `tenantMap` needed — and the three override
  maps are provably unaffected, so they are left untouched.
- Re-keying both category maps (not just the current one) means a tenant owning
  resources in both DAC and ImportedModel resolves everywhere at once — no sibling-map
  staleness.
- **Constraint:** this holds only while the form edits unresolved rows. If it is ever
  extended to edit *resolved* tenants, a rename could affect override-map display and a
  fuller rebuild (group reload) would be required. Documented at `applyTenantSave`.
- History: earlier revisions (a) re-fetched the category from the cluster, then (b)
  reloaded the tenancy group locally + re-keyed via an async `tenantRekeyMsg`. Both are
  superseded by this synchronous in-memory apply.

### 5. Display — ImportedModel Internal column
- Add an `Internal` column to `internal/columns/imported_model.go`, mirroring DAC:
  placed after `Tenant` (order Name, Tenant, Internal, …).
- Add `OwnerState() string` to `pkg/models/imported_model.go`, mirroring
  `DedicatedAICluster.OwnerState()` (`""` when `Owner == nil`, else
  `fmt.Sprint(Owner.IsInternal)`).
- Rebalance ImportedModel column ratios to keep the sum at 1.00. Target:
  Name 0.20, Tenant 0.20, **Internal 0.08**, Namespace 0.13, Display Name 0.23,
  Vendor 0.10, Status 0.06 (= 1.00). Exact values finalized during implementation;
  the invariant is sum == 1.00 (asserted by existing column tests).
- Add the `SortInternal` key to ImportedModel's `catContext` entry for parity with
  DAC (it now has a sortable Internal column).
- DAC already has its `Internal` column (`OwnerState()`), so no DAC column change.

## Data flow

```
[E on unresolved row]
  -> selectedItem() (Owner==nil, TenantID!="")
  -> EditTenantView form (Name / Internal / Note)
  -> enter
  -> TenantMetadata{ID: TenancyOCID(realm), Name, IsInternal, Note}
  -> loader.UpsertTenantMetadata  (merge in-memory + SaveMetadata to file)
  -> tenantSavedMsg{path, entry}
  -> applyTenantSave: append Tenant + re-key DAC & ImportedModel maps in memory
  -> row now resolved: friendly Name + Internal (no reload / no I/O)
```

## Error handling
- Empty Name → inline form hint, save blocked.
- Loader not a writer / empty metadata path / write failure → error toast; form stays open.
- Post-save apply is pure in-memory (no I/O), so it has no failure mode of its own; if
  the dataset is unexpectedly nil it is a no-op.

## Testing
- `configloader.SaveMetadata`: round-trip JSON and YAML; create-if-missing (file +
  parent dir); extension dispatch; unsupported extension error.
- Upsert/merge helper: replace existing `ID`, append new, preserve others.
- TUI gating: table test over (Owner set / TenantID empty / UNKNOWN_TENANCY / happy
  path) asserting form-open vs toast.
- OCID construction from a row for both DAC and ImportedModel.
- Form: focus cycling, Internal toggle, empty-Name block, esc cancel.
- Save path with a fake `TenantMetadataWriter`: asserts the dispatched
  `TenantMetadata` (ID/Name/IsInternal/Note); `applyTenantSave` resolves both maps in
  memory; the saved-after-dismiss case still applies.
- ImportedModel columns: ratio sum == 1.00; `OwnerState()` for nil/internal/external.
- `production.New(...)` satisfies `TenantMetadataWriter` (guards the pointer-receiver
  caveat above — see Approach).

## Out of scope (YAGNI)
- Editing already-resolved tenants.
- Adding/editing tenant metadata from the Tenant page or other categories.
- Deleting metadata entries from the TUI.
- CLI changes (CSV export Tenant column stays the full tenancy OCID).
```
