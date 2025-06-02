.PHONY: build lint test tidy fmt cover install-lint

VERSION ?= $(shell git describe --tags --always --dirty)

build:
	go install -ldflags "-s -w -X main.Version=$(VERSION)" ./cmd/...

lint:
	golangci-lint run ./...

test:
	go test ./... -race -v

test-int:
	go test -tags=integration ./test/integration -v

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

LINT_VERSION ?= v1.64.8

install-lint:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(LINT_VERSION)
