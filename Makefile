.PHONY: build lint test tidy fmt cover install-lint

VERSION ?= $(shell git describe --tags --always --dirty)

build:
	go install -ldflags "-s -w -X main.Version=$(VERSION)" ./cmd/...

lint:
	golangci-lint run ./...

test:
	go test ./... -race -v

tidy:
	go mod tidy

fmt:
	gofumpt -w .

cover:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "Open coverage.html in your browser to view the report."

LINT_VERSION ?= v1.64.8

install-lint:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(LINT_VERSION)
