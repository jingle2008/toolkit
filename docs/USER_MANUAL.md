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
12. [Live Updates](#live-updates)
13. [Exporting Data](#exporting-data)
14. [Clipboard Integration](#clipboard-integration)
15. [Keyboard Reference](#keyboard-reference)
16. [Shell Completion](#shell-completion)
17. [Logging & Debugging](#logging--debugging)
18. [Subcommands](#subcommands)

---

## Installation

### Option A — Go install (latest release)

```bash
go install github.com/jingle2008/toolkit/cmd/toolkit@latest
```

### Option B — Homebrew (macOS / Linux)

```bash
brew install jingle2008/tap/toolkit
```

Migrating from the older `jingle2008/homebrew-toolkit` tap (order matters — `brew untap` refuses while the formula is still installed):

```bash
brew uninstall toolkit
brew untap jingle2008/toolkit              # canonical short name; the homebrew- prefix is implied
brew install jingle2008/tap/toolkit
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
repo-path: "/path/to/your/repo"

# Kubernetes config (defaults to ~/.kube/config)
kubeconfig: "/path/to/.kube/config"

# Environment identifiers
env-type:   "dev"          # e.g. dev, staging, prod
env-region: "us-phoenix-1" # OCI region
env-realm:  "oc1"          # OCI realm

# Default category to open on startup
category: "tenant"

# Optional: path to extra metadata (tenants, etc.)
metadata-file: ""

# Logging
log-file:   "toolkit.log"
log-format: "console"   # console | json | slog
log-level:  ""           # debug | info | warn | error (empty = default)
debug:      false
```

> **Tip:** You can override any config value at runtime with CLI flags (see [Configuration Reference](#configuration-reference)).

---

## Configuration Reference

All flags can be set in the config file or passed directly on the command line. CLI flags take precedence.

| Config Key      | CLI Flag           | Default                              | Required | Description                                  |
|-----------------|--------------------|--------------------------------------|----------|----------------------------------------------|
| `repo-path`     | `--repo-path`      | —                                    | Yes      | Path to Terraform / config repository        |
| `kubeconfig`    | `--kubeconfig`     | `~/.kube/config`                     | No       | Path to kubeconfig file                      |
| `env-type`      | `-t / --env-type`  | —                                    | Yes      | Environment type (`dev`, `prod`, …); `ppe` aliases `preprod` |
| `env-region`    | `-r / --env-region` | —                                   | Yes      | Cloud region — `us-phoenix-1`, or the 3-letter code `phx` |
| `env-realm`     | `-m / --env-realm` | —                                    | Yes      | Cloud realm (e.g. `oc1`)                     |
| `category`      | `-c / --category`  | —                                    | Yes      | Initial data category to display             |
| `filter`        | `-f / --filter`    | `""`                                 | No       | Pre-applied filter on startup                |
| `metadata-file` | `--metadata-file`  | `~/.config/toolkit/metadata.yaml`    | No       | Optional extra metadata file                 |
| `config`        | `--config`         | `~/.config/toolkit/config.yaml`      | No       | Path to the config file itself               |
| `log-file`      | `--log-file`       | `toolkit.log`                        | No       | Log output path                              |
| `debug`         | `-d / --debug`     | `false`                              | No       | Enable debug-level logging                   |
| `log-format`    | `--log-format`     | `console`                            | No       | Log format: `console`, `json`, or `slog`     |
| `log-level`     | `--log-level`      | `""`                                 | No       | Minimum log level: `debug` `info` `warn` `error` |

---

## Launching Toolkit

```bash
# Use values from config file
toolkit

# Override category and filter inline
toolkit -c gpunode -f "phoenix"

# Point to a different environment
toolkit --env-type prod --env-region us-ashburn-1

# Show all flags
toolkit --help
```

---

## Interface Overview

```
┌─────────────────────────────────────────────────────────────────────────────┐
│  Tenant  LimitDefinition  PropertyDefinition  GPUPool  GPUNode  ...         │  ← Category tabs
├─────────────────────────────────────────────────────────────────────────────┤
│  NAME           │  OCIDS                            │  INTERNAL ↕ │  NOTE   │  ← Column headers (↕ = sortable)
│  ───────────────┼───────────────────────────────────┼─────────────┼──────── │
│  acme-corp      │  ocid1.tenancy.oc1..aaa…xyz       │  false      │         │
│  ▶ beta-tenant  │  ocid1.tenancy.oc1..bbb…uvw,…(+1) │  true       │  pilot  │  ← Selected row
│  gamma-inc      │  ocid1.tenancy.oc1..ccc…rst       │  false      │         │
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
| **Status bar** | Shows current filter text, aggregate statistics, and a `● LIVE` indicator when the current category is updating automatically (see [Live Updates](#live-updates)) |
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
| `t` | Tenant |
| `ld` | LimitDefinition |
| `cpd` | ConsolePropertyDefinition |
| `pd` | PropertyDefinition |
| `lto` | LimitTenancyOverride |
| `cpto` | ConsolePropertyTenancyOverride |
| `pto` | PropertyTenancyOverride |
| `lro` | LimitRegionalOverride |
| `cpro` | ConsolePropertyRegionalOverride |
| `pro` | PropertyRegionalOverride |
| `bm` | BaseModel |
| `im` | ImportedModel |
| `ma` | ModelArtifact |
| `e` | Environment |
| `st` | ServiceTenancy |
| `gp` / `gpup` | GPUPool |
| `gn` / `gpun` | GPUNode |
| `gw` / `gpuw` | GPUWorkload |
| `dac` / `daic` | DedicatedAICluster |
| `is` | InferenceService |
| `sr` | ServingRuntime |
| `a` | Alias |

Every category additionally accepts its **full name, lowercased** — `tenant`,
`limitdefinition`, `gpuworkload`, `dedicatedaicluster`, and so on. Run
`toolkit get alias` for the authoritative list, or enable shell completion.

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
| Tenant | LimitTenancyOverride, ConsolePropertyTenancyOverride, PropertyTenancyOverride, DedicatedAICluster, ImportedModel, GPUWorkload and InferenceService (the tenant's pods and endpoints) |
| LimitDefinition | LimitTenancyOverride, LimitRegionalOverride |
| ConsolePropertyDefinition | ConsolePropertyTenancyOverride, ConsolePropertyRegionalOverride |
| PropertyDefinition | PropertyTenancyOverride, PropertyRegionalOverride |
| GPUPool | GPUNode |
| GPUNode | GPUWorkload |
| DedicatedAICluster | InferenceService, GPUWorkload (the pods in the DAC's namespace) |
| BaseModel | InferenceService (endpoints serving the model) |
| ImportedModel | InferenceService (endpoints serving the model) |
| ServingRuntime | InferenceService (endpoints naming the runtime), GPUWorkload (pods running it) |

`Enter` opens the first child category. The scope stays active when you
switch categories, so switching to another child (e.g. `:gw` after drilling
into a DAC) lists that child filtered to the same parent.

Press `Esc` to exit the scoped context and return to the parent. On a parent
scope, `Enter` drills in rather than opening details; use `y` to view a row's
details instead.

`o` goes to a row's related rows, along every relation: it lists what the
selected row relates to — an InferenceService's tenant, model, DAC and
runtime, a GPUWorkload's tenant, node, DAC and runtime — and jumps to the one
you pick (`↑/↓`, `Enter`; `Esc` cancels). With a single target it jumps
straight there. The picker starts on the row's parent: the scope you drilled
in from when one is active, otherwise the structural parent (a GPUWorkload's
node, an InferenceService's DAC), so going up is `o`, `Enter`. A target that
is known not to exist is left out (e.g. a shared-catalog endpoint's model is
offered as a BaseModel, not an ImportedModel).

`g` / `Home` moves to the first row and `End` to the last (`G` is Sort GPUs
on the categories that have it).

---

## Categories

Toolkit organises data into 22 categories:

### Core Infrastructure

| Category | Description |
|----------|-------------|
| **Tenant** | One row per tenancy OCID found in the tenancy overrides or the tenant metadata file, with its internal flag and note; scopes into the tenant's overrides and resources. The name comes from the metadata file only (override directory names are ignored); a tenancy without a metadata name is named by its OCID. Names must be unique: a metadata file that gives two tenancies the same name fails to load. **Status** is the tenancy's account state (`ACTIVE`, `DELETING`, `TERMINATED`, `ALWAYS_FREE_ONLY`) as last recorded by `toolkit tenant sync`, blank when unknown; a `DELETING` or `TERMINATED` tenant is reported as faulty |
| **Environment** | Cloud environment configurations |
| **ServiceTenancy** | Service-to-tenancy mappings |

### Definitions (parent categories)

| Category | Description |
|----------|-------------|
| **LimitDefinition** | Service quota / limit definitions, including the splat throttle definitions (`splat_limit_definitions.json`; filter by operation, e.g. `/ListSemanticStores`) |
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
| **BaseModel** | AI model definitions; supports faulty tracking. `Enter` lists the inference services serving the model (`y` opens details) |
| **ImportedModel** | Tenant-imported models (namespaced BaseModel CRs + ClusterBaseModel CRs with a `tenancy-id` label). A model copied into several namespaces — its `default` library copy plus copies in DAC namespaces — is one row, with every copy listed under **Namespace**. `m` opens the metrics of the DAC serving the model; when several do, it asks which. `Enter` lists the inference services serving the model (`y` opens details) |
| **ModelArtifact** | Model artifact versions |
| **GPUPool** | OCI GPU instance pools; supports scaling |
| **GPUNode** | Individual Kubernetes GPU compute nodes |
| **GPUWorkload** | GPU-consuming Kubernetes pods (anything requesting `nvidia.com/gpu`), scoped by GPUNode, or by DedicatedAICluster (the pods in its namespace). Pods serving a legacy `ome.oracle.com/v1alpha1` endpoint match `/legacy`; their runtime is a legacy one, so they aren't related to a ServingRuntime |
| **DedicatedAICluster** | OCI Dedicated AI Clusters; press `Enter` to see the inference services running on one |
| **InferenceService** | OME `InferenceService` CRs — serving endpoints, scoped by DedicatedAICluster (grouped by namespace). **Type** is `dedicated` (namespace is the DAC ID) or `on-demand` (shared endpoint; namespace is the model name, no tenant). **Replicas** is the engine's `min/max`. A row whose `Ready` condition isn't `True` is reported as faulty. Legacy `ome.oracle.com/v1alpha1` endpoints (on legacy DACs) are listed too and match `/legacy` |
| **ServingRuntime** | OME `ClusterServingRuntime` CRs — the cluster-scoped templates defining how a class of models is served (hardware constraints, container image, resource caps); a `disabled` runtime is reported as faulty. `Enter` lists the inference services naming the runtime; with that scope active, `:gw` lists the pods running it, including endpoints whose runtime OME selected at deploy time (`y` opens details) |

### Meta

| Category | Description |
|----------|-------------|
| **Alias** | Index of every category and its aliases; reachable with `Ctrl+A` or `:alias` |

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

Each category exposes additional sort columns. In the TUI, the column
headers that participate in sorting are marked with a `↕` indicator so
you can see at a glance which columns the keys below will reach.

| Category | Key | Sorts by |
|----------|-----|----------|
| BaseModel | `Shift+S` | Size |
| BaseModel | `Shift+C` | Context |
| Tenant | `Shift+I` | Internal |
| Tenant | `Shift+S` | Status |
| Environment | `Shift+T` | Type |
| ServiceTenancy | `Shift+T` | Type |
| ConsolePropertyDefinition | `Shift+V` | Value |
| PropertyDefinition | `Shift+V` | Value |
| GPUPool | `Shift+S` | Size |
| GPUPool | `Shift+G` | GPUs |
| GPUNode | `Shift+F` | Free |
| GPUNode | `Shift+T` | Type |
| GPUNode | `Shift+A` | Age |
| GPUWorkload | `Shift+T` | Tenant |
| GPUWorkload | `Shift+G` | GPUs |
| GPUWorkload | `Shift+A` | Age |
| DedicatedAICluster | `Shift+T` | Tenant |
| DedicatedAICluster | `Shift+I` | Internal |
| DedicatedAICluster | `Shift+U` | Usage |
| DedicatedAICluster | `Shift+S` | Size |
| DedicatedAICluster | `Shift+A` | Age |
| InferenceService | `Shift+T` | Tenant |
| InferenceService | `Shift+A` | Age |
| ImportedModel | `Shift+T` | Tenant |
| ImportedModel | `Shift+S` | Size |
| ImportedModel | `Shift+C` | Context |
| ServingRuntime | `Shift+G` | GPUs |
| ImportedModel | `Shift+V` | Vendor |
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

> **Warning:** Drain and Restart disrupt running workloads, and Delete is **irreversible**. Toolkit guards against duplicate in-flight requests automatically.

### GPU Nodes (`GPUNode`)

Select a node in the `GPUNode` category:

| Key | Operation | Description |
|-----|-----------|-------------|
| `Shift+C` | Cordon / Uncordon | Cordon a schedulable node, uncordon a cordoned one (the modal says which) |
| `Shift+D` | Drain | Evict all pods from the node |
| `Shift+R` | Restart | Soft-reset (restart) the node's instance |
| `Ctrl+X` | Delete | Terminate the node instance |
| `r` | Refresh | Reload GPU node data |
| `Ctrl+Z` | Toggle Faulty | Show/hide nodes flagged as faulty |

### GPU Pools (`GPUPool`)

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

### Inference Services (`InferenceService`)

| Key | Operation | Description |
|-----|-----------|-------------|
| `Shift+R` | Restart | Rolling-restart the selected endpoint (bumps `restartedAt`) |
| `Ctrl+X` | Delete | Delete the selected endpoint if it is orphaned (its DAC no longer exists) |

`Ctrl+X` checks the cluster before asking. An endpoint that isn't orphaned is refused with a toast saying why: shared endpoints and endpoints whose DAC still exists in any DAC API belong to the control plane (delete the DAC instead). For an orphan, the confirm modal names the action. **Delete** issues a normal delete. **Force-delete** applies when a deletion has been pending for over 10 minutes: the OME controller can't finalize an endpoint whose DAC or model is gone, so the deletion hangs on its `inferenceservice.finalizers` finalizer and the namespace stays `Terminating`; force-delete removes the finalizer. `Ctrl+Z` lists orphans, since they name a DAC that doesn't exist.

### Refreshing cluster-derived categories

The `r` key reloads the current list from the live cluster. In addition
to GPUNode / GPUPool / DedicatedAICluster above, it is also bound on the
read-only cluster-derived categories:

| Category | Key | Operation |
|----------|-----|-----------|
| BaseModel | `r` | Reload base models from the cluster |
| ImportedModel | `r` | Reload tenant-imported models from the cluster |

Most categories also update on their own — see [Live Updates](#live-updates).
`r` remains useful to force an immediate reload and to recover a watch that
has dropped.

### Faulty item tracking

Several categories support a **faulty toggle** (`Ctrl+Z`). When enabled, only faulty items are shown. An item is faulty when its own state is, as each category describes above, or when it names a parent row that doesn't exist. For example, an InferenceService whose DAC was deleted is faulty even if it reports `Ready`. These are the rows `toolkit doctor --relations` reports as FAIL; tenancies missing from the metadata (its WARN) don't count. A reference is checked only once its parent category is loaded, since a missing parent may just be unloaded. A node whose pool is missing is faulty even when some GPU pool sources failed to load, so the pool may only be unloaded: a broken source needs attention too, and `toolkit doctor --relations` names it.

---

## Live Updates

Toolkit keeps the current view fresh automatically. When the category on
screen is being watched, the status bar shows a `● LIVE` indicator. Updates
are **debounced** (~5 s): a burst of changes collapses into a single reload,
and reloads **preserve your active filter and selected row**.

There are two independent watch mechanisms:

| Source | Categories | Triggers on |
|--------|-----------|-------------|
| **Cluster watch** (Kubernetes) | BaseModel, ImportedModel, GPUNode, GPUWorkload, DedicatedAICluster, InferenceService, ServingRuntime | Changes to the watched cluster resources (requires a working kubeconfig) |
| **Working-tree watch** (local files) | Tenants, Definitions, Overrides, Environments, Service Tenancies, Model Artifacts, GPU Pools, Aliases | Saving any file under the repo path (`.git` and dotfiles are ignored) |

So editing a config file in your repo, or a change landing in the cluster,
shows up on screen within a few seconds without pressing a key.

**Notes:**

- **No credentials / unreadable repo:** if a watch can't be established it
  simply doesn't run — the data still loads once and `r` reloads on demand.
  No `● LIVE` indicator is shown.
- **Switching environments** keeps live updates working: the working-tree
  watch continues and the cluster watch re-establishes for the new
  environment as you navigate.
- **Recovering a dropped watch:** the working-tree watch has no
  auto-reconnect. If it drops (rare — e.g. a filesystem error), press `r` to
  re-establish it.
- **Not watched:** the optional external metadata file
  (`metadata-file`, default `~/.config/toolkit/metadata.yaml`) lives outside
  the repo and is read at startup. Tenant metadata you edit in-app updates
  immediately; external edits to that file require a restart.

---

## Exporting Data

Press `e` in any list view to open the **Export CSV** dialog.

1. A file picker appears — navigate to your desired output directory.
2. Select or type a filename ending in `.csv`.
3. Press `Enter` to confirm.

The exported CSV reflects the **current filter** and the faulty-toggle state. Two
things deliberately differ from what's on screen:

- **The interactive sort is not preserved.** Rows are written in the dataset's
  natural order, not the order `Shift+N` / `Shift+T` / … put them in.
- **OCID columns are expanded.** Where the table shows a shortened OCID suffix
  (the Name and Tenant columns on DedicatedAICluster / ImportedModel, and Tenant
  on GPUWorkload / InferenceService), the CSV carries the fully-qualified OCID so the file is ready
  for downstream OCI tooling.

`toolkit get <category> -o csv` produces the same bytes for the same filter.

---

## Clipboard Integration

| Key | Available in | Copies |
|-----|-------------|--------|
| `c` | List & Detail view | Item name or ID (on ImportedModel, the full OCID) |
| `t` | Tenant, DedicatedAICluster, ImportedModel, tenancy override categories | Tenant ID |
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
toolkit --log-file /tmp/tk-debug.log

# Use JSON format for log shipping / parsing
toolkit --log-format json

# Set minimum log level
toolkit --log-level warn
```

Supported log levels (from most to least verbose): `debug` → `info` → `warn` → `error`.

Supported log formats: `console` (human-readable), `json` (structured), `slog` (Go slog).

---

## Subcommands

| Subcommand | Description |
|------------|-------------|
| `toolkit init` | Scaffold `~/.config/toolkit/config.yaml` with example values |
| `toolkit completion <shell>` | Print shell completion script for `bash`, `zsh`, `fish`, or `powershell` |
| `toolkit version [--check-updates]` | Print installed version; `--check-updates` fetches the latest release from GitHub and compares |
| `toolkit tenant sync [--write]` | Fill in oc1 tenant names, internal flags and status in the metadata file from the account admin service (needs `oci-ops`); see below |

### Syncing tenant names and status (`toolkit tenant sync`)

`toolkit tenant sync` fills in the tenant metadata file from the account
admin service, through the `oci-ops` CLI (which must be on `PATH`). It
covers every oc1 tenancy in the repo's tenancy overrides plus the oc1
entries already in the metadata file. Other realms aren't supported.

```bash
toolkit tenant sync              # dry run: prints what would change
toolkit tenant sync --write      # backs up the metadata file, then saves it
```

- **Status** (`ACTIVE`, `DELETING`, `TERMINATED`, `ALWAYS_FREE_ONLY`) is
  refreshed on every run.
- **Name** and **internal flag** are only filled in when an entry has none.
  Your values are never overwritten: where the service disagrees, or its name
  is already used by another tenancy, the report lists it and leaves yours.
- A failed lookup leaves the entry as it was. An expired `oci-ops` session
  stops the run; re-authenticate (run any `oci-ops` command and accept the
  prompt) and rerun.
- `--cache-dir <dir>` saves each response and reuses it on the next run, so
  an interrupted sync resumes without repeating lookups. Cached statuses are
  as old as the files: use a fresh directory for current ones.
- The repo and metadata file are the usual `repo-path` and `metadata-file`
  settings (config file, `--repo-path` / `--metadata-file`, or `TOOLKIT_*`
  environment variables).

Every toolkit process writes the metadata file under a lock and merges into
it as it is at that moment (a `<metadata-file>.lock` file sits next to it). So
`--write` keeps tenant edits made while it ran, and a running TUI or MCP server
keeps the synced entries when it next saves a tenant. A running TUI or MCP
server shows the synced names and statuses only after a restart.

---

## Tips & Tricks

- **Start with a filter** — pass `-f <term>` on the command line to pre-filter noisy categories like `GPUNode` or `Tenant`.
- **Scope then drill** — select a `LimitDefinition` and press `Enter` to instantly view all tenancy overrides for that specific limit.
- **Use command mode for fast navigation** — press `:dac Enter` to jump straight to DedicatedAICluster from any category.
- **Copy-then-filter workflow** — copy a tenant ID with `t`, switch to another category, then paste it as a filter with `p` to quickly cross-reference data.
- **Export after filtering** — apply a filter first, then press `e` to export only the rows you care about.
- **History saves context** — use `[` and `]` to bounce between parent and child category views without re-typing.

---

*For bugs and feature requests, open an issue at [github.com/jingle2008/toolkit](https://github.com/jingle2008/toolkit/issues).*
