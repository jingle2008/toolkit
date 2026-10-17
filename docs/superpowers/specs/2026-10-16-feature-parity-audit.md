# Feature Parity Audit: TUI vs CLI vs MCP

**Date:** 2026-10-16
**Scope:** `internal/ui/tui`, `internal/cli` (+ `cmd/toolkit`), `internal/mcp`
**Baseline:** `main` @ `7b5b636` (refreshed from `dc9e488`; PRs #140–#145 re-checked)

## Summary

All three surfaces share the important layers: categories from `internal/domain`, loading through `loader.Composite`, columns from `internal/columns`, filtering through `internal/collections` / `internal/relations`, and mutation execution through `internal/infra/k8s` and `internal/ui/tui/actions`. Recent work kept new features in step on all three surfaces: #140 added orphaned-InferenceService delete everywhere, and #141 renamed reboot to restart everywhere.

What remains:

1. **Single-surface features:** dangling rows, metrics links, dry-run, drain status, get-by-name.
2. **Per-surface policy:** confirmation, tiering and audit are each implemented separately. MCP has drifted the furthest.
3. **TUI-local logic** standing in for a shared helper (F1).
4. **A shared-writer race:** the metadata file has a lost-update bug that #145's `tenant sync` makes much more likely to bite (F9).

## Changes since `dc9e488`

| PR | Effect on parity |
|---|---|
| #140 delete orphaned InferenceServices | New action on all 3 surfaces: TUI Ctrl+X, CLI `delete inference-service` (`-y` required), MCP `delete_inference_service`. MCP gates it with plain `confirm=true`, which widens F2. |
| #141 reboot → restart | TUI Shift+R, CLI `restart node`, and MCP `restart_node` are consistent. The TUI also stopped carrying state across env switches. |
| #142/#143 perf + relation fixes | No change in parity. TUI `g` now looks targets up by name. |
| #144 one tenant per OCID | Tenant shape is now `{name, id, …}` everywhere. |
| #145 tenant status + `tenant sync` | Tenant `status` shows on all surfaces. A DELETING/TERMINATED tenant is now `IsFaulty`, so TUI Ctrl+Z on Tenant picks it up. `tenant sync` is CLI-only by design, but its writes race the TUI/MCP (F9). |

## Status (2026-10-17)

| Finding | Status |
|---|---|
| F1 TUI compartment lookup | Fixed in #146 |
| F2 MCP destructive gating | Fixed: annotations #146; `confirm_target` and `dry_run` #148; `--mcp-read-only` #149 |
| F3 mutation policy in three places | Fixed in #148 (`internal/mutation`; the TUI now writes audit records) |
| F4 row-level dangling only in the TUI | Fixed in #150 (`get --faulty`, MCP `faulty`, `check_integrity`) |
| F5 no get-by-name | Fixed by the get-by-name PR (`get <category> <name>`, MCP `get_item`) |
| F6 wrong flag name in MCP docs | Fixed in #146 |
| F7 metrics plan in the TUI | Fixed by the parity-backlog PR: planning moved to `telemetry.PlanFor`; `toolkit metrics` and MCP `metrics_url` |
| F8 minor gaps and layering | Fixed: explicit cordon (#148); `ToggleCordon` removed (#149); OCI operations moved to `internal/infra/oci` and output to `internal/output`, `drain --status`, one alias shape, the ServingRuntime TUI column set documented (parity-backlog PR). CLI/MCP sort: left out by decision (pipe to `sort`/jq) |
| F9 metadata lost updates | Fixed in #147 |

A re-check of what those PRs added on more than one surface found new drift, now closed too:

| Drift | Status |
|---|---|
| CLI `--dry-run` didn't look the target up; MCP `dry_run` did | Fixed in #152 (shared `mutation.Resolved`) |
| Refusals audited on MCP only; lookup failures audited differently per surface | Fixed in #152 |
| TUI Ctrl+Z flagged every GPU pool when OCI was unreachable | Fixed in #152 |
| `get_item` returned every match; `get <name>` at most one | Fixed in #151 (shared `collections.PickByName`) |
| `list_related` had no `faulty`; `check_integrity` lacked doctor's hints and pool note | Fixed by the integrity-parity PR (shared `Check.Hint`, `NotePartialPools`, `FaultyCategoryErr`) |

The matrices below are the audit as found, before these fixes.

## Read / query matrix

| Capability | TUI | CLI | MCP |
|---|---|---|---|
| All 21 data categories | ✅ | ✅ `get <cat>` | ✅ 17 tools (defs/overrides via `kind`) |
| Alias listing | ✅ Ctrl+A | ✅ `get alias` | ⚠️ `list_aliases`, one row per alias rather than per category |
| Fuzzy filter | ✅ `/` | ✅ `-f` | ✅ `filter` |
| Parent → child (related-to) | ✅ Enter drill-in | ✅ `--related-to cat/name` | ✅ `list_related` |
| Child → parent | ✅ `o` / `g` | ❌ | ❌ |
| Row-level dangling/faulty | ✅ Ctrl+Z (8 categories) | ⚠️ `doctor --relations` summary only | ❌ |
| Integrity report | ❌ | ✅ `doctor --relations` | ❌ |
| Tenant status | ✅ column + Shift+S sort | ✅ column | ✅ `status` field |
| Sort | ✅ (TUI-local comparators) | ❌ | ❌ |
| Column selection | n/a | ✅ `--columns` | ❌ full struct |
| Limit | n/a | ✅ `--limit` | ✅ `limit` (no offset/cursor) |
| Single item by name | ✅ detail view (JSON) | ❌ | ❌ |
| Export | ✅ CSV via `cli/output.WriteDelimited` | ✅ table/json/jsonl/yaml/csv/tsv | JSON |
| Tenant owner resolution | ✅ | ✅ | ✅ |
| GPU pool live status | ⚠️ local compartment lookup (F1) | ✅ `resolve.EnrichGPUPools` | ✅ `resolve.EnrichGPUPools` |
| Metrics dashboard URL | ✅ `m` (TUI-local plan resolution) | ❌ | ❌ |
| Env switching | ✅ Enter / quick-switch 0–5 | env flags | per-call `env_*` overrides |

## Mutation matrix

| Action | TUI | CLI | MCP |
|---|---|---|---|
| Cordon / uncordon | ⚠️ toggle only (Shift+C) | ✅ `cordon` / `uncordon` | ✅ `cordon_node` / `uncordon_node` |
| Drain | ✅ | ✅ blocking, resumable | ✅ async + background driver |
| Drain status | ❌ | ❌ | ✅ `get_drain_status` |
| Restart node (soft reset) | ✅ Shift+R | ✅ `restart node` (+`--ocid`) | ✅ `restart_node` (+`ocid`) |
| Terminate node | ✅ irreversible tier | ✅ `-y` required | ⚠️ `confirm=true` only |
| Restart inference service | ✅ Shift+R | ✅ `restart inference-service` | ✅ |
| Delete orphaned inference service (+ force-delete) | ✅ Ctrl+X, irreversible tier | ✅ `-y` required | ⚠️ `confirm=true` only |
| Scale pool to Terraform size | ✅ | ✅ | ✅ |
| Delete DAC | ✅ irreversible tier | ✅ `-y` required | ⚠️ `confirm=true` only |
| Set tenant metadata | ⚠️ unresolved DAC/ImportedModel rows only | ✅ `set tenant` | ✅ `set_tenant` |
| Bulk tenant sync | ❌ (by design) | ✅ `tenant sync [--write]` | ❌ (by design) |
| Dry-run | ❌ | ✅ `-n` (and `tenant sync` defaults to dry) | ❌ |
| Tiered confirmation | ✅ capital-Y for irreversible | ✅ `RequireExplicitYes` | ❌ one flat `confirm` |
| Structured audit (`msg=mutation`, phase) | ❌ ad-hoc `action started` lines | ✅ `surface=cli` | ✅ `surface=mcp` |
| Read-only mode | n/a | n/a | ❌ mutation tools always registered |
| MCP tool annotations | n/a | n/a | ❌ none |

## Findings

### F1 — Bug: the TUI's GPU pool enrichment bypasses the shared compartment lookup (still open)

**Where:** `internal/ui/tui/model_reducer.go:26-80`, in `knownCompartmentID`, `lookupCompartmentID` and `updateGPUPoolState`. #141 reshaped this code to work on a snapshot so it is safe across env switches, but it still re-implements `resolve.CompartmentID`.

**What goes wrong:** with zero GPU nodes it returns `("", nil)` and calls `actions.PopulateGPUPools` with an empty compartment. The shared helper returns `no GPU nodes in cluster (cannot resolve compartment ID)` and caches the result per kube context.

**Fix:** keep the `known` shortcut, and fall back to `resolve.CompartmentID`. `resolve` does not import `tui`, so there is no cycle.

### F2 — MCP destructive tools have no extra gate (widened by #140)

`terminate_node`, `delete_dac` and the new `delete_inference_service` all need the same `confirm=true` as `cordon_node`. For the same actions:

- the CLI requires `-y`
- the TUI accepts only capital Y

MCP also lacks:

- dry-run
- a read-only mode
- tool annotations (`ReadOnlyHint` / `DestructiveHint`). The go-sdk v1.8.0 supports them, but `internal/mcp` sets none.

### F3 — Mutation policy is implemented three times

| Surface | Implementation |
|---|---|
| CLI | `runMutation` / `mutationPlan`, `internal/cli/mutate.go:176` |
| MCP | `runMutationToolWith`, `internal/mcp/mutations.go:118` |
| TUI | `confirmTier`, `internal/ui/tui/confirm.go` |

The action/kind vocabulary is consistent but nothing enforces it:

- actions: `cordon`, `uncordon`, `drain`, `restart`, `terminate`, `scale`, `delete`, `force-delete`, `set`
- kinds: `node`, `gpu_pool`, `dac`, `inference_service`, `tenant`

`force-delete` comes from `k8s.OrphanDeleteOutcome.Action()`. The TUI still emits no `msg=mutation` audit records (no `"surface"` field anywhere in `internal/ui/tui`). So a terminate or force-delete started from the TUI never shows up when you search the logs for mutations.

### F4 — Row-level dangling is TUI-only (still open)

Ctrl+Z (`faultyGateFor`, `internal/ui/tui/table_utils.go:41`) combines `IsFaulty()` with `relations.DanglingRows`. #145 extended `IsFaulty` to Tenant (DELETING/TERMINATED). CLI and MCP have no row-level equivalent.

**Caveat:** `DanglingRows` only checks relations whose parent is loaded, so CLI/MCP must load the parents first. `loadIntegrityDataset` (`internal/cli/doctor_relations.go:70`) already does this, and should become shared.

This now matters more than before, because MCP's `delete_inference_service` targets *dangling* InferenceServices. An agent has no MCP tool to find them first.

### F5 — No get-by-name in the CLI or MCP (still open)

### F6 — Docs bug: wrong flag name in the MCP footer (still open)

`internal/mcp/mutations.go:453,464` say `--mutation_env_override_allowed`. The real flag is `--mutation-env-override-allowed` (`internal/cli/flags.go:31`).

### F7 — Metrics plan resolution lives in the TUI (still open)

**Where:** `internal/ui/tui/reducer_actions.go:196-490`: `openMetrics`, `resolveMetricsPlan` and `capabilityPrecedence`.

### F8 — Minor gaps and layering

- The TUI can only toggle cordon.
- The TUI's set-tenant only works on unresolved rows.
- Drain status exists only in MCP.
- There is no sort in the CLI or MCP.
- The ServingRuntime column set is TUI-only (`columns.ServingRuntimeTUIColumns`) and undocumented.
- `list_aliases` and `get alias -o json` return different shapes.
- Layering:
  - `cli`, `mcp` and `resolve` import `internal/ui/tui/actions`.
  - The TUI imports `internal/cli/output`.

### F9 — NEW, Bug: metadata writes lose updates across processes

**Where:** `production.Client.UpsertTenantMetadata` (`internal/infra/loader/production/production.go:211`). It writes back the copy of the metadata loaded at startup, plus one entry, and never re-reads the file.

**What goes wrong:** a TUI or MCP server running for hours overwrites, on its next set-tenant:

- everything `toolkit tenant sync --write` recorded (statuses, filled-in names)
- every entry another process added with `set tenant` / `set_tenant`

**Status:** the `tenant sync` help text documents the hazard ("Quit any running toolkit TUI or MCP server before --write"), but the code doesn't prevent it. MCP servers are long-lived and easy to forget.

`tenant sync` (`internal/tenantsync/cli.go:79`) has the same race in the other direction. It reads the file at the start of a run that can take minutes, so a `set tenant` written during the run is lost when sync saves.

**Fix:** read the file again at write time, under an advisory lock. Writes are already atomic (`SaveMetadata` uses `fileutil.WriteFileAtomic`); what's missing is that re-read. If the file is unreadable, keep refusing to write (`metadataLoadErr`). The fix applies to all surfaces at once.

## Method

- **Initial pass:** three agents, one per surface, inventoried capabilities.
- **Refresh:** done by diffing `dc9e488..7b5b636` across `internal/{cli,mcp,ui/tui,resolve,relations,infra}` and re-checking every finding against the new code:
  - F1: read `model_reducer.go`.
  - F2: grep for `Annotations` in `internal/mcp`.
  - F3: grep for `"surface"` in `internal/ui/tui`, and collect the action/kind literals from CLI and MCP.
  - F4: grep for `DanglingRows` / `CheckIntegrity` users.
  - F6: grep for the flag name.
  - F9: read `UpsertTenantMetadata`.
- Matrix cells not listed above come from the agents' reads plus the PR diffs; they were not each re-verified.
