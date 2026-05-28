# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]

## [0.7.1] - 2026-05-27

### Fixed
- **Detail view (`y` from a list row) showed the literal string `null` for `DedicatedAICluster` and `ImportedModel` (and any other category whose key columns are middle-truncated).** Root cause: `applyMiddleTruncation` rewrites Name/Tenant cells in place to elide long OCID-shaped values with `…`, then 8 downstream sites derived `ItemKey` from `m.table.SelectedRow()` — `itemKeyFrom` built a `ScopedItemKey` from the truncated strings, the `DedicatedAIClusterMap`/`ImportedModelMap` lookup missed, `findItem` returned nil, and `jsonutil.Pretty(nil)` rendered `null`. The same path also silently broke `delete`, `toggle cordon`, `drain`, `reboot`, `scale up`, `copy tenant`, and tenant scope-enter actions on those categories (and on the three `*TenancyOverride` categories whose Tenant column is also middle-truncated). Fix: `applyRows` now stores a pre-truncation clone of the rows in `m.rawRows`; a new `selectedRawRow()` helper returns the un-elided row at the cursor; all identity-deriving call sites use it.
- **Export-CSV popup couldn't be cancelled with `esc`.** The bubbles `filepicker.KeyMap.Back` default included `esc` (alongside h/backspace/left for "go up a directory"), so the popup absorbed esc and the only exits were re-pressing `e` (undocumented in the popup's help line) or `q` (quits the whole app). Fix: drop `esc` from the filepicker's Back keymap so h/backspace/left still navigate up a directory, and intercept `esc` in `updateExportView` to restore `m.lastViewMode` — matching the global Back/Clear convention used in every other view.

### Internal
- New `Model.rawRows []table.Row` field + `cloneRows` helper preserve a parallel un-truncated copy of the table for identity lookups; `applyMiddleTruncation` continues to mutate the live rows in place for display.
- `handleItemActions` now threads `itemKey` through to `cordonNode` / `drainNode` / `rebootNode` / `scaleUpGPUPool` instead of each method re-deriving it via `itemKeyFrom(m.category, m.selectedRawRow())`. Single source of truth for the per-key-press identity, four fewer duplicate lookups.

## [0.7.0] - 2026-05-27

### Added
- CLI flag shorthands. `-d` for the root `--debug` flag (interactive use), and `-n` for `--dry-run` on `cordon` / `reboot` / `terminate` / `drain` / `delete dac` / `scale gpu-pool`. Long forms are unchanged.

### Changed
- **TUI load-error handling redesign.** A failed category or dataset load no longer traps the user in a terminal `ErrorView` (where only `q` worked); the error now shows as a transient red toast banner at the bottom of the active view, auto-dismissed after 8s. Retry remains the existing `r` (Refresh) key. The same path covers export errors and detail-render errors. Pressing `r` on the failed category re-runs the load.
- **Commit-navigation on category switch.** Switching to a category whose data hasn't loaded yet now immediately shows the destination chrome — new headers, empty rows — with a small loading indicator in the status bar. Previously the table kept showing the *previous* category's rows under the new category's name, and a failed load locked the user in `ErrorView`. The new behavior matches industry convention (k9s, Gmail, GitHub): commit the navigation, surface the failure inline. Full-screen `LoadingView` is now only used for the very first dataset load, when there's no content to layer over.
- TUI's inline status-bar loading indicator is unstyled (no lipgloss background pill, so it doesn't compete with the context/stats cells) and the elapsed-time stopwatch ticks at 1s intervals (was 500ms).

