# Test targets:
#   make test      - Run all unit tests (excludes integration tests)
#   make test-int  - Run integration tests only (requires explicit build tag)
#   make cover     - Unit test coverage report
#   make cover-int - Integration test coverage report

.PHONY: build lint test tidy fmt cover install-lint

VERSION ?= $(shell git describe --tags --always --dirty)

build:
	go install -ldflags "-s -w -X main.Version=$(VERSION)" ./cmd/...

lint:
	golangci-lint run ./...

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

LINT_VERSION ?= v1.64.8

install-lint:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(LINT_VERSION)
