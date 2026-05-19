# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]

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