### Fixed
- **Input lag and dropped keystrokes during navigation when k8s auth is expired.** Two-layer fix: (1) force `ExecProvider.InteractiveMode = Never` on every `rest.Config` so client-go no longer pipes `cmd.Stdin = os.Stdin` to the exec auth plugin — the OCI CLI's interactive prompt was eating keystrokes the user thought they were sending to the TUI; (2) redirect process stderr (fd 2 at the kernel level via `dup2`) to a sibling `<log-file>.stderr` capture file for the duration of the TUI session, so the OCI CLI's "Abort:" output on non-tty prompt failure can't interleave with bubbletea's alt-screen frame writes. Child processes inherit the redirected fd because the swap happens at the kernel level rather than via reassigning `os.Stderr`. Stderr is restored on TUI exit.
- **NPE on category switch when no dataset is loaded.** Pressing `r` or `Tab` after a failed first load used to crash the TUI because the row-source closure dereferenced a nil `dataset` (`d.LimitDefinitionGroup.Values` etc.). `computeTableRows` now returns `(nil, nil)` early when `dataset == nil`.
- **Spinner and stopwatch ticks no longer fire when no load is in flight.** The self-perpetuating tick chain used to keep firing forever after the first load — invisible before this release because the full-screen `LoadingView` swap hid the spinner, but newly visible with the inline indicator. Ticks now die when `pendingTasks` reaches 0 and re-arm on the next load.
- **Lazy-loaded category data arriving in `DetailsView`/`HelpView`/`ExportView` is no longer silently dropped.** Typed `*LoadedMsg` messages and `dataMsg`/`datasetLoadedMsg` are now routed at the top of `Update`; pre-fix, only `updateListView` and `updateLoadingView` consumed them, so a load completing while the user had navigated into a non-list view left `pendingTasks` elevated, the dataset un-updated, and the inline spinner stuck on.
- **Detail-render errors no longer silently swallow the error.** Failures inside `handleDetailContentRenderedMsg` now flow through the same toast path as load errors instead of being captured into an unread `m.err` field.

### Internal
- Removed unreachable `common.ErrorView` constant, `updateErrorView` function, `m.err` field, and three associated `case common.ErrorView:` branches across `delegateToActiveView` / `renderActiveView` / `fullHelpView` — all dead code after the toast-banner switch.
- Promoted `dataMsg`, `datasetLoadedMsg`, and 9 typed `*LoadedMsg` variants to top-level `Update` routing alongside `errMsg` / `spinner.TickMsg` / `stopwatch.TickMsg`. Collapsed the now-dead `routeListDataMsg` helper and shrank `updateLoadingView` to just the Quit handler.
- Dropped `yaml:"tag"` from `PropertyTenancyOverride.TenantID`; `json:"tag"` preserves on-disk back-compat. No user-visible effect because `sigs.k8s.io/yaml` (used by `toolkit get -o yaml`) marshals via JSON struct tags.
- `redirectStderr` helper is split into per-OS files (`redirect_stderr_unix.go` + `redirect_stderr_other.go`) so the binary still builds on linux/arm64 (which only exposes `dup3(2)`) and windows. Uses `golang.org/x/sys/unix.Dup`/`Dup2` instead of the stdlib `syscall` package.

## [0.6.0] - 2026-05-26

### Added
- New `importedmodel` (alias `im`) category covering tenant-owned base models. Two sources are merged: (1) namespaced `ome.io/v1beta1` `BaseModel` CRs across all namespaces, with the originating namespace on `namespace`; (2) cluster-scoped `ClusterBaseModel` CRs carrying a `tenancy-id` label. Items are **grouped by tenant** — the same pattern as `DedicatedAICluster` — with `tenantId` always populated: the label value when present, or `"UNKNOWN_TENANCY"` for orphans (namespaced CRs missing the label, treated as a config error). `namespace` is orthogonal: empty for cluster-scoped CRs, non-empty for namespaced CRs. CLI tables use a `TENANT` column (the resolved Tenant.Name when the OCID suffix matches `tenants/*.json` realm config, raw OCID otherwise); JSON/YAML output adds an `owner` object pointing at the resolved tenant when matched. The TUI lists imported models scoped under their tenant, so you can drill from a tenant row into "this tenant's imported models" (same UX as the DAC drill-down). All existing `BaseModel` fields are JSON-flattened at the top level, so jq pipelines built for `toolkit get basemodel` keep working — `namespace`, `tenantId`, and `owner` are the new keys.
- `toolkit get importedmodel` (CLI) and MCP `list_imported_models` tool, plus a TUI view (5 columns: Name, Tenant, Namespace, Display Name, Status). Imported model names are long OCID suffixes that crowd out other columns; the TUI trims to identity + status while CLI and MCP wire shapes keep every BaseModel field flat. All three surfaces share the same loader; the TUI lazy-loads on first navigation to the category. The TUI's Tenant column shows the resolved `Tenant.Name` (via `Dataset.SetImportedModelMap`) when the OCID suffix matches a tenant in the realm config; the CLI's TENANT column shows the raw OCID (same asymmetry as DAC).
- `BaseModel.storageUri` parsed from `spec.storage.storageUri`. The OCI Object Storage URI (`oci://n/<tenancy>/b/<bucket>/o/<object>`) is where the model artifact actually lives; surfaces on both `toolkit get basemodel` and `toolkit get importedmodel` JSON/YAML output. Empty (omitempty) for CRs without `spec.storage`.

