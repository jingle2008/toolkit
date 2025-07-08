# Toolkit

[![CI](https://github.com/jingle2008/toolkit/actions/workflows/ci.yml/badge.svg)](https://github.com/jingle2008/toolkit/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/jingle2008/toolkit)](https://goreportcard.com/report/github.com/jingle2008/toolkit)
[![Go Reference](https://pkg.go.dev/badge/github.com/jingle2008/toolkit.svg)](https://pkg.go.dev/github.com/jingle2008/toolkit)
[![codecov](https://codecov.io/gh/jingle2008/toolkit/branch/main/graph/badge.svg)](https://codecov.io/gh/jingle2008/toolkit)

Toolkit is a collection of reusable Go components exposed through a modular CLI and optional TUI (built with [Bubble Tea](https://github.com/charmbracelet/bubbletea)).  
It targets day-to-day DevOps & development automation: querying Kubernetes, parsing Terraform plans, mass-editing config files, and inspecting large data tables directly in your terminal.

- [Kubernetes Client Fakes & Testing Patterns](docs/k8s-fake-patterns.md)

---

## Feature Highlights

| Area           | Packages                        | Notes |
| -------------- | ------------------------------- | ----- |
| **CLI core**   | `internal/cli`                  | Cobra-based, flags auto-generated |
| **Interactive TUI** | `internal/ui/tui`             | Sort/search/filter large tabular datasets |
| **Infrastructure loaders** | `internal/infra/k8s`, `internal/infra/terraform` | Uniform abstraction for K8s & TF |
| **Config loading & validation** | `internal/config`, `internal/configloader` | JSON / YAML with defaulting & schema checks |
| **Collections helpers** | `internal/collections` | Generic filter/sort with predicates |
| **Encoding helpers** | `internal/encoding/jsonutil` | Fast JSON pointer traversal |
| **Error & logging** | `internal/errors`, `internal/infra/logging` | Typed errors, zap logger |

---

## Install

```bash
# Latest release
go install github.com/jingle2008/toolkit/cmd/toolkit@latest

# From source
git clone https://github.com/jingle2008/toolkit.git
cd toolkit && make
```

```zsh
# From homebrew (macOS)
brew tap jingle2008/homebrew-toolkit
brew install --cask toolkit

# Step to resolve macOS security prompt after installation:
1. Go to System Settings > Privacy & Security.
2. Look for the toolkit app under the Security section.
3. Click Open Anyway and enter your password if prompted.
4. In the pop-up window, click Open to run the app.
```

---

## Usage

```bash
toolkit --help                # all global flags
```

### Global Flags

| Flag            | Default   | Description                        |
| --------------- | --------- | ---------------------------------- |
| `--config, -c`  | *n/a*     | Path to YAML/JSON config file      |
| `--format, -o`  | `table`   | Output: table/json/yaml            |
| `--log-level`   | `info`    | zap log level                      |
| `--no-color`    | `false`   | Disable ANSI colors                |

*(See `internal/cli/root.go` for the authoritative list.)*

---

## Project Layout

```
.
├── cmd/
│   └── toolkit/            # main()
├── internal/
│   ├── cli/                # cobra root & sub-commands
│   ├── ui/tui/             # Bubble Tea models & views
│   ├── infra/
│   │   ├── k8s/            # K8s data sources
│   │   └── terraform/      # Terraform provider
│   ├── config/             # typed config structs
│   ├── configloader/       # env + file loader
│   ├── collections/        # generic filter/sort
│   ├── encoding/jsonutil/  # JSON helpers
│   └── errors/             # error helpers
└── test/
    └── integration/
```

---

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

---

## License

This project is licensed under the MIT License.
