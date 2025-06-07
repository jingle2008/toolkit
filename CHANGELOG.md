# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]

### Added
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