### Changed
- `toolkit get basemodel` (CLI), MCP `list_base_models`, and the TUI BaseModel view no longer include `ClusterBaseModel` CRs carrying a `tenancy-id` label — those are tenant-specific (custom or fine-tuned for a single tenancy) and are now surfaced exclusively under the new `importedmodel` category. Scripts that ran `toolkit get basemodel -o json | jq length` against a cluster with tenant-scoped CBMs will see fewer items in this release; use `importedmodel` (or query both) to recover the full set.
- MCP `list_base_models` description updated to point at `list_imported_models` for tenant-scoped CRs.

### Breaking changes

CLI surface (audit-driven naming sweep):

- **Persistent flags renamed snake_case → kebab-case.** `--repo_path` → `--repo-path`; `--env_type` / `--env_region` / `--env_realm` → `--env-type` / `--env-region` / `--env-realm`; `--metadata_file` → `--metadata-file`; `--log_file` / `--log_format` / `--log_level` → `--log-file` / `--log-format` / `--log-level`; `--mutation_env_override_allowed` → `--mutation-env-override-allowed`. No alias period — scripts and config files using the old names break at this release. **Config file keys (YAML/JSON) follow the same rename**: `repo_path:` → `repo-path:`, etc. Environment variables are unchanged: viper's existing `SetEnvKeyReplacer("-", "_")` keeps `TOOLKIT_REPO_PATH`-style env vars resolving to the new kebab keys.
- **`toolkit version --check` renamed to `--check-updates`.** Bare `--check` was ambiguous; the verbose form spells out the action.
- **`toolkit scale gpupool` renamed to `toolkit scale gpu-pool`.** `gpupool` retained as a Cobra alias (per the same pattern that lets `get gpupool` work), so existing scripts continue.
- **Category display strings became all-caps initialisms.** TUI tabs and CLI output show `GPUPool` / `GPUNode` (was `GpuPool` / `GpuNode`). Auto-computed short aliases shifted from `gp`/`gn` to `gpup`/`gpun`; the legacy `gp` and `gn` are kept as manual aliases, so `toolkit get gp`/`gn` still works.

MCP tool input JSON schema is **unchanged** — agent-facing field names (`env_type`, `env_region`, `env_realm`, `repo_path` references in tool descriptions) stay snake_case. The CLI and MCP layers are intentionally different surfaces.

Public Go API (anyone consuming `pkg/models` as a library):

