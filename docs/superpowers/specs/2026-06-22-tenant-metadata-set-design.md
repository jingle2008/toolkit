# Design: `set tenant` — tenant-metadata write parity for CLI & MCP

**Date:** 2026-06-22
**Status:** Approved (design); pending implementation plan

## Problem

A cross-surface parity audit (TUI vs CLI vs MCP) found one genuine
write-capability gap: **upserting tenant metadata is TUI-only.**

`loader.TenantMetadataWriter.UpsertTenantMetadata` is invoked solely from
`internal/ui/tui/edit_tenant.go:276` (the `shift+e` form). There is no CLI
command and no MCP tool to set a tenant's friendly name / internal flag /
note. An automation driving the CLI, or an MCP agent, cannot perform an
edit the interactive TUI can.

Read parity and the other six mutations (cordon, uncordon, drain, reboot,
terminate, scale, delete dac) are already complete across all three
surfaces.

## Goal

Expose tenant-metadata upsert on the CLI and MCP surfaces with exact
behavioral parity to the TUI form — no more, no less.

## Non-goals

- No delete/remove of metadata entries (the TUI doesn't offer it; no
  loader method exists). Upsert-only.
- No new loader code: both surfaces reuse the existing
  `TenantMetadataWriter.UpsertTenantMetadata`.
- No change to the TUI.

## Decisions (locked with user)

| Decision | Choice |
|----------|--------|
| Naming | CLI `toolkit set tenant <ocid>`; MCP tool `set_tenant`. Mirrors the verb-kind pattern (`delete dac` / `delete_dac`, `scale_gpu_pool`). |
| Env scope | **Drop** the `--env-type/region/realm` requirement. Tenant metadata is keyed by the full tenancy OCID and stored in one global metadata file; env is irrelevant. MCP tool omits `envOverride` entirely. |
| Write scope | Upsert only — exact TUI parity. |
| CLI prelude | Approach A: add a `needsEnv bool` parameter to `withMutationSetup` / `validateMutationConfig`. Single setup path; ripple to the 6 existing call sites is mechanical (pass `true`) and compiler-caught. |
| OCID validation | Require the target to start with `ocid1.tenancy.`; fail fast otherwise (an ill-formed OCID would produce an entry that never resolves, since `Metadata.GetTenants` keys off `ID` split by `.`). |

## Data model & shared core

Both surfaces build a `models.TenantMetadata` identical to the TUI's
`editTenantForm.toEntry()` and call the same writer:

```go
entry := models.TenantMetadata{
    ID:         ocid,        // the full tenancy OCID — the entry key
    Name:       &name,       // required; empty name is rejected
    IsInternal: &isInternal, // default true (matches TUI)
}
if note != "" {
    entry.Note = &note       // optional
}
// writer is loader.TenantMetadataWriter (production.Client)
err := writer.UpsertTenantMetadata(entry)
```

`UpsertTenantMetadata` merges by `ID` (replacing any existing entry with the
same ID, else appending) and persists, creating the file if absent. Because
it replaces the whole entry (not field-level merge), `Name` and `IsInternal`
are always set; `Note` is set only when non-empty — matching the TUI.

## CLI — `toolkit set tenant <ocid>`

New file `internal/cli/set_tenant.go`, registered in `root.go` alongside the
other mutation commands.

- **Command tree:** a `set` parent verb (`Short: "Set/update a resource"`)
  with a `tenant <ocid>` subcommand, paralleling `delete` → `dac`.
- **Args:** `<ocid>` — `cobra.ExactArgs(1)`.
- **Flags:**
  - `--name` (string, **required**) — friendly tenant name.
  - `--internal` (bool, default `true`) — `--internal=false` marks external.
  - `--note` (string, optional).
  - `--dry-run, -n` and `--yes, -y` (standard mutation flags).
- **Validation:** name non-empty; OCID has `ocid1.tenancy.` prefix. Both
  produce clear errors before any write.
- **Flow:**
  ```
  withMutationSetup(cfgFile, needsKube=false, needsRepo=false, needsEnv=false, fn)
    └─ runMutation(plan{Action:"set", Kind:"tenant", Target:ocid,
                        RequireExplicitYes:false}, perform)
         └─ perform → setTenantFn(ctx, cfg, entry)
  ```
