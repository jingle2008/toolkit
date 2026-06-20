# Metrics Shortcut for ImportedModel & GPUWorkload — Design

**Date:** 2026-06-19

**Goal:** Extend the existing `<m>` "Open Metrics" shortcut (today DAC-only) to
the ImportedModel and GPUWorkload categories, including a new **on-demand**
metrics mode that filters by `ResourceId` and supports two additional model
capabilities.

**Builds on:** `2026-06-19-dac-metrics-shortcut-design.md` (the DAC metrics
shortcut, capability-aware dashboards, Zipson encoder, lazy catalog loading).

---

## Background

The DAC metrics shortcut opens an OCI Telemetry MQL Explore dashboard for the
selected DedicatedAICluster. Each MQL query is filtered by the DAC's OCID
(`{DacId = "<ocid>"}`), and the metric set is chosen from the served model's
capability (CHAT 9-query grid / TEXT_RERANK / TEXT_EMBEDDINGS).

Two more categories can carry the same kind of metrics:

- **ImportedModel** rows whose K8s `Namespace` is itself a DAC name.
- **GPUWorkload** rows, either dedicated (namespace is a DAC) or **on-demand**
  (serving a public base model, metrics scoped by `ResourceId`).

## Rules (authoritative)

1. **ImportedModel** — if its `Namespace` starts with `amaaaaaa` (the OCID
   resource-id prefix, same prefix imported/finetune model *names* carry),
   there is a DAC whose name equals the namespace. Show that DAC's metrics.
2. **GPUWorkload** — if `Model` is empty, do nothing. Otherwise:
   1. If `Namespace` starts with `amaaaaaa`, there is a DAC named = namespace;
      show DAC metrics (dedicated mode).
   2. Otherwise this is **on-demand mode**:
      1. Match `Model` against the **base** model catalog.
      2. Filter each capability-driven query by `ResourceId = "<displayName>"`
         (the matched base model's display name), e.g.
         `GenerativeAiService.chat.InputTokenLength[1m]{ResourceId = "openai.gpt-5.5"}.grouping().sum()`.
      3. On-demand adds two capabilities with fixed, **unfiltered** queries:
         - `TEXT_CLASSIFICATION` →
           `ContentModeration.TotalInvocation.Count[1m].grouping().sum()`
         - `IMAGE_CONTENT_MODERATION` →
           `ImageContentModeration.Latency.ChatInput[1m].grouping().sum()`
           and
           `ImageContentModeration.Latency.ApplyGuardrails[1m].grouping().sum()`

## Resolved decisions

- **Two new capabilities are on-demand only.** If a *dedicated* model (DAC or
  amaaaaaa-namespace workload) resolves to `TEXT_CLASSIFICATION` or
  `IMAGE_CONTENT_MODERATION`, treat it as unreachable: no-op with an error
  toast, do not open a dashboard.
- **Unresolvable cases show an error toast and open nothing** — GPUWorkload
  with empty `Model`, an ImportedModel whose namespace isn't a DAC, an
  on-demand `Model` not found in the base catalog. Consistent with the
  existing load-failure behavior.
- **Dedicated GPUWorkload capability comes from `GPUWorkload.Model`**, resolved
  via the catalog (imported if `Model` has the `amaaaaaa` prefix, else base) —
  mirrors how DAC resolves `ModelName`. Single catalog load. We do not look up
  the actual DAC by name.
- **ResourceId value is the matched base model's `DisplayName`** (e.g.
  `openai.gpt-5.5`).
- **Capability precedence:** `CHAT > TEXT_RERANK > TEXT_EMBEDDINGS >
  TEXT_CLASSIFICATION > IMAGE_CONTENT_MODERATION`. Nil / finetune / unknown
  fall back to CHAT.

## Architecture

Generalize the telemetry layer to a `Filter{Key, Value}` (replacing the
hard-coded `DacId`), extend the `Capability` enum, and replace the TUI's
`openDacMetrics` with one `openMetrics(item)` that type-switches over the three
item types and reuses the existing two-phase flow: determine the catalog →
(lazily) load it → fire a trigger message → resolve the plan on the Update loop
→ launch or toast. Telemetry stays a pure string/URL builder; the TUI owns all
item→plan resolution.

### Component 1 — Telemetry (`internal/infra/telemetry/mql.go`)

- Extend `Capability`: add `CapabilityTextClassification`,
  `CapabilityImageContentModeration` (after the existing three).
- Add a filter type:
  ```go
  type Filter struct {
      Key   string // FilterDacId or FilterResourceId
      Value string
  }
  const (
      FilterDacId      = "DacId"
      FilterResourceId = "ResourceId"
  )
  ```
- `metricQueries(capability Capability, filter Filter) []string`:
  - `CapabilityTextClassification` → the one fixed ContentModeration query,
    filter ignored.
  - `CapabilityImageContentModeration` → the two fixed ImageContentModeration
    queries, filter ignored.
  - `CapabilityTextRerank` / `CapabilityTextEmbeddings` → single query with the
    filtered suffix `[1m]{<Key> = "<Value>"}.grouping().sum()`.
  - default `CapabilityChat` → the 3×3 token-length grid with the filtered
    suffix.
- `MetricsURL(filter Filter, capability Capability, regionID, project, fleet string, start, end time.Time) string`
  (replaces the `dacOCID string` first parameter; body otherwise unchanged —
  panels, searchPanelState, layout, startMs/endMs, base64+url-escape).