- **Initialism casing normalized**: every exported `Gpu*` → `GPU*` (types `GpuPool` → `GPUPool`, `GpuNode` → `GPUNode`; fields `GpuPools`, `GpuNodeMap`, `GpuCount`, `GpuShape`, …); every `Dac*` → `DAC*` (`DacShapeConfigs` → `DACShapeConfigs`).
- **`Get*` prefix dropped from non-field-shadowing accessors**: `Filterable.GetFilterableFields` → `FilterableFields`; `RealmedID.GetID(realm, region)` → `OCID(realm, region)`; `RealmedTenancyID.GetTenantID(realm)` → `TenancyOCID(realm)`; plus `GetOwnerState/Usage/GPUs/KubeContext/DefaultDACShape/Flags/Code/GPUConfig/Aliases` (drop prefix on each). `GetName`, `GetDescription`, `GetValue`, `GetRegions`, `GetStatus`, `GetTenants`, and zero-arg `GetTenantID` are **kept** because the new names would shadow same-named struct fields on the receiver types.
- **`Dataset.ResetScopedData` → `Dataset.ResetRealmScopedFields`.** Method body unchanged; name now matches semantics.
- **`PropertyTenancyOverride.Tag` Go field → `TenantID`.** YAML/JSON wire format is preserved (`json:"tag" yaml:"tag"`), so on-disk override files don't need to change. Only Go consumers of the struct literal see the rename.
- **Package-stutter renames**: `loader.Loader` → `loader.Composite`; `production.Loader`/`NewLoader` → `production.Client`/`New`; `jsonutil.PrettyJSON` → `jsonutil.Pretty`; `columns.Set.SelectColumns` → `Set.Select`; `columns.UnknownColumnError` → `columns.UnknownKeyError`.
- **`domain.ToolkitContext` → `domain.Scope`** (file `context.go` → `scope.go`).
- **`internal/infra/k8s.LoadGPUNodes` → `LoadGPUNodesByPool`.** Disambiguates from the flat `ListGPUNodes` in the same package.
- **`internal/resolve.EnrichGPUPools` return type changed from `string` (warning) to `error`** (nil on success). Best-effort semantics are unchanged; the function still doesn't abort the caller's operation.
- **`internal/infra/oci.GetComputeClient`/`GetComputeManagementClient`/`GetGenAIClient` → `NewComputeClient`/`NewComputeManagementClient`/`NewGenAIClient`.**
- **`internal/infra/terraform.GetLocalAttributes` → `LoadLocalAttributes`.**

## [0.5.0] - 2026-05-19

### Changed
- `toolkit get gpupool` (CLI) and MCP `list_gpu_pools` now enrich each pool with live `actualSize` and `status` from OCI's `ListInstancePools` API, matching the TUI's behavior (previously these read paths returned the Terraform-derived placeholders `actualSize: 0` and `status: "..."`). A Terraform-defined pool that hasn't been applied yet renders as `status: "NONEXIST"` (the value `PopulateGpuPools` writes when the OCI list returns 200 but doesn't include that pool). Enrichment degrades gracefully: a K8s/OCI failure surfaces as a stderr warning (CLI) or a `warnings` entry plus a notification (MCP), and Terraform-derived data is still returned. Pre-existing OCI auth requirement is now active for `get gpupool` / `list_gpu_pools` — same auth used by the TUI and the mutation commands. Hosts without kubeconfig/OCI auth will see one K8s-lookup attempt fail (a few hundred ms to a few seconds depending on the kubeconfig path) before the warning is emitted and Terraform data prints — `--no-headers --output json | jq` pipelines previously offline are now online-by-default.
- `toolkit get gpupool -o table` adds two columns: `ACTUAL SIZE` (between `SIZE` and `CAPACITY TYPE`) and `STATUS` (at the end). JSON/JSONL/YAML shape is unchanged — those fields were already on the wire from the model definition.
- CLI `toolkit get gpupool` warning prefixes are now symmetric: partial-Terraform failures are surfaced as `warning: load gpu pools: ...` (was just `warning: ...`), matching the new `warning: gpu pool enrichment incomplete: ...` form so a reader can tell at a glance which step degraded.

## [0.4.0] - 2026-05-19

### Added
- `toolkit get --limit N` and matching `limit` field on every MCP `list_*` tool. Applied after filtering (filter is fuzzy/client-side, so source-side limit at the K8s API would silently break "first N matching" semantics — see commit message for the audit). `0` = unlimited (matches `kubectl --limit`). For grouped categories the cap is across the whole flattened result, not per group.

### Breaking
- MCP `list_gpu_nodes`/`list_dacs`/`list_model_artifacts` and CLI `toolkit get gpunode|dac|modelartifact -o json|jsonl|yaml` no longer inject a `pool` / `tenant` / `model` top-level field — those duplicated the existing `poolName` / `tenantId` / `model_name` fields the loader was already setting (the loader keys each grouped map by that same field, so the wrapper was strictly redundant). Consumers should switch to reading `poolName` (GpuNode), `tenantId` (DAC), or `model_name` (ModelArtifact). The `*tenancyoverride` categories keep their `tenant` injection (those source TenantID/Tag from JSON content, not the directory name used as the map key). Table / CSV / TSV output is unaffected — those still use the per-category `POOL` / `TENANT` / `MODEL` column for visual grouping.

