# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]

### Added
- New `importedmodel` (alias `im`) category covering tenant-imported models. Two sources are merged: (1) namespaced `ome.io/v1beta1` `BaseModel` CRs across all namespaces, with the originating namespace on `namespace`; (2) cluster-scoped `ClusterBaseModel` CRs carrying a `tenancy-id` label, with the label value on `tenantId`. `namespace` and `tenantId` are **orthogonal facets**, not synonyms: `namespace` is the K8s scope (empty ⇒ cluster-scoped CBM; non-empty ⇒ namespaced BM — this is the authoritative source-kind indicator), while `tenantId` is the OCI tenant identifier sourced from the `tenancy-id` label and may appear on either source kind (same pattern as `DedicatedAICluster.tenantId`). Consumers wanting "which K8s scope" should read `namespace`; consumers wanting "which OCI tenant" should read `tenantId`. All existing `BaseModel` fields are JSON-flattened at the top level, so jq pipelines built for `toolkit get basemodel` keep working — only `namespace` and `tenantId` are new keys.
- `toolkit get importedmodel` (CLI) and MCP `list_imported_models` tool, plus a TUI view (8 columns: Name, Namespace, Tenant ID, Display Name, Version, DAC Shape, Flags, Status). All three surfaces share the same loader; the TUI lazy-loads on first navigation to the category.
- `BaseModel.storageUri` parsed from `spec.storage.storageUri`. The OCI Object Storage URI (`oci://n/<tenancy>/b/<bucket>/o/<object>`) is where the model artifact actually lives; surfaces on both `toolkit get basemodel` and `toolkit get importedmodel` JSON/YAML output. Empty (omitempty) for CRs without `spec.storage`.

### Changed
- `toolkit get basemodel` (CLI), MCP `list_base_models`, and the TUI BaseModel view no longer include `ClusterBaseModel` CRs carrying a `tenancy-id` label — those are tenant-specific (custom or fine-tuned for a single tenancy) and are now surfaced exclusively under the new `importedmodel` category. Scripts that ran `toolkit get basemodel -o json | jq length` against a cluster with tenant-scoped CBMs will see fewer items in this release; use `importedmodel` (or query both) to recover the full set.
- MCP `list_base_models` description updated to point at `list_imported_models` for tenant-scoped CRs.

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