### Component 2 — Models (`pkg/models`)

- `base_model.go`: add
  ```go
  CapabilityTextClassification     = "TEXT_CLASSIFICATION"
  CapabilityImageContentModeration = "IMAGE_CONTENT_MODERATION"
  ```
  next to the existing capability constants.
- `dataset.go`: add `FindBaseModelByName(name string) *BaseModel` — searches
  only `BaseModels` (the on-demand rule explicitly matches the base catalog).
  Returns nil on empty name or no match.

### Component 3 — TUI resolution (`internal/ui/tui/reducer_actions.go`)

Rename `openDacMetrics` → `openMetrics(item any)`. The dispatch in
`handleItemActions` becomes `return m.openMetrics(item)`.

Resolution table:

| Item | Condition | Catalog to load | Filter | Capability from | Outcome |
|---|---|---|---|---|---|
| DAC | `ModelName==""` | none | `DacId`=DAC.OCID | — | launch CHAT |
| DAC | `ModelName` set | base/imported by name prefix | `DacId`=DAC.OCID | `FindModelByName(ModelName)` | dedicated |
| ImportedModel | ns has `amaaaaaa` | none (caps inline on row) | `DacId`=OCID(name=ns) | item's own embedded `BaseModel` | dedicated |
| ImportedModel | ns empty/other | none | — | — | toast: not tied to a DAC |
| GPUWorkload | `Model==""` | none | — | — | toast: no model |
| GPUWorkload | `Model` set, ns `amaaaaaa` | base/imported by `Model` prefix | `DacId`=OCID(name=ns) | `FindModelByName(Model)` | dedicated |
| GPUWorkload | `Model` set, ns other | base | `ResourceId`=`DisplayName` | `FindBaseModelByName(Model)` | on-demand |

- **dedicated** rows: if the resolved capability is `CapabilityTextClassification`
  or `CapabilityImageContentModeration` → error toast, no open.
- **on-demand**: base match not found → error toast; otherwise all five
  capabilities are valid.
- A namespace's DAC OCID is `models.DedicatedAICluster{Name: ns}.OCID(realm, region)`.

Control flow:

- `openMetrics(item)` computes `(cat domain.Category, need bool)` via a pure
  `metricsCatalog(item)`. If `!need || catalogLoaded(cat)`, it returns
  `finishMetrics(item)` immediately. Otherwise it bumps the generation and
  returns `tea.Sequence(tea.Batch(beginTask, catalogLoadCmd(cat, gen)), trigger)`
  where the trigger is `openMetricsTriggerMsg{item, cat, gen}`.
- `openMetricsTriggerMsg` carries `item any`, `cat domain.Category`, `gen int`.
- `handleOpenMetricsTrigger`: decline (nil) on stale gen or `!catalogLoaded(cat)`
  (load failed → its errMsg toast already fired; or stale-dropped); else
  `finishMetrics(item)`.
- `finishMetrics(item)` calls `resolveMetricsPlan(item) (filter, capability, ok, reason)`:
  on `!ok` returns `showToast(reason, toastError)`; else `launchMetrics(filter, capability)`.
- `launchMetrics(filter, capability)` builds the URL via the updated
  `metricsURL(env, filter, capability, now)` and opens it off the UI goroutine,
  reporting failure as `metricsOpenErrMsg`.
- `capabilityForModel(*models.BaseModel)` extended with the two new capabilities
  at the end of the precedence chain.

The item pointers returned by `findItem` index into `m.dataset` and remain valid
across a background catalog load, because that load no longer refreshes the
on-screen table (see the `applyDataset` category guard).

### Component 4 — Keybinding (`internal/ui/tui/keys/registry.go`)

Add `OpenMetrics` to `catContext[domain.GPUWorkload][common.ListView]` and
`catContext[domain.ImportedModel][common.ListView]`. Help text unchanged
("Open Metrics").

## Error handling

Every unresolvable path produces an error toast and opens nothing:

- "no model" — GPUWorkload with empty `Model`.
- "not tied to a dedicated AI cluster" — ImportedModel whose namespace lacks the
  `amaaaaaa` prefix (or is empty).
- "model not found" — on-demand `Model` absent from the base catalog.
- dedicated classification/moderation — "metrics not available for this model".

Catalog load failures retain the existing `metricsOpenErrMsg` toast and the
stale-generation guard.

## Testing

- **Telemetry:** golden Zipson/MQL for (a) a `ResourceId`-filtered query set,
  (b) the `TEXT_CLASSIFICATION` fixed query, (c) the two
  `IMAGE_CONTENT_MODERATION` fixed queries. Confirm filter ignored for the two
  new capabilities.
- **Models:** `FindBaseModelByName` (hit in base, miss when only in imported,
  empty name); `capabilityForModel` precedence including the two new caps.
- **TUI:** one test per resolution-table row — DAC (existing, regression),
  ImportedModel dedicated, ImportedModel not-a-DAC toast, GPUWorkload empty-model
  toast, GPUWorkload dedicated, GPUWorkload on-demand, on-demand not-found toast,
  dedicated classification no-op toast. Plus the catalog-load-then-trigger path
  for a workload (generation bump dispatched; stale-gen and not-loaded triggers
  decline).

## Out of scope

- Changing the metric window (stays 7 days), Project, or fleet derivation.
- Resolving on-demand models against the imported catalog (base only, per rule).
- Any DAC-mode support for the two new capabilities.
