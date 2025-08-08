# Toolkit

[![CI](https://github.com/jingle2008/toolkit/actions/workflows/ci.yml/badge.svg)](https://github.com/jingle2008/toolkit/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/jingle2008/toolkit)](https://goreportcard.com/report/github.com/jingle2008/toolkit)
[![Go Reference](https://pkg.go.dev/badge/github.com/jingle2008/toolkit.svg)](https://pkg.go.dev/github.com/jingle2008/toolkit)
[![codecov](https://codecov.io/gh/jingle2008/toolkit/branch/main/graph/badge.svg)](https://codecov.io/gh/jingle2008/toolkit)

Toolkit is a collection of reusable Go components exposed through a modular CLI and optional TUI (built with [Bubble Tea](https://github.com/charmbracelet/bubbletea)).  
It targets day-to-day DevOps & development automation: querying Kubernetes, parsing Terraform plans, mass-editing config files, and inspecting large data tables directly in your terminal.

- [Kubernetes Client Fakes & Testing Patterns](docs/guide/k8s-fake-patterns.md)

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
| **Error & logging** | `pkg/infra/logging` | Typed errors via std errors; zap logger |

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
# From Homebrew (macOS/Linux)
brew tap jingle2008/homebrew-toolkit
brew install toolkit
```

---

## Getting Started

After installation, try these quick commands:

```sh
toolkit init
# Scaffold an example config file at ~/.config/toolkit/config.yaml

toolkit completion bash   # or zsh/fish
# Output shell completion script for your shell

toolkit version --check
# Print your installed version and check for updates
```

---

## Usage

```bash
toolkit --help                # all global flags
```

### Global Flags

| Flag | Default | Description |
| ---- | ------- | ----------- |
| `--config` | `~/.config/toolkit/config.yaml` | Path to config file (YAML or JSON) |
| `--repo_path` |  | Path to the repository |
| `--env_type` |  | Environment type (e.g. dev, prod) |
| `--env_region` |  | Environment region |
| `--env_realm` |  | Environment realm |
| `--category, -c` |  | Category to display |
| `--filter, -f` |  | Initial filter for current category |
| `--metadata_file` | `~/.config/toolkit/metadata.yaml` | Optional additional metadata file |
| `--kubeconfig` | `~/.kube/config` | Path to kubeconfig file |
| `--log_file` | `toolkit.log` | Path to log file |
| `--debug` | `false` | Enable debug logging |
| `--log_format` | `console` | Log format: console or json |

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
│   └── encoding/jsonutil/  # JSON helpers
├── pkg/
│   ├── infra/logging/      # zap-based logging
│   └── models/             # domain models and types
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

- One-time setup: `make setup` (installs golangci-lint, gofumpt, goimports)
- Optional: enable git hooks with pre-commit: `pre-commit install`
- Run `make ci` before pushing to ensure code passes lint and tests.
- Use `make lint` to check for style and static analysis issues.
- Use `make test` for a full race-enabled test run.
- Use `make fmt` and `make tidy` to auto-format and tidy dependencies.

## Architecture Overview

Toolkit follows a modular, testable architecture:
- **Loaders**: `internal/infra/loader` provides concrete and interface-based loaders for datasets (K8s, Terraform, OCI), enabling dependency injection and testability.
- **TUI Model**: `internal/ui/tui` contains Bubble Tea models, views, and update loop; composed via functional options.
- **Domain types**: `pkg/models` defines strongly-typed domain models used across loaders and UI.
- **Category enum**: `internal/domain/category.go` provides strongly-typed categories and parsing.
- **Logging**: `pkg/infra/logging` wraps zap for structured logs with configurable format and file path.

## Logging

Toolkit uses [zap](https://github.com/uber-go/zap) for structured, machine-readable logging. By default logs are written to `toolkit.log` (configurable via `--log_file`) and support `--log_format` of `console` or `json`.

## Contributing

Contributions are welcome! Please open issues or submit pull requests for new features, bug fixes, or improvements.

---

## License

This project is licensed under the MIT License.
