# Toolkit

[![CI](https://github.com/jingle2008/toolkit/actions/workflows/ci.yml/badge.svg)](https://github.com/jingle2008/toolkit/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/jingle2008/toolkit)](https://goreportcard.com/report/github.com/jingle2008/toolkit)
[![Go Reference](https://pkg.go.dev/badge/github.com/jingle2008/toolkit.svg)](https://pkg.go.dev/github.com/jingle2008/toolkit)
[![codecov](https://codecov.io/gh/jingle2008/toolkit/branch/main/graph/badge.svg)](https://codecov.io/gh/jingle2008/toolkit)

Toolkit is a modular command-line utility written in Go, designed to provide a collection of tools and utilities for various development and automation tasks. The project is organized for extensibility and maintainability, following Go best practices.

## Features

- **Modular CLI**: Easily extendable command-line interface.
- **Category-based Utilities**: Organized by categories for clear separation of concerns.
- **Configurable**: Uses Go modules and a Makefile for streamlined building and management.
- **Test Coverage**: Includes unit tests for core logic.
- **Structured Logging**: Uses zap for machine-readable, robust logs.

## Project Structure

```
.
├── cmd/                  # Entry points for CLI commands
│   └── toolkit/          # Main CLI application
│       └── main.go
├── internal/             # Internal application logic
│   └── app/
│       └── toolkit/
│           ├── category.go
│           ├── constants.go
│           ├── headers.go
│           ├── key_map.go
│           ├── loader.go
│           ├── logging.go
│           ├── model.go
│           ├── options.go
│           ├── render.go
│           ├── requestctx.go
│           ├── row_marshaler.go
│           ├── rows_cluster.go
│           ├── rows_environment.go
│           ├── rows_gpu.go
│           ├── rows_service.go
│           ├── rows_tenancy.go
│           ├── rows_tenant.go
│           ├── table_utils.go
│           └── table_utils_test.go
├── Makefile              # Build and management commands
├── go.mod                # Go module definition
├── go.sum                # Go module checksums
└── .gitignore
```

## Getting Started

### Prerequisites

- Go 1.18 or later

## Configuration

Toolkit can be configured via CLI flags or environment variables. Flags take precedence over environment variables.

| Flag           | Env Variable           | Description                        | Default                |
|----------------|-----------------------|------------------------------------|------------------------|
| --repo         | TOOLKIT_REPO_PATH     | Path to repo                       | (none)                 |
| --kubeconfig   | KUBECONFIG            | Path to kubeconfig                 | ~/.kube/config         |
| --envtype      | TOOLKIT_ENV_TYPE      | Environment type                   | preprod                |
| --envregion    | TOOLKIT_ENV_REGION    | Environment region                 | us-chicago-1           |
| --envrealm     | TOOLKIT_ENV_REALM     | Environment realm                  | oc1                    |
| --category     | TOOLKIT_CATEGORY      | Toolkit category                   | Tenant                 |

Example usage:
```sh
./bin/toolkit --repo /path/to/repo --envtype prod
```
or
```sh
export TOOLKIT_REPO_PATH=/path/to/repo
export TOOLKIT_ENV_TYPE=prod
./bin/toolkit
```

### Build

```sh
make
```
or
```sh
go build -o bin/toolkit ./cmd/toolkit
```

### Run

```sh
./bin/toolkit --help
```
or, if built with Go:
```sh
go run ./cmd/toolkit --help
```

## Testing

Run all tests with:
```sh
go test ./...
```

### Test Organization & CI

- **Unit tests** are located next to their source files and use the `_test.go` suffix.
- **Integration tests** are in `test/integration/` and use Go build tags (`//go:build integration`).
- **Fixtures** for tests are stored in `testdata/` directories within each package.
- **External test packages** (e.g., `package utils_test`) are used where possible to verify public APIs.
- **Table-driven and sub-tests** are encouraged for clarity and parallelization.
- **Test helpers** and mocks are placed in dedicated `*_test_helpers.go` files.

#### Running tests

- Unit tests (default):
  ```sh
  make test
  ```
- Integration tests (with build tag):
  ```sh
  make test-int
  ```
- Coverage reports:
  ```sh
  make cover      # unit test coverage
  make cover-int  # integration test coverage
  ```

#### Continuous Integration

- **Unit tests** run on all pushes and pull requests.
- **Integration tests** run on pushes to `main` and nightly (see `.github/workflows/ci.yml`).
- **CI target**: Run `make ci` to execute both lint and test in one step (recommended for local and CI use).

## Developer Workflow

- Run `make ci` before pushing to ensure code passes lint and tests.
- Use `make lint` to check for style and static analysis issues.
- Use `make test` for a full race-enabled test run.
- Use `make fmt` and `make tidy` to auto-format and tidy dependencies.

## Architecture Overview

Toolkit follows a modular, testable architecture:
- **Loader interfaces** (see `internal/app/toolkit/loader.go`): Abstract data loading for datasets, models, GPU pools, etc. Split by concern for testability and clean dependency injection.
- **Renderer interfaces** (see `internal/app/toolkit/render.go`): Abstract rendering logic for different output formats (e.g., JSON, table).
- **Model** (see `internal/app/toolkit/model.go`): Central state and update logic, using the Bubble Tea TUI pattern. Composed via functional options for flexibility.
- **Category enum** (see `internal/app/toolkit/category.go`): Strongly-typed, extensible grouping for all toolkit data and UI.

## Logging

Toolkit uses [zap](https://github.com/uber-go/zap) for structured, machine-readable logging. Logs are written to `debug.log` by default.

## Contributing

Contributions are welcome! Please open issues or submit pull requests for new features, bug fixes, or improvements.

## License

This project is licensed under the MIT License.
