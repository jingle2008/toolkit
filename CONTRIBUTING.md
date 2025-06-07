# Contributing Guidelines

## Testing Best Practices

- **Parallelization:**  
  Add `t.Parallel()` at the top of all independent test functions to enable concurrent execution and faster feedback. For table-driven tests, use sub-tests with `t.Run(...)` and call `t.Parallel()` inside each sub-test.

- **Coverage Enforcement:**  
  The project enforces a minimum code coverage threshold. Use `make cover-check` to verify that coverage is at least 80%. Pull requests that drop coverage below this threshold should be updated with additional tests.

- **Static Analysis & Formatting:**  
  The linter configuration (`.golangci.yml`) enables strict static analysis, including: `govet`, `staticcheck`, `revive`, `errcheck`, `gocognit`, `ineffassign`, `misspell`, `wastedassign`, `paralleltest`, `gocritic`, `gosec`, `contextcheck`, `unused`, `dupl`, `depguard`, `nilnil`, `prealloc`, `unparam`, `dogsled`, `bodyclose`, and `gocyclo`.  
  Run `make lint` before submitting changes.

  **Formatting:**  
  Code must be formatted with [gofumpt](https://github.com/mvdan/gofumpt) (a stricter gofmt). This is not run by golangci-lint and must be run separately, e.g.:
  ```
  gofumpt -w .
  ```
  Consider adding gofumpt as a pre-commit hook or running it in CI to ensure consistent formatting.

- **Benchmarks:**  
  Add micro-benchmarks for performance-sensitive code using `func BenchmarkXxx(b *testing.B)`. Run `go test -bench ./...` to execute benchmarks. Use `benchstat` to compare results and catch regressions.

- **Golden Files:**  
  For output-heavy functions, use golden-file tests. Store expected outputs in `testdata/` and provide a `-update` flag to regenerate them when needed.

- **Fuzzing:**  
  Add fuzz tests for functions that process untrusted or complex input. Store interesting fuzz corpora in `testdata/fuzz/`. Run `go test -run=Fuzz -fuzz=Fuzz -fuzztime=10s ./...` to continuously search for new crashes.

- **Mocks and Fakes:**  
  Use the `internal/testutil` package for shared mocks and fixtures. For Kubernetes-related code, prefer `client-go/fake` or `envtest` for high-fidelity mocks.

## Pull Request Checklist

- [ ] All new and changed code is covered by tests.
- [ ] Tests use `t.Parallel()` where possible.
- [ ] Lint and static analysis pass (`make lint`).
- [ ] Code is formatted with `gofumpt -w .`.
- [ ] Code coverage is at least 80% (`make cover-check`).
- [ ] Benchmarks are added for performance-critical code.
- [ ] Golden files and fuzz corpora are updated as needed.