- **Seam:** `var setTenantFn = func(ctx, cfg, entry) error` — default builds
  `production.New(ctx, cfg.MetadataFile)` and calls `UpsertTenantMetadata`.
  Tests override it (mirrors `deleteDACFn`, `resolveGPUNodeFn`).
- Reuses the existing confirm / dry-run / audit machinery verbatim. Audit
  log records `action=set kind=tenant surface=cli`.

### Shared prelude change (Approach A)

`validateMutationConfig(cfg, needsKube, needsRepo, needsEnv bool)` and
`withMutationSetup(cfgFile, needsKube, needsRepo, needsEnv bool, fn)` gain a
`needsEnv` parameter. When `false`, the env-triple checks are skipped (the
`Environment` is still constructed from cfg, harmlessly empty). The 6
existing call sites (cordon/uncordon share one) pass `needsEnv=true`.
`set tenant` passes `false`.

> Per the repo mandate, run `gitnexus_impact` on `withMutationSetup` and
> `validateMutationConfig` before editing, and report the blast radius.

## MCP — `set_tenant` tool

Added to `internal/mcp/mutations.go`.

- **Input:**
  ```go
  type setTenantInput struct {
      OCID       string  `json:"ocid" jsonschema:"the full tenancy OCID (the metadata entry key)"`
      Name       string  `json:"name" jsonschema:"friendly tenant name (required)"`
      IsInternal *bool   `json:"is_internal,omitempty" jsonschema:"mark tenant internal; defaults to true"`
      Note       string  `json:"note,omitempty" jsonschema:"optional free-form note"`
      confirmGate
      // NO envOverride — global, OCID-keyed file
  }
  ```
- **Handler `handleSetTenant`:** validate name + OCID prefix; type-assert
  `s.loader.(loader.TenantMetadataWriter)` (graceful error if unsupported —
  mirrors the TUI's own guard); default `IsInternal` to `true` when nil; then
  call `runMutationTool(ctx, req, "set", "tenant", ocid, in.Confirm, perform)`
  **directly** (not `handleMutation`, which derives env).
- **Seam:** `var mcpUpsertTenantFn = func(w loader.TenantMetadataWriter, entry) error`
  (or equivalent) for handler tests.
- **Registration:** add as the 8th tool in `registerMutationTools`. Its
  description states `confirm=true` is required but, unlike the other six,
  does **not** mention env overrides — append a trimmed footer rather than
  the shared `mutationToolFooter`.

## Confirmation tier

Recoverable (re-runnable, non-destructive): CLI prompts unless `--yes`; MCP
requires `confirm=true` like every MCP mutation. Not flagged destructive
(no `RequireExplicitYes`, no `DESTRUCTIVE` wording).

## Error handling

- Missing/empty `--name` (CLI) or `name` (MCP) → validation error, no write.
- OCID without `ocid1.tenancy.` prefix → validation error, no write.
- Loader doesn't implement `TenantMetadataWriter` (MCP) → clear error
  ("loader does not support writing metadata"), mirroring the TUI.
- Write failure from `UpsertTenantMetadata` → surfaced verbatim; CLI returns
  it to Cobra, MCP routes through `failTool` with `phase=failed` audit.

## Testing

- **CLI** (`set_tenant_test.go`): table tests via the `setTenantFn` seam —
  dry-run output, abort-on-"n", name-missing error, bad-OCID error, success
  path + audit fields. Mirrors `delete_dac` tests.
- **MCP** (`mutations_test.go` additions): handler tests via the
  `mcpUpsertTenantFn` seam — confirm-refusal, success envelope,
  loader-not-a-writer error, validation errors. Mirrors `handleDeleteDAC`.
- No new loader tests: `UpsertTenantMetadata` is already covered by
  `internal/infra/loader/production/production_test.go`.

## Risk

Low. No new persistence logic; both surfaces delegate to a battle-tested
writer already used by the TUI. The only shared-code change is the additive
`needsEnv` parameter, which is compiler-enforced across its call sites.
