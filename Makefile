# Test targets:
#   make test      - Run all unit tests (excludes integration tests)
#   make test-int  - Run integration tests only (requires explicit build tag)
#   make cover     - Unit test coverage report
#   make cover-int - Integration test coverage report

.PHONY: build lint vet test bench bench-int tidy fmt fmt-check goimports goimports-check cover cover-int cover-check install-lint setup ci clean

VERSION ?= $(shell git describe --tags --always --dirty)

build:
	go install -trimpath -ldflags "-s -w -X main.version=$(VERSION)" ./cmd/...

lint:
	golangci-lint config verify
	golangci-lint run ./...

vet:
	go vet ./...

test:
	go test ./... -race -shuffle=on -count=1 -v

bench:
	go test -bench=. -benchmem ./...

test-int:
	go test -tags=integration ./test/integration -v

bench-int:
	go test -tags=integration -bench=. -benchmem ./test/integration

tidy:
	go mod tidy

fmt:
	gofumpt -w .

fmt-check:
	@diffs=$$(gofumpt -l .); if [ -n "$$diffs" ]; then echo "Run 'make fmt' to format the following files:"; echo "$$diffs"; exit 1; fi

# Format imports using goimports (enforces import grouping)
goimports:
	goimports -w -local github.com/jingle2008/toolkit .

goimports-check:
	@diffs=$$(goimports -l -local github.com/jingle2008/toolkit .); if [ -n "$$diffs" ]; then echo "Run 'make goimports' to fix imports in:"; echo "$$diffs"; exit 1; fi

cover:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "Open coverage.html in your browser to view the report."

cover-int:
	go test -tags=integration ./test/integration -coverprofile=coverage-int.out
	go tool cover -html=coverage-int.out -o coverage-int.html
	@echo "Open coverage-int.html in your browser to view the integration test report."

cover-check:
	go test ./... -race -shuffle=on -count=1 -covermode=atomic -coverprofile=coverage.out
	go tool cover -func=coverage.out | awk '/total:/ {if ($$3+0 < 80) {print "Coverage below 80%"; exit 1}}'

# Keep these in sync with the module pins in go.mod (single source of truth):
#   golangci-lint -> github.com/golangci/golangci-lint/v2
#   gofumpt       -> mvdan.cc/gofumpt
#   goimports     -> golang.org/x/tools
LINT_VERSION ?= v2.12.2
GOFUMPT_VERSION ?= v0.10.0
GOIMPORTS_VERSION ?= v0.46.0

install-lint:
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(LINT_VERSION)

setup:
	$(MAKE) install-lint
	go install mvdan.cc/gofumpt@$(GOFUMPT_VERSION)
	go install golang.org/x/tools/cmd/goimports@$(GOIMPORTS_VERSION)

ci: fmt-check goimports-check lint vet cover-check

clean:
	rm -f coverage.out coverage.html coverage-int.out coverage-int.html toolkit.log
	rm -rf bin/
