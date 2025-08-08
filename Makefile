# Test targets:
#   make test      - Run all unit tests (excludes integration tests)
#   make test-int  - Run integration tests only (requires explicit build tag)
#   make cover     - Unit test coverage report
#   make cover-int - Integration test coverage report

.PHONY: build lint vet test bench bench-int tidy fmt fmt-check goimports goimports-check cover cover-int cover-check install-lint setup ci

VERSION ?= $(shell git describe --tags --always --dirty)

build:
	go install -ldflags "-s -w -trimpath -X main.version=$(VERSION)" ./cmd/...

lint:
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
	go test ./... -covermode=atomic -coverprofile=coverage.out
	go tool cover -func=coverage.out | awk '/total:/ {if ($$3+0 < 80) {print "Coverage below 80%"; exit 1}}'

LINT_VERSION ?= v2.3.1

install-lint:
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(LINT_VERSION)

setup:
	$(MAKE) install-lint
	go install mvdan.cc/gofumpt@v0.6.0
	go install golang.org/x/tools/cmd/goimports@v0.28.0

ci: fmt-check goimports-check lint vet test cover-check
