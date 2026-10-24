# Test targets:
#   make test      - Run all unit tests (excludes integration tests)
#   make test-int  - Run integration tests only (requires explicit build tag)
#   make cover     - Unit test coverage report
#   make cover-int - Integration test coverage report

.PHONY: build lint vet test bench test-int bench-int tidy fmt fmt-check goimports goimports-check cover cover-int cover-check install-lint setup ci clean

VERSION ?= $(shell git describe --tags --always --dirty)

# Dev tools (golangci-lint, gofumpt, goimports) are pinned by `tool`
# directives in the separate tools module (tools/go.mod), which keeps their
# dependency trees out of the main module graph. tools/go.mod is the single
# source of truth for their versions; bump with:
#   go -C tools get -tool <module>@<version>
GOTOOL := go tool -modfile=tools/go.mod
LOCAL_PREFIX := github.com/jingle2008/toolkit

build:
	go install -trimpath -ldflags "-s -w -X main.version=$(VERSION)" ./cmd/...

lint:
	$(GOTOOL) golangci-lint config verify
	$(GOTOOL) golangci-lint run ./...

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
	go -C tools mod tidy

fmt:
	$(GOTOOL) gofumpt -w .

fmt-check:
	@diffs=$$($(GOTOOL) gofumpt -l .); if [ -n "$$diffs" ]; then echo "Run 'make fmt' to format the following files:"; echo "$$diffs"; exit 1; fi

# Format imports using goimports (enforces import grouping)
goimports:
	$(GOTOOL) goimports -w -local $(LOCAL_PREFIX) .

goimports-check:
	@diffs=$$($(GOTOOL) goimports -l -local $(LOCAL_PREFIX) .); if [ -n "$$diffs" ]; then echo "Run 'make goimports' to fix imports in:"; echo "$$diffs"; exit 1; fi

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

# Nothing is installed into GOBIN: `go tool` builds each tool on first use
# and caches the binary. These targets just download and prebuild them so
# the first lint/fmt run (and CI) doesn't pay that cost mid-check.
install-lint:
	$(GOTOOL) golangci-lint version

setup:
	go -C tools mod download
	$(MAKE) install-lint
	$(GOTOOL) gofumpt --version
	$(GOTOOL) goimports </dev/null

ci: fmt-check goimports-check lint vet cover-check

clean:
	rm -f coverage.out coverage.html coverage-int.out coverage-int.html toolkit.log
	rm -rf bin/
