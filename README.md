# Toolkit

Toolkit is a modular command-line utility written in Go, designed to provide a collection of tools and utilities for various development and automation tasks. The project is organized for extensibility and maintainability, following Go best practices.

## Features

- **Modular CLI**: Easily extendable command-line interface.
- **Category-based Utilities**: Organized by categories for clear separation of concerns.
- **Configurable**: Uses Go modules and a Makefile for streamlined building and management.
- **Test Coverage**: Includes unit tests for core logic.

## Project Structure

```
.
├── cmd/                  # Entry points for CLI commands
│   └── toolkit/          # Main CLI application
│       └── main.go
├── internal/             # Internal application logic
│   └── app/
│       └── toolkit/
│           ├── category.go
│           ├── constants.go
│           ├── context.go
│           ├── key_map.go
│           ├── model.go
│           ├── model_test.go
│           ├── table_utils.go
│           └── table_utils_test.go
├── Makefile              # Build and management commands
├── go.mod                # Go module definition
├── go.sum                # Go module checksums
└── .gitignore
```

## Getting Started

### Prerequisites

- Go 1.18 or later

### Build

```sh
make
```
or
```sh
go build -o bin/toolkit ./cmd/toolkit
```

### Run

```sh
./bin/toolkit
```
or, if built with Go:
```sh
go run ./cmd/toolkit
```

## Testing

Run all tests with:
```sh
go test ./...
```

## Contributing

Contributions are welcome! Please open issues or submit pull requests for new features, bug fixes, or improvements.

## License

This project is licensed under the MIT License.
