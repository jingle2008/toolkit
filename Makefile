.PHONY: build lint test tidy fmt cover

build:
	go build ./cmd/toolkit

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
