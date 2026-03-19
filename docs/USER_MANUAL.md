# Toolkit — User Manual

> **Toolkit** is an interactive terminal application for DevOps & infrastructure management.
> It provides a keyboard-driven TUI (Terminal User Interface) for inspecting and operating on Kubernetes clusters, Terraform configurations, OCI resources, and large tabular datasets — all without leaving your terminal.

---

## Table of Contents

1. [Installation](#installation)
2. [Initial Setup](#initial-setup)
3. [Configuration Reference](#configuration-reference)
4. [Launching Toolkit](#launching-toolkit)
5. [Interface Overview](#interface-overview)
6. [Navigation](#navigation)
7. [Categories](#categories)
8. [Filtering & Searching](#filtering--searching)
9. [Sorting](#sorting)
10. [Detail View](#detail-view)
11. [Infrastructure Operations](#infrastructure-operations)
12. [Exporting Data](#exporting-data)
13. [Clipboard Integration](#clipboard-integration)
14. [Keyboard Reference](#keyboard-reference)
15. [Shell Completion](#shell-completion)
16. [Logging & Debugging](#logging--debugging)
17. [Subcommands](#subcommands)

---

## Installation

### Option A — Go install (latest release)

```bash
go install github.com/jingle2008/toolkit/cmd/toolkit@latest
```

### Option B — Homebrew (macOS / Linux)

```bash
brew tap jingle2008/homebrew-toolkit
brew install toolkit
```

### Option C — Build from source

```bash
git clone https://github.com/jingle2008/toolkit.git
cd toolkit
make
# binary is placed in ./bin/toolkit
```

Verify the install:

```bash
toolkit version
```

---

## Initial Setup

Run `init` to generate an example config file:

```bash
toolkit init
```

This creates `~/.config/toolkit/config.yaml` with all available options pre-populated. Open it in your editor and fill in the required fields:

```yaml
# Path to your Terraform / infrastructure repository
repo_path: "/path/to/your/repo"

# Kubernetes config (defaults to ~/.kube/config)
kubeconfig: "/path/to/.kube/config"

# Environment identifiers
env_type:   "dev"          # e.g. dev, staging, prod
env_region: "us-phoenix-1" # OCI region
env_realm:  "oc1"          # OCI realm

# Default category to open on startup
category: "tenant"

# Optional: path to extra metadata (tenants, etc.)
metadata_file: ""

# Logging
log_file:   "toolkit.log"
log_format: "console"   # console | json | slog
log_level:  ""           # debug | info | warn | error (empty = default)
debug:      false
```

> **Tip:** You can override any config value at runtime with CLI flags (see [Configuration Reference](#configuration-reference)).

---

## Configuration Reference

All flags can be set in the config file or passed directly on the command line. CLI flags take precedence.

| Config Key      | CLI Flag           | Default                              | Required | Description                                  |
|-----------------|--------------------|--------------------------------------|----------|----------------------------------------------|
| `repo_path`     | `--repo_path`      | —                                    | Yes      | Path to Terraform / config repository        |
| `kubeconfig`    | `--kubeconfig`     | `~/.kube/config`                     | No       | Path to kubeconfig file                      |
| `env_type`      | `--env_type`       | —                                    | Yes      | Environment type (`dev`, `prod`, …)          |
| `env_region`    | `--env_region`     | —                                    | Yes      | Cloud region (e.g. `us-phoenix-1`)           |
| `env_realm`     | `--env_realm`      | —                                    | Yes      | Cloud realm (e.g. `oc1`)                     |
| `category`      | `-c / --category`  | —                                    | Yes      | Initial data category to display             |
| `filter`        | `-f / --filter`    | `""`                                 | No       | Pre-applied filter on startup                |
| `metadata_file` | `--metadata_file`  | `~/.config/toolkit/metadata.yaml`    | No       | Optional extra metadata file                 |
| `config`        | `--config`         | `~/.config/toolkit/config.yaml`      | No       | Path to the config file itself               |
| `log_file`      | `--log_file`       | `toolkit.log`                        | No       | Log output path                              |
| `debug`         | `--debug`          | `false`                              | No       | Enable debug-level logging                   |
| `log_format`    | `--log_format`     | `console`                            | No       | Log format: `console`, `json`, or `slog`     |
| `log_level`     | `--log_level`      | `""`                                 | No       | Minimum log level: `debug` `info` `warn` `error` |

---

## Launching Toolkit

```bash
# Use values from config file
toolkit

# Override category and filter inline
toolkit -c gpunode -f "phoenix"

# Point to a different environment
toolkit --env_type prod --env_region us-ashburn-1

# Show all flags
toolkit --help
```

---

## Interface Overview

```
┌─────────────────────────────────────────────────────────────────────────────┐
│  Tenant  LimitDefinition  PropertyDefinition  GpuPool  GpuNode  ...         │  ← Category tabs
├─────────────────────────────────────────────────────────────────────────────┤
│  NAME              │  INTERNAL  │  STATUS    │  REGION                       │  ← Column headers
│  ──────────────────┼────────────┼────────────┼──────────────────────────── │
│  acme-corp         │  false     │  Active    │  us-phoenix-1                 │
│  ▶ beta-tenant     │  true      │  Active    │  us-ashburn-1                 │  ← Selected row
│  gamma-inc         │  false     │  Inactive  │  eu-frankfurt-1               │
├─────────────────────────────────────────────────────────────────────────────┤
│  Filter: _                                                                   │  ← Filter / status bar
│  [?] Help  [q] Quit  [/] Filter  [tab] Next  [y] Details  [e] Export CSV   │  ← Key hints
└─────────────────────────────────────────────────────────────────────────────┘
```

The interface has four zones:

| Zone | Description |
|------|-------------|
| **Category tabs** (top) | Navigate between data categories with `Tab` / `Shift+Tab` |
| **Table** (center) | Scrollable, sortable, filterable data rows |
| **Status bar** | Shows current filter text and aggregate statistics |
| **Key hints** (bottom) | Context-sensitive reminder of available keys |

---

## Navigation

### Moving around the table

| Key | Action |
|-----|--------|
| `↑` / `↓` | Move selection up / down |
| `PgUp` / `PgDn` | Jump one page |
| `Home` / `End` | Jump to first / last row |

### Switching categories

| Key | Action |
|-----|--------|
| `Tab` | Next category |
| `Shift+Tab` | Previous category |
| `:` + alias + `Enter` | Jump directly to a category by alias (Command mode) |

**Category aliases** — type `:` then one of these shortcuts:

| Alias | Category |
|-------|----------|
| `t` / `tenant` | Tenant |
| `l` / `ld` | LimitDefinition |
| `cpd` | ConsolePropertyDefinition |
| `pd` | PropertyDefinition |
| `lto` | LimitTenancyOverride |
| `cpto` | ConsolePropertyTenancyOverride |
| `pto` | PropertyTenancyOverride |
| `lro` | LimitRegionalOverride |
| `cpro` | ConsolePropertyRegionalOverride |
| `pro` | PropertyRegionalOverride |
| `bm` | BaseModel |
| `ma` | ModelArtifact |
| `e` / `env` | Environment |
| `st` | ServiceTenancy |
| `gp` | GpuPool |
| `gn` | GpuNode |
| `dac` | DedicatedAICluster |

### History navigation

Navigate back and forward through the categories you've visited:

| Key | Action |
|-----|--------|
| `[` | History back |
| `]` | History forward |

### Scoping into a context

Some categories are **parent scopes** — pressing `Enter` on a row zooms in to the child category filtered to that parent.

| Parent | Child categories available |
|--------|---------------------------|
| Tenant | LimitTenancyOverride, ConsolePropertyTenancyOverride, PropertyTenancyOverride, DedicatedAICluster |
| LimitDefinition | LimitTenancyOverride, LimitRegionalOverride |
| ConsolePropertyDefinition | ConsolePropertyTenancyOverride, ConsolePropertyRegionalOverride |
| PropertyDefinition | PropertyTenancyOverride, PropertyRegionalOverride |
| GpuPool | GpuNode |

Press `Esc` to exit the scoped context and return to the parent.

---

## Categories

Toolkit organises data into 17 categories:

### Core Infrastructure

| Category | Description |
|----------|-------------|
| **Tenant** | Tenant-level data; supports faulty tracking, internal flag, and scoping into overrides |
| **Environment** | Cloud environment configurations |
| **ServiceTenancy** | Service-to-tenancy mappings |

### Definitions (parent categories)

| Category | Description |
|----------|-------------|
| **LimitDefinition** | Service quota / limit definitions |
| **ConsolePropertyDefinition** | Console feature-flag definitions |
| **PropertyDefinition** | Generic property definitions |

### Tenancy Overrides (child of Definitions)

| Category | Description |
|----------|-------------|
| **LimitTenancyOverride** | Per-tenant quota overrides |
| **ConsolePropertyTenancyOverride** | Per-tenant console property overrides |
| **PropertyTenancyOverride** | Per-tenant property overrides |

### Regional Overrides

| Category | Description |
|----------|-------------|
| **LimitRegionalOverride** | Region-specific limit overrides |
| **ConsolePropertyRegionalOverride** | Region-specific console property overrides |
| **PropertyRegionalOverride** | Region-specific property overrides |

### AI / GPU Infrastructure

| Category | Description |
|----------|-------------|
| **BaseModel** | AI model definitions; supports faulty tracking |
| **ModelArtifact** | Model artifact versions |
| **GpuPool** | OCI GPU instance pools; supports scaling |
| **GpuNode** | Individual Kubernetes GPU compute nodes |
| **DedicatedAICluster** | OCI Dedicated AI Clusters |

---

## Filtering & Searching

### Enter filter mode

Press `/` to open the filter input at the bottom of the screen. Start typing and the table updates in real time (100 ms debounce).

```
Filter: phoenix█
```

- Matching is **case-insensitive** and **substring-based**.
- Press `Esc` to clear the filter and exit filter mode.

### Paste a filter from clipboard

If you have a filter expression already copied:

```
p    → paste clipboard contents directly as the filter
```

This is useful for pasting long IDs or complex strings.

### Pre-apply a filter on startup

```bash
toolkit -c gpunode -f "us-phoenix"
```

---

## Sorting

### Sort by name (always available in list view)

```
Shift+N    → sort by Name (ascending / descending toggle)
```

### Category-specific sort keys

Each category exposes additional sort columns:

| Category | Key | Sorts by |
|----------|-----|----------|
| BaseModel | `Shift+S` | Size |
| BaseModel | `Shift+C` | Context |
| Tenant | `Shift+I` | Internal |
| Environment | `Shift+T` | Type |
| ServiceTenancy | `Shift+T` | Type |
| ConsolePropertyDefinition | `Shift+V` | Value |
| PropertyDefinition | `Shift+V` | Value |
| GpuPool | `Shift+S` | Size |
| GpuNode | `Shift+F` | Free |
| GpuNode | `Shift+T` | Type |
| GpuNode | `Shift+A` | Age |
| DedicatedAICluster | `Shift+T` | Tenant |
| DedicatedAICluster | `Shift+I` | Internal |
| DedicatedAICluster | `Shift+U` | Usage |
| DedicatedAICluster | `Shift+S` | Size |
| DedicatedAICluster | `Shift+A` | Age |
| LimitTenancyOverride | `Shift+T` | Tenant |
| LimitTenancyOverride | `Shift+R` | Regions |
| ConsolePropertyTenancyOverride | `Shift+T` | Tenant |
| ConsolePropertyTenancyOverride | `Shift+R` | Regions |
| ConsolePropertyTenancyOverride | `Shift+V` | Value |
| PropertyTenancyOverride | `Shift+T` | Tenant |
| PropertyTenancyOverride | `Shift+R` | Regions |
| PropertyTenancyOverride | `Shift+V` | Value |
| LimitRegionalOverride | `Shift+R` | Regions |
| PropertyRegionalOverride | `Shift+R` | Regions |
| PropertyRegionalOverride | `Shift+V` | Value |
| ConsolePropertyRegionalOverride | `Shift+R` | Regions |
| ConsolePropertyRegionalOverride | `Shift+V` | Value |

Pressing the same key again **reverses** the sort direction.

### Toggle alias view

```
Ctrl+A    → toggle between full category names and short aliases in the UI
```

---

## Detail View

Press `y` on any selected row to open the **Detail View**, which shows the full JSON object for that item.

```
┌─────────────────────────────────────────────────┐
│  DETAIL — acme-corp (Tenant)                    │
├─────────────────────────────────────────────────┤
│  {                                              │
│    "name":     "acme-corp",                     │
│    "internal": false,                           │
│    "status":   "Active",                        │
│    "region":   "us-phoenix-1",                  │
│    ...                                          │
│  }                                              │
├─────────────────────────────────────────────────┤
│  [y/esc] Back  [c] Copy Name  [o] Copy Object  │
└─────────────────────────────────────────────────┘
```

| Key | Action |
|-----|--------|
| `y` or `Esc` | Return to list view |
| `↑` / `↓` | Scroll the JSON content |
| `c` | Copy the item's name to clipboard |
| `o` | Copy the **entire JSON object** to clipboard |

---

## Infrastructure Operations

Certain categories expose live infrastructure operations. These require Kubernetes or OCI credentials to be configured.

> **Warning:** Operations like Drain, Reboot, and Delete are **irreversible**. Toolkit guards against duplicate in-flight requests automatically.

### GPU Nodes (`GpuNode`)

Select a node in the `GpuNode` category:

| Key | Operation | Description |
|-----|-----------|-------------|
| `Shift+C` | Toggle Cordon | Mark node unschedulable / schedulable |
| `Shift+D` | Drain | Evict all pods from the node |
| `Shift+R` | Reboot | Soft-reset (reboot) the node |
| `Ctrl+X` | Delete | Terminate the node instance |
| `r` | Refresh | Reload GPU node data |
| `Ctrl+Z` | Toggle Faulty | Show/hide nodes flagged as faulty |

### GPU Pools (`GpuPool`)

| Key | Operation | Description |
|-----|-----------|-------------|
| `Shift+U` | Scale Up | Request an increase in pool capacity |
| `r` | Refresh | Reload pool data |
| `Ctrl+Z` | Toggle Faulty | Show/hide pools with faulty status |

### Dedicated AI Clusters (`DedicatedAICluster`)

| Key | Operation | Description |
|-----|-----------|-------------|
| `Ctrl+X` | Delete | Delete the selected Dedicated AI Cluster |
| `r` | Refresh | Reload cluster data |
| `Ctrl+Z` | Toggle Faulty | Show/hide faulty clusters |

### Faulty item tracking

Several categories support a **faulty toggle** (`Ctrl+Z`). When enabled, only items that are in a faulty state are shown. The status bar displays counts like `Faulty: 2, Healthy: 10`.

---

## Exporting Data

Press `e` in any list view to open the **Export CSV** dialog.

1. A file picker appears — navigate to your desired output directory.
2. Select or type a filename ending in `.csv`.
3. Press `Enter` to confirm.

The exported CSV reflects the **current filter and sort state** — what you see is what you get.

---

## Clipboard Integration

| Key | Available in | Copies |
|-----|-------------|--------|
| `c` | List & Detail view | Item name or ID |
| `t` | Tenant, DedicatedAICluster, tenancy override categories | Tenant ID |
| `o` | Detail view only | Full JSON object |
| `p` | List view | *Pastes* clipboard content as a filter |

---

## Keyboard Reference

### Always available (Global)

| Key | Action |
|-----|--------|
| `q` / `Ctrl+C` | Quit |
| `?` / `h` | Toggle help overlay |
| `Esc` | Back / Clear filter |
| `y` | Toggle detail view |
| `c` | Copy item name / ID |

### List view

| Key | Action |
|-----|--------|
| `Tab` | Next category |
| `Shift+Tab` | Previous category |
| `/` | Enter filter mode |
| `:` | Enter command mode (alias jump) |
| `p` | Paste clipboard as filter |
| `Enter` | View item / scope into context |
| `[` | History back |
| `]` | History forward |
| `e` | Export table as CSV |
| `Ctrl+A` | Toggle alias view |
| `Shift+N` | Sort by name |

### Detail view

| Key | Action |
|-----|--------|
| `y` / `Esc` | Return to list |
| `↑` / `↓` | Scroll content |
| `c` | Copy item name |
| `o` | Copy full JSON object |

### In-app help

Press `?` or `h` at any time to display the full keybinding help overlay. The help is **context-sensitive** — it only shows keys relevant to your current category and view mode.

---

## Shell Completion

Generate and install completion scripts for your shell:

```bash
# Bash
toolkit completion bash > /etc/bash_completion.d/toolkit

# Zsh
toolkit completion zsh > "${fpath[1]}/_toolkit"

# Fish
toolkit completion fish > ~/.config/fish/completions/toolkit.fish

# PowerShell
toolkit completion powershell > toolkit.ps1
```

After sourcing the file / restarting your shell, pressing `Tab` while typing `toolkit` commands will auto-complete flags and subcommands.

---

## Logging & Debugging

Toolkit writes structured logs to a file (default: `toolkit.log` in your working directory).

```bash
# Enable verbose debug logging
toolkit --debug

# Write logs to a custom path
toolkit --log_file /tmp/tk-debug.log

# Use JSON format for log shipping / parsing
toolkit --log_format json

# Set minimum log level
toolkit --log_level warn
```

Supported log levels (from most to least verbose): `debug` → `info` → `warn` → `error`.

Supported log formats: `console` (human-readable), `json` (structured), `slog` (Go slog).

---

## Subcommands

| Subcommand | Description |
|------------|-------------|
| `toolkit init` | Scaffold `~/.config/toolkit/config.yaml` with example values |
| `toolkit completion <shell>` | Print shell completion script for `bash`, `zsh`, `fish`, or `powershell` |
| `toolkit version [--check]` | Print installed version; `--check` fetches the latest release from GitHub and compares |

---

## Tips & Tricks

- **Start with a filter** — pass `-f <term>` on the command line to pre-filter noisy categories like `GpuNode` or `Tenant`.
- **Scope then drill** — select a `LimitDefinition` and press `Enter` to instantly view all tenancy overrides for that specific limit.
- **Use command mode for fast navigation** — press `:dac Enter` to jump straight to DedicatedAICluster from any category.
- **Copy-then-filter workflow** — copy a tenant ID with `t`, switch to another category, then paste it as a filter with `p` to quickly cross-reference data.
- **Export after filtering** — apply a filter first, then press `e` to export only the rows you care about.
- **History saves context** — use `[` and `]` to bounce between parent and child category views without re-typing.

---

*For bugs and feature requests, open an issue at [github.com/jingle2008/toolkit](https://github.com/jingle2008/toolkit/issues).*
