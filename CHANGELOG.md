# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]

### Fixed
- Prevent nil-pointer panic in loading view stopwatch when handling spinner tick during tests (`update_loading.go`, affected Test_updateLoadingView_SpinnerTick).

### Added / Changed
- GPU pool display now includes **Actual Size** and **Status** columns; fault detection reflects OCI-reported size.
- New OCI helper `GetComputeManagementClient` and refactored `PopulatePoolFromOCI`.
- `listGpuNodes` now supports a `limit` parameter via Kubernetes `ListOptions`, plus updated helpers and tests.

### Added
- Table stats: Added a new `tableStats` type (map[string]int) and integrated it into the TUI. The table now displays aggregate statistics for selected columns in the status bar for GpuPool, GpuNode, and DedicatedAICluster categories.
- `getTableRows` now returns both rows and stats, and stats are computed for specific columns per category.
- Status bar displays stats in "key: value" format.
- All usages and tests updated for new signature and behaviour.

- Generic filter utility using Go generics; deduplicated filter logic and tests for environment, service, and tenant domains.
- Table-driven tests for all domain filters.
- Migration to `spf13/pflag` for CLI flags.
- Pre-commit hooks for `gofumpt`, `golangci-lint`, and `go vet`.
- Improved error handling with `log.Fatal` for unrecoverable errors.
- Category enum now uses generated stringer method; tests and UI updated for Go-style names.
- Makefile targets for build, lint, vet, test, coverage, and CI.

### Changed
- Refactored CLI flag parsing and configuration logic.
- Updated all code and tests to expect Go-style enum names for Category.

### Removed
- Manual String method for Category enum (now generated).

## [0.1.0] - 2025-06-06
- Initial release.
