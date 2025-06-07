# utils

This package provides shared utility functions for file, directory, JSON, Terraform, and Kubernetes operations used throughout the project.

## Modules

- **file.go**: Secure file reading with path validation and extension whitelisting.
- **dir.go**: Directory listing utilities.
- **json.go**: JSON file loading and pretty-printing.
- **terraform.go**: Helpers for parsing and working with Terraform HCL files and model deployment metadata.
- **k8shelper.go**: Helpers for interacting with Kubernetes clusters, including node and resource queries.

## Example Usage

### Secure File Read

```go
import "yourmodule/internal/utils"

data, err := utils.SafeReadFile("config.json", "/trusted/dir", map[string]struct{}{".json": {}})
if err != nil {
    // handle error
}
```

### List Files by Extension

```go
files, err := utils.ListFiles("/some/dir", ".json")
```

### Load JSON File

```go
type Config struct { ... }
cfg, err := utils.LoadFile[Config]("config.json")
```

### Pretty Print JSON

```go
str, err := utils.PrettyJSON(cfg)
```

### Kubernetes Helper

```go
helper, err := utils.NewK8sHelper("kubeconfig.yaml", "my-context")
nodes, err := helper.ListGpuNodes(ctx)
```

### Terraform Helpers

See `terraform.go` for advanced helpers for parsing HCL and extracting model metadata.

## Conventions

- All helpers are intended for internal use and follow Go best practices for error handling and documentation.
- See individual files for detailed function and type documentation.
