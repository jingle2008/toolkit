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
 oc1 ›  PROD  › us-ashburn-1 · dp-prod-iad · k8s v1.31.4   1 dp-prod-ord   ● LIVE  v1.2.3   ← Env bar
 DedicatedAICluster › acme   ⟨/gpu⟩  ⟨sort: age ▼⟩                  Active: 2 • Failed: 1   ← Breadcrumb
╭──────────────────────────────────────────────────────────────────────────────────────╮
│ Name ↑         Tenant       Internal  Usage  Model  Size  Age  Status                 │   ← Column headers (↑ = sort column)
│──────────────────────────────────────────────────────────────────────────────────────│
│ dac-alpha      acme         false     75%    —      4     12d  Active                 │   ← Selected row
│ dac-beta       acme         false     100%   —      2     3d   Failed                 │
╰──────────────────────────────────────────────────────────────── 1 of 2 (40 total) ─╯   ← Position
 o go to related  t copy tenant ID  m open metrics  s sort  / filter  : command palette  ? more   ← Footer
```

| Zone | Description |
|------|-------------|
| **Env bar** (top) | Where the data comes from: realm › type › region · kube context · k8s version. In production the type is a red `PROD` badge, so it can't be mistaken for dev or preprod. Other quick-switch slots (`1 dp-prod-ord`) appear when they fit. On the right: the action running, if any — the newest one's latest step, how many run, and its environment when you've switched away (`⣾ 2 running · drain 10.0.0.11 (dp-prod-ord): evicted pod=ns/p1`); a spinner while loading; `● LIVE` while the category updates automatically (see [Live Updates](#live-updates)); and the toolkit version. While an action runs, its row's Status says so (`Draining`, `Deleting`, `Terminating`, `Restarting`, `Scaling`), and every action ends with a notification: one that fades on success, one that waits for `Esc` on failure |
| **Breadcrumb** | The category, and the scope it is narrowed to (`› acme`). While you type, it holds the filter input. Otherwise it shows chips only when they're active: `⟨/filter⟩`, `⟨sort: …⟩`, `⟨faulty⟩`, `⟨◀ 2/5 ▶⟩` (position in history, with an arrow for each way `[` and `]` can go), `s…` (a pending sort key). On the right: what the rows add up to (`Active: 2 • Failed: 1`), which gives way on a narrow terminal |
| **Panel** (center) | The table, or a row's details. Its bottom edge says where you are: the cursor among the rows (`1 of 2`), with the total when a filter or faulty-only mode hides some (`(40 total)`), or in the detail view the layout and scroll position. Its title says only what isn't shown elsewhere: the row in the detail view, which columns are hidden in the list. Columns are as wide as their header or their widest value, and share the rest of the width. On a narrow terminal long values (names, OCIDs) shorten first, short ones keep their width, and Status is never cut; if that still doesn't fit, columns drop from the right (never the first or Status) and the title names them (`Age hidden`), or counts them when the names don't fit. Long values are shortened in the middle (`ocid1.tenancy.oc1…eectrta`), so the distinguishing end stays visible, and resizing re-shortens them to the new width. An empty cell shows `—`. An empty table says why: still loading, the load failed (and `r` to retry, where the category has it), the filter or faulty-only mode hides every row (and how to undo it), or there are none in this context. Status cells are colored by meaning: red for failed or deleting, amber for pending or transitional, green for healthy. On terminals at least 160 columns wide, `v` adds a [preview pane](#preview-pane) |
| **Footer** (bottom) | The category's most useful actions, then `/ filter`, `: command palette` and `? more`. On a narrow terminal the actions give way first, so filtering and the palette stay in reach. An empty list shows only the actions that don't need a row (`^z faulty only`, `r refresh`) |

Colors follow the terminal. The neutrals — body text, dim text, borders and the selected row's tint — are blended from the terminal's own text and background colors when it reports them (most do; until it answers, or if it doesn't, a fixed dark or light palette is used), so they match your theme instead of sitting on it as grey. Every text color keeps at least 4.5:1 contrast against the background and against the selected row; on a low-contrast theme the selection tint gets quieter rather than the text unreadable. Each color has one meaning: orange is a key you can press, the periwinkle accent is focus (the border of the panel you're in, modal borders, cursors, the `▌` beside the selected row), green, amber and red are status, and the only filled badge is the red `PROD`. The selected row is tinted toward the accent, as strongly as keeps its status readable. Numbers (counts, sizes, ages, percentages) are right-aligned so their digits line up, and OCIDs are dimmed so the data around them leads. With `NO_COLOR` set (or a terminal without color), the selected row and the focused button are drawn in reverse video.

Modals (help, the command palette, confirmations, pickers, export, edit tenant) open over the dimmed view, without its footer: the modal's own last line lists its keys. One nearly as tall as the terminal opens on a blank screen instead, and on a narrow terminal help shortens its lines to fit. Notifications appear in the bottom-right corner. Info and warnings fade after 8 seconds; errors stay until you press `Esc` in the list or details view, and the full message is in the log (`` ` ``). An error names the key that retries or works around it when there is one (`D retry (resumes)`, `^x retry`, `e pick another folder`). A refusal by a safety check (not an orphan, nothing to show) is a warning, not an error.

