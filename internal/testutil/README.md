# testutil

This package contains shared test helpers, mocks, and fixtures for use across unit and integration tests.

- All code here should be test-only (never imported by production code).
- Use the `_test.go` suffix for all files to ensure they are excluded from production builds.
- Place generic mocks, fake clients, and golden-file helpers here to avoid duplication across packages.

## Usage

Import as `testutil` in your test files:

```go
import "yourmodule/internal/testutil"
