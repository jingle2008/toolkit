//go:build tools
// +build tools

// This file pins versions of developer tools used in CI and local workflows.
// See Makefile `setup` target for the install commands/versions.
// Keeping them as imports helps `go mod tidy` retain these dependencies.

package tools

import (
	_ "github.com/golangci/golangci-lint/v2/cmd/golangci-lint"
	_ "golang.org/x/tools/cmd/goimports"
	_ "mvdan.cc/gofumpt"
)