---

## Navigation

### Moving around the table

| Key | Action |
|-----|--------|
| `↑` / `↓` | Move selection up / down |
| `PgUp` / `PgDn` | Jump one page |
| `g` / `Home` | Jump to the first row |
| `G` / `End` | Jump to the last row |

### Switching categories

| Key | Action |
|-----|--------|
| `Tab` | Next category |
| `Shift+Tab` | Previous category |
| `:` | Open the [command palette](#command-palette); `:` + alias + `Enter` jumps to a category |

**Category aliases**: type `:` then one of these shortcuts and press `Enter`:

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

Navigate back and forward through the lists you've visited:

| Key | Action |
|-----|--------|
| `[` | History back |
| `]` | History forward |

Each step returns to a list as you saw it, scope included: after drilling into a node's pods and going (`o`) to their DAC, `[` returns to that node's pods. Switching environments keeps the history's categories but drops the scopes that name rows of the old environment (a DAC, a node); a definition's scope, and a tenant's within the same realm, still hold and are kept, as the current scope is.

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

`Enter` opens the first child category that has rows for the selected parent (a tenant with no limit overrides opens its console property overrides, or its DACs); when none of the loaded ones do, the first child. The scope stays active when you
switch categories, so switching to another child (e.g. `:gw` after drilling
into a DAC) lists that child filtered to the same parent.

Press `Esc` to exit the scoped context and return to the parent, with the cursor on the row you drilled in from. It steps back in history when the parent is there, so `]` goes back into the child. (With a filter applied, the first `Esc` clears the filter.) On a parent
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

### Command palette

`:` opens the command palette over the table. Type to fuzzy-search:

- **Categories**: every category, matched by name or any alias.
- **Environments**: recent quick-switch slots first, then every environment. Listed only in environment-scoped categories, the same as the `0`–`5` keys. An environment you switch to takes a quick-switch slot only if your kubeconfig has its context.
- **Actions**: the selected row's actions (go to related, metrics, edit tenant, delete, …), each sort ("Sort by Tenant"), and Export CSV.
- **Toggles**: alias view, faulty-only, preview pane.

With nothing typed, the items you ran this session come first, under **recent**. Each group is named on its first row, a category's hint is its short aliases, and the open category is marked `●`.

`↑`/`↓` (or `Ctrl+P`/`Ctrl+N`) move, `Enter` runs, `Esc` closes. An exact alias always wins, so `:dac` + `Enter` opens DedicatedAICluster as it always has. Actions run exactly as their key would, confirmations included.

### Preview pane

On a terminal at least 160 columns wide, `v` splits the panel: the table on the left, and on the right the selected row's fields, laid out as in the detail view: plain values first, then one level of nested objects as indented sections (deeper ones collapse to `{…}`, lists of objects to `[n]`), each value on one line. It follows the cursor. Press `v` again to close it. It stays on for the session: narrowing the terminal hides it, and widening brings it back. On a narrower terminal, `v` says how many columns it needs.

---

## Categories

Toolkit organises data into 22 categories:

### Core Infrastructure

| Category | Description |
|----------|-------------|
| **Tenant** | One row per tenancy OCID found in the tenancy overrides or the tenant metadata file, with its internal flag and note; scopes into the tenant's overrides and resources. The name comes from the metadata file only (override directory names are ignored); a tenancy without a metadata name is named by its OCID. Names must be unique within a realm: a metadata file that gives two tenancies in one realm the same name fails to load. **Status** is the tenancy's account state (`ACTIVE`, `DELETING`, `TERMINATED`, `ALWAYS_FREE_ONLY`) as last recorded by `toolkit tenant sync`, blank when unknown; a `DELETING` or `TERMINATED` tenant is reported as faulty |
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
| **ServingRuntime** | OME `ClusterServingRuntime` CRs — the cluster-scoped templates defining how a class of models is served (hardware constraints, container image, resource caps); a `disabled` runtime is reported as faulty. The TUI's columns are a slimmer set than `toolkit get servingruntime`'s (see [Exporting Data](#exporting-data)). `Enter` lists the inference services naming the runtime; with that scope active, `:gw` lists the pods running it, including endpoints whose runtime OME selected at deploy time (`y` opens details) |

### Meta

| Category | Description |
|----------|-------------|
| **Alias** | Index of every category and its aliases; reachable with `Ctrl+A` or `:alias` |

---

## Filtering & Searching

### Enter filter mode

Press `/` to open the filter input in the breadcrumb line, under the env bar. Start typing and the table updates in real time (100 ms debounce).

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

Press `s`, then the letter of a column, to sort by it. While `s` is pending, each sortable column's letter is underlined in the header and the breadcrumb shows `s…`. A column's letter is its first letter, unique within the category, except three that are the same in every category: `s` is always Status, `z` siZe and `x` maX. Rows that tie on the sort column are ordered by the rest of the row, so they stay put across refreshes. Pressing the same sort again **reverses** the direction. `Esc`, or any key that isn't a column letter, cancels; a letter no column has says which ones would have worked (`no sort on x · try s n/i/s`).

`s n` sorts by Name in every list view. The header marks only the column the table is sorted by (`↑`/`↓`); `?` lists every sort under **Sort**, and pressing `s` underlines the letters. The other sortable columns:

| Category | Keys |
|----------|------|
| Tenant | `s i` Internal, `s s` Status |
| ConsolePropertyDefinition | `s v` Value |
| PropertyDefinition | `s v` Value |
| LimitDefinition | `s m` Min, `s x` Max |
| LimitTenancyOverride | `s t` Tenant, `s r` Regions, `s m` Min, `s x` Max |
| ConsolePropertyTenancyOverride | `s t` Tenant, `s r` Regions, `s v` Value |
| PropertyTenancyOverride | `s t` Tenant, `s r` Regions, `s v` Value |
| LimitRegionalOverride | `s r` Regions, `s m` Min, `s x` Max |
| ConsolePropertyRegionalOverride | `s r` Regions, `s v` Value |
| PropertyRegionalOverride | `s r` Regions, `s v` Value |
| BaseModel | `s z` Size, `s c` Context |
| ImportedModel | `s t` Tenant, `s z` Size, `s c` Context, `s v` Vendor |
| Environment | `s t` Type |
| ServiceTenancy | `s t` Type |
| GPUPool | `s z` Size, `s g` GPUs |
| GPUNode | `s f` Free, `s t` Type, `s a` Age |
| GPUWorkload | `s t` Tenant, `s g` GPUs, `s r` Restarts, `s a` Age |
| DedicatedAICluster | `s t` Tenant, `s i` Internal, `s u` Usage, `s z` Size, `s a` Age, `s s` Status |
| InferenceService | `s t` Tenant, `s r` Ready, `s a` Age |
| ServingRuntime | `s g` GPUs |

Sorting used to be on `Shift`+letter. Those keys are now free for row actions, and each one means the same action in every category: `Shift+R` is always Restart, `Shift+C` Cordon, `Shift+D` Drain, `Shift+U` Scale Up, `Shift+E` Edit Tenant.

### Toggle alias view

```
Ctrl+A    → toggle between full category names and short aliases in the UI
```

---

## Detail View

Press `y` on any selected row to open the **Detail View**. It shows the item's fields with readable labels: plain values first, then each nested object as a titled, indented section, and each list of objects as numbered entries. Lists of plain values are joined on one line, and long values wrap. `Tab` switches to the raw JSON (the same JSON `o` copies) and back. It follows the data: when the category reloads (the live watch, a repo change, an action finishing), the view shows the item as reloaded and keeps its scroll position. If the item is deleted meanwhile, the view keeps showing it, titled `· gone`, until you leave.

```
 Tenant › acme-corp                                                       [fields]
╭─ acme-corp ──────────────────────────────────────────────────────────────────────╮
│ Name         acme-corp                                                           │
│ ID           ocid1.tenancy.oc1..aaaa…                                            │
│ Is internal  false                                                               │
│ Status       ACTIVE                                                              │
╰──────────────────────────────────────────────────────────────────────────────────╯
 o copy object  tab fields/JSON  ? more
```

The panel's bottom edge shows the layout and, when the content is taller than the panel, how far down you are (`JSON · 40%`).

| Key | Action |
|-----|--------|
| `y` or `Esc` | Return to list view |
| `↑` / `↓` | Scroll |
| `Tab` | Switch between fields and JSON |
| `c` | Copy the item's name to clipboard |
| `o` | Copy the **entire JSON object** to clipboard |

---

## Infrastructure Operations

Certain categories expose live infrastructure operations. These require Kubernetes or OCI credentials to be configured.

> **Warning:** Drain and Restart disrupt running workloads, and Delete is **irreversible**. Toolkit guards against duplicate in-flight requests automatically.

**Edit tenant** (`Shift+E`, in any category with a Tenant column — DedicatedAICluster, ImportedModel, GPUWorkload, InferenceService and the three tenancy-override categories — on a row whose tenant has no metadata name yet, so it shows as an OCID or tenancy id) opens a short form: Name (required), Internal/External, Note (optional). `Tab` or `Enter` moves to the next field, `Enter` on the last field or `Ctrl+S` saves, `Ctrl+O` opens the tenancy's metadata page in the DevOps portal. Saving without a name marks the Name field instead of saving.

Every operation asks first. The confirmation names the category, the row and the kube context (with a red `PRODUCTION` badge in production) and says what the operation does. Irreversible operations take a capital `Y`; a lowercase `y` neither confirms nor cancels, it asks for the capital `Y`, so muscle memory never destroys anything and never throws the confirmation away: what a cordon or drain does to pods, how far a scale-up grows the pool, or why a delete can't be undone.

### GPU Nodes (`GPUNode`)

Select a node in the `GPUNode` category:

| Key | Operation | Description |
|-----|-----------|-------------|
| `Shift+C` | Cordon / Uncordon | Cordon a schedulable node, uncordon a cordoned one (the modal says which) |
| `Shift+D` | Drain | Evict all pods from the node |
| `Shift+R` | Restart | Soft-reset (restart) the node's instance |
| `Ctrl+X` | Delete | Terminate the node instance |
| `r` | Refresh | Reload GPU node data |
| `Ctrl+Z` | Faulty only | Show/hide nodes flagged as faulty |

### GPU Pools (`GPUPool`)

| Key | Operation | Description |
|-----|-----------|-------------|
| `Shift+U` | Scale Up | Request an increase in pool capacity |
| `r` | Refresh | Reload pool data |
| `Ctrl+Z` | Faulty only | Show/hide pools with faulty status |

### Dedicated AI Clusters (`DedicatedAICluster`)

| Key | Operation | Description |
|-----|-----------|-------------|
| `Ctrl+X` | Delete | Delete the selected Dedicated AI Cluster |
| `r` | Refresh | Reload cluster data |
| `Ctrl+Z` | Faulty only | Show/hide faulty clusters |

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
- **Returning to a cluster category** shows its rows from before at once
  and reloads them, since its watch stopped while you were away.
- **A repo edit that breaks loading** (a malformed file) keeps the data
  from before on screen and raises a warning naming the file; fixing the
  file reloads as usual.
- **Recovering a dropped watch:** the working-tree watch has no
  auto-reconnect. If it drops (rare — e.g. a filesystem error), press `r` to
  re-establish it.
- **Not watched:** the optional external metadata file
  (`metadata-file`, default `~/.config/toolkit/metadata.yaml`) lives outside
  the repo and is read at startup. Tenant metadata you edit in-app updates
  immediately; external edits to that file require a restart.

---

## Exporting Data

Press `e` in any list view to open the **Export CSV** dialog. It shows the **Folder** the file goes to (starting in your home directory, then wherever you last exported), the file **Name** (`<region>-<category>.csv`), and the folders inside.

1. Move into a folder with `→` (or `l`) and back out with `←` (or `h`).
2. To change the name, press `Tab`, type, and `Tab` back (or just press `Enter`).
3. Press `Enter` to export into the folder shown. A notification says where the file went.

`Esc` cancels. A blank name, or one containing `/`, is refused with a warning.

The exported CSV reflects the **current filter** and the faulty-toggle state. Two
things deliberately differ from what's on screen:

- **The interactive sort is not preserved.** Rows are written in the dataset's
  natural order, not the order `s n` / `s t` / … put them in.
- **OCID columns are expanded.** Where the table shows a shortened OCID suffix
  (the Name and Tenant columns on DedicatedAICluster / ImportedModel, and Tenant
  on GPUWorkload / InferenceService), the CSV carries the fully-qualified OCID so the file is ready
  for downstream OCI tooling.

`toolkit get <category> -o csv` produces the same bytes for the same filter, with
one deliberate exception: **ServingRuntime**. The TUI shows a slimmer set built for
scanning placement and rollout — Name, Hardware (GPU families only), GPUs, Image,
Status — and exports that. `toolkit get servingruntime` keeps the full set — Name,
Hardware (`ac:`/`it:` constraint), Image, Size Range, CPU/Mem, GPUs, Auto-Select,
Disabled — so scripts and `--columns` keys don't change underneath them. Size range
and CPU/memory remain in the TUI's detail view (`y`).

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
| `q` / `Ctrl+C` | Quit. With actions running it asks first and lists them, since quitting stops them (a drain resumes when run again); `q` again quits anyway |
| `?` / `h` | Toggle help overlay |
| `Esc` | Back / Clear filter |
| `y` | Toggle detail view |
| `c` | Copy item name / ID |
| `` ` `` | Open the [in-app log](#in-app-log) |

### List view

| Key | Action |
|-----|--------|
| `Tab` | Next category |
| `Shift+Tab` | Previous category |
| `/` | Enter filter mode |
| `:` | Open the command palette |
| `p` | Paste clipboard as filter |
| `Enter` | View item / scope into context |
| `[` | History back |
| `]` | History forward |
| `e` | Export table as CSV |
| `Ctrl+A` | Toggle alias view |
| `s` + letter | Sort by a column (`s n` Name; see [Sorting](#sorting)) |
| `g` / `G` | First / last row |
| `v` | Toggle the preview pane (≥160 columns) |

### Detail view

| Key | Action |
|-----|--------|
| `y` / `Esc` | Return to list |
| `↑` / `↓` | Scroll content |
| `Tab` | Switch between fields and JSON |
| `c` | Copy item name |
| `o` | Copy full JSON object |

### In-app help

Press `?` or `h` at any time to display the full keybinding help overlay. The help is **context-sensitive**: it only shows keys relevant to your current category and view mode, grouped as Quick switch, Actions, Sort, the view's keys, General and Navigation. Keys are written as the footer writes them: `C` for Shift+C, `^x` for Ctrl+X. Its sections flow into columns on wide terminals. `/` filters it as you type, by key or description (`/drain`, `/^x`); a section whose title matches stays whole (`/sort`), `Enter` keeps the filter, and `Esc` clears it before closing help. When they still don't fit, it scrolls (`j`/`k`, `PgUp`/`PgDn`, `Home`/`End`) and shows how far down you are.

The first time you run a version whose keys changed, a **Keys changed** note in the bottom-right corner says what moved. Any key dismisses it, and that key still does its usual job. The note won't show again: the version you've seen is saved in `~/.config/toolkit/ui.yaml`.

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

### In-app log

Press `` ` `` in the list or detail view to see the session's recent log entries without leaving the TUI. It sits in the usual frame: the breadcrumb shows `following` while it tails new entries, `PAUSED` once you scroll up, and the active filters; the panel's bottom edge counts the entries (`60 entries`, or `11 of 60` when filtered).

| Key | Action |
|-----|--------|
| `↑` / `↓`, `PgUp` / `PgDn` | Scroll (scrolling up pauses the tail) |
| `End` / `Home` | Follow the tail / jump to the oldest entry |
| `/` | Filter entries by text, case-insensitive (`Enter` keeps it, `Esc` clears it) |
| `l` | Cycle the minimum level: all → info → warn → error |
| `Esc` | Clear the filters; with none, close the log |
| `` ` `` | Close the log |

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