## [0.3.1] - 2026-05-19

### Breaking
- Homebrew tap moved from `jingle2008/homebrew-toolkit` to the centralized `jingle2008/homebrew-tap`. Install command is now `brew install jingle2008/tap/toolkit`. Existing users migrate with `brew uninstall toolkit && brew untap jingle2008/toolkit && brew install jingle2008/tap/toolkit` (the old tap is no longer updated).

### Fixed
- `brews:` block was restored (had been removed in v0.3.0's deprecation cleanup, which left Linuxbrew users without an install path and froze the stale tap Formula). Its `test:` block now invokes `toolkit version` (subcommand) instead of `--version` (the CLI doesn't accept it as a flag, so `brew test toolkit` was erroring on every install).

### Added
- macOS Developer ID code-signing + Apple notarytool submission via GoReleaser's `notarize.macos` block. Disabled until the five `MACOS_*` secrets are configured (see `docs/release/macos-notarization.md`); when active, drops the Gatekeeper quarantine prompt on first launch.

## [0.3.0] - 2026-05-19

### Breaking
- `GpuPool` JSON shape changed: fields now use lowercase / camelCase tags (`name`, `shape`, `actualSize`, `capacityType`, `isOkeManaged`, `availabilityDomain`) to match every other model in `pkg/models/`. Scripts that targeted `.Name` / `.Shape` etc. on `toolkit get gpupool -o json` or the MCP `list_gpu_pools` output need to switch to the lowercase keys.

### Added
- `toolkit doctor` — read-only health-check subcommand that aggregates the file-existence and schema checks scattered across the subcommands into one report. Each row is PASS / FAIL / SKIP with a remediation hint; exit non-zero on any FAIL. Renders `table` (default), `json`, or `yaml`.
- `docs/recipes.md` — four end-to-end flows: wire `toolkit mcp` into Claude Desktop / Claude Code / Codex CLI; GPU node maintenance window (cordon → drain → reboot → uncordon); tenants → CSV / TSV → spreadsheet; daily GPU-pool digest to Slack via `jq` + `curl` (cron / launchd).
- Architecture mermaid diagram in the README showing how config + data sources funnel through the loader into the four surfaces (TUI, headless `get`, MCP, mutations).

### Changed
- Every Kubernetes client call now has a 30s per-request timeout (`internal/infra/k8s/client.go`). A broken or unreachable cluster fails the spinner in seconds instead of hanging on TCP dial / TLS handshake. Override via `k8s.RequestTimeout` before any client is built; setting it to zero restores client-go's no-timeout default.
- `release-drafter` autolabels PRs from conventional-commit prefixes (`feat:` / `fix:` / `refactor:` / `docs:` / etc.) and resolves the next version automatically (minor on `feat`, patch on most others, major on `breaking` label). Bumps the action to v6.
- CI now exercises `.goreleaser.yaml` on every push/PR via a `release-snapshot` job, so config drift fails fast instead of on tag push.
- `toolkit config --validate` is now schema-stable: pass and fail paths emit the same `{valid, config_file, error?}` shape; the redundant `config` key inside the `settings` map is dropped (the top-level `config_file` is authoritative); `--pretty` is exposed for parity with `toolkit get`.
- `cordon` / `uncordon` `--help` now carries a `Long:` block with examples to match the other mutation subcommands.

### Fixed
- README mutation table referenced flags that didn't exist (`--confirm`; `--size` on `scale gpupool`). The actual flag is `--yes` / `-y`, and `scale gpupool` derives size from Terraform.
- `docs/recipes.md` (introduced in this release) initially pointed `jq` at fictional fields (`.status` on GPU nodes; lowercase pool fields before the JSON-tag change; a `{status: .result}` shape in the audit log). Now matches the real envelope.
- `.goreleaser.yaml` cleared all v2 deprecation warnings: `snapshot.name_template` → `version_template`; legacy `brews:` block removed in favor of `homebrew_casks:`; `homebrew_casks.binary` dropped (auto-detected).
- `.github/workflows/release.yml` installs `syft` (was missing, killed the first v0.2.0 release attempt) and tracks Go version via `go-version-file: go.mod` instead of a hard pin.

