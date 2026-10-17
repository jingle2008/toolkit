# Feature Parity Fixes Implementation Plan

> **For agentic workers:** Work task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the gaps in `docs/superpowers/specs/2026-10-16-feature-parity-audit.md` (F1–F9), most important first. Each phase is one PR.

**Baseline:** `main` @ `7b5b636`.

**Approach:** Put the logic in a shared package first, then wire each surface to it. Surfaces drift apart when each one keeps its own copy, which is the root cause of F2 and F3.

## Global Constraints

- **Lint:** `gofumpt -l` must show no diff and `golangci-lint run` must report zero issues. `cyclop` allows a cyclomatic complexity of at most 10.
- **TDD:** write a failing test first, then implement.
- **Repo `CLAUDE.md` rules:**
  - Run `gitnexus_impact` before touching shared helpers: `runMutation`, `runMutationToolWith`, `DanglingRows`, `UpsertTenantMetadata`.
  - Run `gitnexus_detect_changes` before commits that touch 3+ behavioral files.
  - Package-level `*Fn` seams don't show up in the call graph, so grep for them instead.
- **Branching:** don't stack PRs. Start each phase from a fresh `main`.

## Decisions (defaults marked; confirm before Phase 2–3)

| # | Question | Default |
|---|---|---|
| D1 | MCP gate for irreversible tools (F2): `terminate_node`, `delete_dac`, `delete_inference_service` | Add a required `confirm_target` that must exactly echo the target, on top of `confirm=true`. This is the MCP analogue of the TUI's capital-Y and the CLI's mandatory `-y`. |
| D2 | MCP read-only mode | `--mcp-read-only` flag, **default false** (today's behavior). When set, mutation tools are not registered. |
| D3 | MCP dry-run | Optional `dry_run` that resolves and plans the target but doesn't act. `delete_inference_service` already plans (`PlanOrphanInferenceServiceDelete`), so its dry-run reports delete vs force-delete. |
| D4 | F9 locking | Add a new `configloader.UpdateMetadata(path, func(*models.Metadata) error)` that takes an advisory `flock` on `<metadata>.lock`, re-reads the file, applies the change, and saves through the existing `SaveMetadata` (already atomic via `fileutil.WriteFileAtomic`). No new dependency: `syscall.Flock` on unix, a no-op elsewhere, like the existing `redirect_stderr_{unix,other}.go` split. |
| D5 | Phases 5–6 (metrics extraction, `tui/actions` move) | Backlog. |

---

## Phase 1 — Bugs and quick fixes (F1, F9, F6, MCP annotations)

No design decisions apart from D4. It can be split into two PRs if F9 grows: 1a = F1/F6/annotations, 1b = F9.

### Task 1.1: TUI compartment lookup falls back to `resolve.CompartmentID` (F1)

**Files:**
- Modify: `internal/ui/tui/model_reducer.go:40-56`
- Test: `internal/ui/tui/model_reducer_test.go`

- [ ] Write a failing test: `known == ""` and zero GPU nodes should make `updateGPUPoolState` return an `updateDoneMsg` with a non-nil `err`. The existing `handleUpdateDoneMsg` path then marks the pools "UNKNOWN".
  - Add a TUI-local seam, `var compartmentIDFn = resolve.CompartmentID`. `resolve`'s own seams are unexported.
- [ ] Have `lookupCompartmentID` return `known` when it is non-empty and `compartmentIDFn(ctx, kubeConfig, env)` otherwise. Delete the local clientset/ListGPUNodes code.
  - Keep the snapshot/merge design from #141 as-is.
- [ ] Run `go test ./internal/ui/tui/... ./internal/resolve/...`.

### Task 1.2: Metadata writes re-read the file (F9)

**Files:**
- Modify: `internal/infra/loader/production/production.go:211-231`
- Modify: `internal/tenantsync/cli.go:79`
- Create: `internal/configloader/metadata_update.go` (with a `_unix`/`_other` lock split)
- Test: `internal/configloader/metadata_update_test.go`, `internal/infra/loader/production/production_test.go`

- [ ] Run `gitnexus_impact({target: "UpsertTenantMetadata", direction: "upstream"})` and `gitnexus_impact({target: "SaveMetadata", direction: "upstream"})`. Expect callers from TUI edit-tenant, CLI `set tenant`, MCP `set_tenant`, and `tenantsync`.
- [ ] Write a failing test with two `Client`s on one temp file:
  - A upserts X, then B upserts Y; the file must contain both.
  - An external `SaveMetadata` (standing in for `tenant sync`) adds a status, then A upserts; the status must survive.
- [ ] Implement `configloader.UpdateMetadata`: lock, re-load (a missing file counts as empty, a parse error refuses), apply the change, `SaveMetadata`, unlock.
- [ ] `UpsertTenantMetadata` calls it with `configloader.UpsertTenant`, then caches the result in `l.metadata`. The `metadataLoadErr` refusal stays.
- [ ] `tenantsync` (`cli.go:79`) writes through `UpdateMetadata` too.
  - It currently reads at the start of a run that can take minutes, so it has the same race in reverse.
  - Give the merge step the freshly re-read file instead of the copy read at the start of the run. Keep the backup step as-is.
- [ ] Update the `tenant sync` long help. TUI/MCP no longer clobber the file; their in-memory view is still stale until they restart or reload.

### Task 1.3: Fix the flag name in MCP descriptions (F6)

**Files:** `internal/config/config.go`, `internal/cli/flags.go:31`, `internal/mcp/mutations.go:453,464`

- [ ] Export `config.FlagMutationEnvOverrideAllowed = "mutation-env-override-allowed"`. Use it in `flags.go` and in `mutationToolFooter`, and fix the comment at line 464.
- [ ] Test: every mutation tool's description contains `"--" + config.FlagMutationEnvOverrideAllowed`.

### Task 1.4: MCP tool annotations (F2, partial)

**Files:** `internal/mcp/tools.go`, `internal/mcp/mutations.go`, `internal/mcp/related.go`

| Tools | Annotation |
|---|---|
| all `list_*`, `get_drain_status` | `ReadOnlyHint: true` |
| `cordon_node`, `uncordon_node`, `scale_gpu_pool`, `set_tenant` | `IdempotentHint: true`, `DestructiveHint: false` |
| `drain_node`, `restart_node`, `restart_inference_service` | `DestructiveHint: false` |
| `terminate_node`, `delete_dac`, `delete_inference_service` | `DestructiveHint: true` |

- [ ] Write a failing test using the in-memory transport (the pattern in `integration_test.go`). List the tools and assert each one's annotations against a table defined in the test.
  - The test must also fail if any registered tool is **missing** from the table. Then a future tool can't ship without annotations.
- [ ] Implement.

**Verify PR 1:** `go test ./...`, `golangci-lint run`, `gitnexus_detect_changes()`.

---

## Phase 2 — Shared mutation policy (F3, F2 via D1/D3)

### Task 2.1: `internal/mutation` package

**Files:** create `internal/mutation/policy.go` and `internal/mutation/policy_test.go`

```go
type Tier int
const (
    Recoverable Tier = iota
    Irreversible
)

type Action struct {
    Verb string // cordon|uncordon|drain|restart|terminate|scale|delete|force-delete|set
    Kind string // node|gpu_pool|dac|inference_service|tenant
    Tier Tier
}

func Actions() []Action                         // canonical list
func Lookup(verb, kind string) (Action, bool)
func Audit(logger logging.Logger, surface string, a Action, target, phase string, err error)
// phase ∈ refused|dry_run|begin|failed|done; field names match today's CLI/MCP records
```

- [ ] Write a tier table test. It pins these as Irreversible:
  - `terminate/node`
  - `delete/dac`
  - `delete/inference_service`
  - `force-delete/inference_service`

  Everything else is Recoverable.
- [ ] Test that the `Audit` field names are exactly `action`, `kind`, `target`, `surface`, `phase`, `dry_run`, `error`. Existing log queries depend on them.
- [ ] The package may import only `pkg/infra/logging`.

### Task 2.2: CLI uses the policy

**Files:** `internal/cli/mutate.go`, plus the files that build `mutationPlan`: `cordon.go`, `drain.go`, `restart.go`, `scale.go`, `delete_dac.go`, `terminate.go`, `set_tenant.go`

- [ ] Run `gitnexus_impact({target: "runMutation", direction: "upstream"})` and report the result. Expect HIGH, but that is positional (fan-out). The change preserves behavior.
- [ ] Replace `mutationPlan.Action/Kind/RequireExplicitYes` with `mutation.Action`.
  - Derive `RequireExplicitYes` from `Tier == Irreversible`.
  - For `delete inference-service`, look up `Lookup(outcome.Action(), "inference_service")` after planning.
  - Write the audit records through `mutation.Audit`.
- [ ] The existing CLI tests must pass **unmodified**. They lock in the output contract.

### Task 2.3: MCP uses the policy, plus D1 and D3

**Files:** `internal/mcp/mutations.go`, `internal/mcp/mutations_test.go`

- [ ] Change `runMutationToolWith` and `handleMutation` to take a `mutation.Action` and audit through `mutation.Audit`.
- [ ] D1: give irreversible tools a `confirm_target` (an `irreversibleGate` embedded next to `confirmGate`).
  - Refuse with `phase=refused` unless `confirm && confirm_target == target`.
  - For `delete_inference_service`, `target` is the **resolved** `namespace/name`. Accept either the bare `name` or `namespace/name`, so an agent that omitted the namespace can still confirm.
  - The tool description says exactly what to echo.
- [ ] D3: add an optional `dry_run`.
  - Split each handler into a `resolve` step (`resolve.GPUNode`, `resolve.GPUPool`, `ResolveInferenceService`, `PlanOrphanInferenceServiceDelete`) and a `perform` step.
  - A dry run executes `resolve` only and returns `{status:"DRY_RUN", action, kind, target, resolved}`, with `phase=dry_run`.
- [ ] Tests:
  - irreversible tools refuse a missing or wrong `confirm_target`
  - a dry run never invokes a perform seam (`mcpTerminateFn`, `mcpDeleteOrphanISVCFn`, …)
  - every `mutation.Actions()` verb/kind has a registered tool

### Task 2.4: TUI audit and tier from the policy

**Files:** `internal/ui/tui/confirm.go`, `internal/ui/tui/isvc_delete.go`, `internal/ui/tui/update_list_ops.go`, `internal/ui/tui/reducer_actions.go`

- [ ] Set `confirmOverlay.tier` from `mutation.Lookup(...)` and remove the local `confirmTier` constants. Modal strings stay local.
- [ ] Emit `mutation.Audit(..., "tui", ...)` at these points:
  - `begin` when the deferred `run` fires
  - `done`/`failed` in each result-message handler: delete, terminate, isvc delete, cordon, drain, restart node/isvc, scale

  These replace the ad-hoc `action started` lines.
- [ ] Test: confirming terminate emits `begin` followed by `done`, and cancelling emits nothing. Use the logging test sink.

**Verify PR 2:**
- Run `go test ./...`.
- Run `gitnexus_detect_changes()`. Only `cli`, `mcp`, `tui` and `mutation` should change.
- Manually run `toolkit terminate <node> -n`. The output must be unchanged.

---

## Phase 3 — MCP read-only mode (F2, D2)

**Files:** `internal/cli/flags.go`, `internal/config/config.go`, `internal/mcp/server.go`, `internal/mcp/tools.go:136`, `internal/mcp/mutations.go:460`

- [ ] Move `get_drain_status` out of `registerMutationTools` into the read-only registration.
- [ ] Add the `--mcp-read-only` flag and `mcp-read-only` config key (default false). When set, skip `registerMutationTools`.
- [ ] Test: in read-only mode, no tool covered by `mutation.Actions()` is listed, and `get_drain_status` is.
- [ ] Docs: the MCP section of `docs/USER_MANUAL.md`.

---

## Phase 4 — Faulty rows, integrity, get-by-name for CLI and MCP (F4, F5)

This phase is now higher value. `delete_inference_service` targets dangling rows, and an agent currently can't list those through MCP.

### Task 4.1: Shared integrity loader and faulty predicate

- [ ] Move `loadIntegrityDataset` (`internal/cli/doctor_relations.go:70`) to `resolve.IntegrityDataset(ctx, ld, cfg, env)`.
- [ ] Move `faultyGateFor` (`internal/ui/tui/table_utils.go:41`) to `relations.FaultyRows(ds, cat) func(group string, row any) bool`, combining `IsFaulty() || dangling`.
- [ ] Export the 8 Ctrl+Z categories as `relations.FaultyCategories()` instead of leaving them in `keys/registry.go`. Add a test asserting that the TUI binds `ToggleFaulty` on exactly those categories.
- [ ] Switch `doctor --relations` and TUI Ctrl+Z to the shared functions. Their existing tests must pass unchanged.

### Task 4.2: `get --faulty` (CLI)

- [ ] Add a `--faulty` flag to `internal/cli/get.go`. It is valid only for `FaultyCategories()`.
  - Load through `resolve.IntegrityDataset` so parents are loaded; otherwise `DanglingRows` would silently pass every row.
  - It composes with `-f`, `--related-to`, `--limit` and `-o`.
- [ ] Test: one orphan GPU node yields exactly that node. A DELETING tenant yields that tenant.

### Task 4.3: MCP `faulty` param and `check_integrity` tool

- [ ] Add an optional `faulty bool` to `listInput`. Tools whose category is not in `FaultyCategories()` return an error when it is set.
- [ ] Add a `check_integrity` tool that mirrors `doctor --relations`: one row per relation with checked/dangling counts and every dangling row. Mention it in the `delete_inference_service` description ("find candidates with list_inference_services faulty=true or check_integrity").

### Task 4.4: Get-by-name

- [ ] CLI: `get <category> [name]`, an exact match on the first column (reuse the `--related-to` parent matching).
  - It emits the single object (default `-o yaml`).
  - No match exits non-zero.
  - Multiple matches (grouped categories) list the candidates and exit non-zero.
- [ ] MCP: `get_item {category, name, env_*}`, with the same rules.

---

## Phase 5 (backlog) — Metrics plan extraction (F7)

- Move `resolveMetricsPlan` and its helpers (`internal/ui/tui/reducer_actions.go:196-490`) into `internal/infra/telemetry` as `telemetry.Plan(ds, item, env)`. When an item spans several DACs, it returns the candidate DACs and the TUI keeps its picker.
- CLI: `metrics <category> <name> [--dac <name>]`.
- MCP: `metrics_url`.

## Phase 6 (backlog) — Layering and minor gaps (F8)

- Move `internal/ui/tui/actions` to `internal/infra/actions`, and `internal/cli/output` to `internal/output`. Package moves are import-path-only commits, so run `gitnexus_impact` first.
- Add explicit TUI cordon/uncordon, or show the resulting state in the confirm text.
- Add CLI `drain <node> --status` (`k8s.GetDrainStatus`).
- Document the ServingRuntime TUI column set.
- Align `list_aliases` and `get alias -o json`. One side's output breaks, so decide which when this phase is scheduled.
- CLI/MCP sort: lift the `table_sort.go` comparators into `internal/columns`. Only on demand.

---

## PR map

| PR | Phase | Findings | Size |
|---|---|---|---|
| 1a | Quick fixes | F1, F6, F2 (annotations) | S |
| 1b | Metadata lost-update | F9 | S–M |
| 2 | Shared mutation policy | F3, F2 (D1, D3) | M |
| 3 | MCP read-only | F2 (D2) | S |
| 4 | Faulty / integrity / get-by-name | F4, F5 (split 4a: 4.1–4.3, 4b: 4.4) | M–L |
| — | Backlog | F7, F8 | — |
