# Contributing Guidelines

## Testing Best Practices

- **Parallelization:**  
  Add `t.Parallel()` at the top of all independent test functions to enable concurrent execution and faster feedback. For table-driven tests, use sub-tests with `t.Run(...)` and call `t.Parallel()` inside each sub-test.

- **Serial tests & package seams:**  
  Prefer `t.Parallel()`, but some tests are intentionally serial and that is acceptable. Two causes exist in this codebase: (1) package-global function-pointer seams (e.g. `newGenAIClient`, `mcpSetCordonFn`, `resolveGPUPoolFn`) that tests swap to inject fakes — use the `swap(&fn, fake)()` helper with its deferred restore; (2) the global Viper/Cobra singleton, which CLI tests mutate via `viper.Reset()` / `t.Setenv`. When a test mutates shared global state, mark it `//nolint:paralleltest` with a one-line justification (e.g. `// mutates the package-global X seam`).  
  New code should prefer **dependency injection** (constructor params or struct fields) over adding new package-global seams, so the set of serial tests does not grow. Migrating the existing seams to injected dependencies — to let those tests run in parallel — is a known, consciously-deferred improvement (review finding #4); it was scoped out as low-ROI relative to its blast radius.

- **Coverage Enforcement:**  
  The project enforces a minimum code coverage threshold. Use `make cover-check` to verify that coverage is at least 80%. Pull requests that drop coverage below this threshold should be updated with additional tests.

- **Static Analysis & Formatting:**  
  The linter configuration (`.golangci.yml`) enables strict static analysis, including: `govet`, `staticcheck`, `revive`, `errcheck`, `gocognit`, `ineffassign`, `misspell`, `wastedassign`, `paralleltest`, `gocritic`, `gosec`, `contextcheck`, `unused`, `dupl`, `depguard`, `nilnil`, `prealloc`, `unparam`, `dogsled`, `bodyclose`, `cyclop`, and `goimports`.  
  Run `make lint` before submitting changes.

  **Formatting:**  
  Code must be formatted with [gofumpt](https://github.com/mvdan/gofumpt) and imports organized with `goimports` (with local prefix set to `github.com/jingle2008/toolkit`).  
  You can run:
  ```
  make fmt
  make goimports
  ```
  or directly:
  ```
  gofumpt -w .
  goimports -w -local github.com/jingle2008/toolkit .
  ```
  Consider enabling pre-commit hooks: `pre-commit install`, or run `make ci` before pushing to ensure formatting, lint, and tests pass.

- **Benchmarks:**  
  Add micro-benchmarks for performance-sensitive code using `func BenchmarkXxx(b *testing.B)`. Run `go test -bench ./...` to execute benchmarks. Use `benchstat` to compare results and catch regressions.

- **Golden Files:**  
  For output-heavy functions, use golden-file tests. Store expected outputs in `testdata/` and provide a `-update` flag to regenerate them when needed.

- **Fuzzing:**  
  Add fuzz tests for functions that process untrusted or complex input. Store interesting fuzz corpora in `testdata/fuzz/`. Run `go test -run=Fuzz -fuzz=Fuzz -fuzztime=10s ./...` to continuously search for new crashes.

- **Mocks and Fakes:**  
  Use the `internal/testutil` package for shared mocks and fixtures. For Kubernetes-related code, prefer `client-go/fake` or `envtest` for high-fidelity mocks.

## Release infrastructure (maintainer-only)

- **macOS code-signing & notarization** depends on an active **Apple Developer Program** enrollment ($99/year). The release pipeline (`notarize.macos` in `.goreleaser.yaml`) stays inert until the five `MACOS_*` secrets are configured; if the Apple cert lapses, releases continue to publish unsigned binaries (Gatekeeper-quarantined on macOS users' first run) until the cert is renewed and the secrets refreshed. See [`docs/release/macos-notarization.md`](docs/release/macos-notarization.md) for the full setup checklist and recovery steps.
- **Homebrew tap PAT** in `GH_TOKEN` needs `Contents: write` on both `jingle2008/toolkit` and `jingle2008/homebrew-tap`. Fine-grained PATs expire on a schedule; the release workflow will fail at the tap-push step if the token lapses. The recovery path is to refresh the PAT in repo settings, then `gh run rerun --failed` on the affected release run.

## Pull Request Checklist

- [ ] All new and changed code is covered by tests.
- [ ] Tests use `t.Parallel()` where possible.
- [ ] Lint and static analysis pass (`make lint`).
- [ ] Code is formatted with `gofumpt -w .`.
- [ ] Code coverage is at least 80% (`make cover-check`).
- [ ] Benchmarks are added for performance-critical code.
- [ ] Golden files and fuzz corpora are updated as needed.