## [0.2.0] - 2026-05-18

This release adds a headless CLI surface, an MCP server, and node/pool mutation subcommands. Everything previously available only through the TUI is now scriptable.

### Added

#### Headless CLI
- `toolkit get <category>` prints any category to stdout in `table | json | jsonl | yaml | csv | tsv` (`-o`, `--no-headers`, `--pretty`). Accepts the same aliases the TUI uses (`t`, `bm`, `dac`, `gn`, …); `toolkit get alias` lists them all.
- `toolkit config` prints the effective merged settings (defaults + `TOOLKIT_*` env + config file + flags) plus the resolved config-file path and an `exists` boolean. `--validate` mode runs `config.Validate()` and exits non-zero on failure, with a structured `{valid, config_file, error?}` payload on stdout.
- Mutation subcommands (`--confirm` required, `--dry-run` available, JSON audit log):
  `toolkit cordon` / `uncordon` (Kubernetes node), `toolkit drain`, `toolkit reboot`, `toolkit scale gpupool`, `toolkit delete dac`, `toolkit terminate` (OCI instance).
- `toolkit completion` for bash/zsh/fish/powershell.

#### MCP server
- `toolkit mcp` exposes a stdio MCP server. Read tools cover every category: `list_tenants`, `list_base_models`, `list_gpu_pools`, `list_gpu_nodes`, `list_dacs`, `list_environments`, `list_service_tenancies`, `list_model_artifacts`, `list_definitions`, `list_tenancy_overrides`, `list_regional_overrides`, `list_aliases`.
- Mutation tools mirror the CLI subcommands and are gated on `confirm=true`; failures surface via `notifications/message` and a unified audit log.
- `mutation_env_override_allowed` flag opts mutation tools into per-call env override (default off — operator's startup env is the maximum blast radius).

#### TUI
- GPU node inspection reports pod-level issues (Pending/Failed pods scheduled to the node).
- Table stats: aggregate counts displayed in the status bar for GpuPool, GpuNode, and DedicatedAICluster.
- ScaleUp shortcut (`shift+u`) for OCI GPU instance pools (GpuPool view).
- DAC view shows model info plus Active vs Failed counts.
- GpuPool gains an `AvailabilityDomain` column (extracted from Terraform `placement_logical_ad`, supports string/list/`"all"`).

### Changed
- Terraform loader is partial-tolerant: `LoadGpuPools` returns a typed `PartialLoadError` instead of failing the whole load when one source can't be resolved. Both TUI and `toolkit get` surface the warning on stderr without dropping the rows that did load.
- TUI internals: typed messages route directly to typed handlers; per-generation cancellable load contexts replace the long-lived `m.ctx`; loader commands are pure; lazy loads carry generation guards; immutable lipgloss styles are centralized.
- `errors.As` replaced with a typed `errors.AsType` helper across the codebase.
- `GpuPool.InstancePoolId` → `GpuPool.ID`.

### Fixed
- `toolkit get` now reads `~/.config/toolkit/config.yaml` (previously the headless path ignored the user's config file).
- Filter debounce and stale-data guards no longer flicker results during fast typing.
- OCI helpers no longer nil-deref on the non-prod path (`client.Host` instead of `Endpoint()`) and the URL parser tolerates missing schemes.
- TUI list view bottom border no longer shifts after exiting detail view.
- `BaseModel.GetDefaultDacShape` no longer panics on unknown shapes.
- `UNKNOWN` region fallback replaced with a city-segment slug so special-region IDs resolve correctly.

### Build / CI
- Go toolchain pinned to 1.26.1 (closes stdlib CVEs).
- User manual added (`docs/`).
- SBOM + GPG-signed checksums produced by GoReleaser; Homebrew tap auto-updated.

## [0.1.0] - 2025-06-06
- Initial release.
