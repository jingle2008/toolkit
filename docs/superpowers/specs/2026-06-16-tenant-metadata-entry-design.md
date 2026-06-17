# Tenant Metadata Entry (TUI) — Design

**Date:** 2026-06-16
**Status:** Approved (pending spec review)

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

### 4. Refresh (write + auto-refresh)
- On save success, reset the realm-scoped tenant fields (`Tenants` + the three
  tenancy-override maps) and the **current category's** map (`DedicatedAIClusterMap`
  or `ImportedModelMap`) to `nil`.
- Dispatch `tea.Sequence(loadTenancyOverrideGroupCmd(...), <current-category load cmd>)`
  so `d.Tenants` is rebuilt from the updated metadata **before** the DAC/ImportedModel
  map is re-fetched from k8s and re-keyed — ensuring the edited row resolves.
- Return to `ListView` and show a success toast including the metadata file path.

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
  -> reset Tenants + tenancy maps + current category map
  -> Sequence(load tenancy group -> load current category)
  -> row now resolved: friendly Name + Internal
```

## Error handling
- Empty Name → inline form hint, save blocked.
- Loader not a writer / empty metadata path / write failure → error toast; form stays open.
- Refresh load failure → existing load-error path (error toast); the file was still written.

## Testing
- `configloader.SaveMetadata`: round-trip JSON and YAML; create-if-missing (file +
  parent dir); extension dispatch; unsupported extension error.
- Upsert/merge helper: replace existing `ID`, append new, preserve others.
- TUI gating: table test over (Owner set / TenantID empty / UNKNOWN_TENANCY / happy
  path) asserting form-open vs toast.
- OCID construction from a row for both DAC and ImportedModel.
- Form: focus cycling, Internal toggle, empty-Name block, esc cancel.
- Save path with a fake `TenantMetadataWriter`: asserts the dispatched
  `TenantMetadata` (ID/Name/IsInternal/Note) and that a reload is triggered.
- ImportedModel columns: ratio sum == 1.00; `OwnerState()` for nil/internal/external.

## Out of scope (YAGNI)
- Editing already-resolved tenants.
- Adding/editing tenant metadata from the Tenant page or other categories.
- Deleting metadata entries from the TUI.
- CLI changes (CSV export Tenant column stays the full tenancy OCID).
```
