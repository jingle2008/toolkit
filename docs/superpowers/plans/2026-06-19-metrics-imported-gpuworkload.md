# Metrics Shortcut for ImportedModel & GPUWorkload — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extend the `<m>` "Open Metrics" shortcut from DAC-only to ImportedModel and GPUWorkload, including a new on-demand mode (ResourceId-filtered queries + two new capabilities).

**Architecture:** Generalize the telemetry layer to a `Filter{Key,Value}` (replacing the hard-coded `DacId`) and extend the `Capability` enum; replace the TUI's `openDacMetrics` with one `openMetrics(item)` that type-switches over DAC / ImportedModel / GPUWorkload and reuses the existing two-phase flow (determine catalog → lazily load → trigger message → resolve on the Update loop → launch or toast). Telemetry stays a pure string/URL builder; the TUI owns item→plan resolution.

**Tech Stack:** Go, Bubble Tea (charmbracelet), testify. Spec: `docs/superpowers/specs/2026-06-19-metrics-imported-gpuworkload-design.md`.

## Global Constraints

- `importedModelNamePrefix = "amaaaaaa"` — a namespace or model name with this prefix is OCID-form (imported/finetune / a DAC name); anything else is a public base model.
- Capability strings (exact, in `pkg/models`): `CHAT`, `TEXT_RERANK`, `TEXT_EMBEDDINGS`, `TEXT_CLASSIFICATION`, `IMAGE_CONTENT_MODERATION`.
- Capability precedence in `capabilityForModel`: `CHAT > TEXT_RERANK > TEXT_EMBEDDINGS > TEXT_CLASSIFICATION > IMAGE_CONTENT_MODERATION`; nil / `Fine-tuning` / unknown → `CapabilityChat`.
- Filter dimension keys (exact): `DacId`, `ResourceId`. Query suffix for filtered capabilities: `[1m]{<Key> = "<Value>"}.grouping().sum()` (note the spaces around `=`).
- The two on-demand-only capability query sets are fixed and unfiltered, used verbatim:
  - `TEXT_CLASSIFICATION` → `ContentModeration.TotalInvocation.Count[1m].grouping().sum()`
  - `IMAGE_CONTENT_MODERATION` → `ImageContentModeration.Latency.ChatInput[1m].grouping().sum()` and `ImageContentModeration.Latency.ApplyGuardrails[1m].grouping().sum()`
- On-demand `ResourceId` value is the matched **base** model's `DisplayName`.
- Two on-demand-only capabilities are unreachable in dedicated mode: a dedicated model (DAC / amaaaaaa-namespace workload) resolving to one → error toast, no open.
- Unresolvable cases → error toast, open nothing. Metric window stays 7 days; fleet stays `"generative-ai-service-api-" + env.Type`; project stays `telemetry.Project`.
- Run `make ci` green before the final commit of each task that touches Go.

---

### Task 1: Telemetry — Filter type, extended Capability, generalized queries/URL

**Files:**
- Modify: `internal/infra/telemetry/mql.go`
- Test: `internal/infra/telemetry/mql_test.go`

**Interfaces:**
- Consumes: existing `Encoder` (zipson.go), `exploreBaseURL`, `Project`.
- Produces:
  - `type Filter struct { Key, Value string }`
  - `const FilterDacId = "DacId"`, `const FilterResourceId = "ResourceId"`
  - `Capability` adds `CapabilityTextClassification`, `CapabilityImageContentModeration`
  - `func metricQueries(capability Capability, filter Filter) []string` (param order changed)
  - `func MetricsURL(filter Filter, capability Capability, regionID, project, fleet string, start, end time.Time) string` (first param changed from `dacOCID string`)

- [ ] **Step 1: Update the existing tests to the new signatures (they must fail to compile first)**

In `internal/infra/telemetry/mql_test.go`, change the four existing call sites:

```go
// TestMetricQueries
got := metricQueries(CapabilityChat, Filter{Key: FilterDacId, Value: testOCID})

// TestMetricsURL
got := MetricsURL(Filter{Key: FilterDacId, Value: testOCID}, CapabilityChat, "me-abudhabi-1", "GenerativeAIService", "generative-ai-service-api-prod", start, end)

// TestMetricQueries_RerankAndEmbed
assert.Equal(t, []string{
    `GenerativeAiService.rerankText.InputTokenLength[1m]{DacId = "` + testOCID + `"}.grouping().sum()`,
}, metricQueries(CapabilityTextRerank, Filter{Key: FilterDacId, Value: testOCID}))
assert.Equal(t, []string{
    `GenerativeAiService.embedText.InputTokenLength[1m]{DacId = "` + testOCID + `"}.grouping().sum()`,
}, metricQueries(CapabilityTextEmbeddings, Filter{Key: FilterDacId, Value: testOCID}))

// TestMetricsURL_RerankSingleQuery
got := MetricsURL(Filter{Key: FilterDacId, Value: testOCID}, CapabilityTextRerank, "me-abudhabi-1",
    "GenerativeAIService", "generative-ai-service-api-prod",
    time.UnixMilli(1781787680652), time.UnixMilli(1781832733444))

// TestMetricsURL_EmbedSingleQuery
got := MetricsURL(Filter{Key: FilterDacId, Value: testOCID}, CapabilityTextEmbeddings, "me-abudhabi-1",
    "GenerativeAIService", "generative-ai-service-api-prod",
    time.UnixMilli(1781787680652), time.UnixMilli(1781832733444))
```

The `wantZipson` golden constant is unchanged: a `DacId` filter produces byte-identical output to today's hard-coded `DacId`.

- [ ] **Step 2: Add new failing tests for ResourceId filtering and the two new capabilities**

Append to `internal/infra/telemetry/mql_test.go`:

```go
func TestMetricQueries_ResourceIdFilter(t *testing.T) {
	t.Parallel()
	f := Filter{Key: FilterResourceId, Value: "openai.gpt-5.5"}
	assert.Equal(t, []string{
		`GenerativeAiService.rerankText.InputTokenLength[1m]{ResourceId = "openai.gpt-5.5"}.grouping().sum()`,
	}, metricQueries(CapabilityTextRerank, f))
	got := metricQueries(CapabilityChat, f)
	assert.Len(t, got, 9)
	assert.Equal(t, `GenerativeAiService.chat.InputTokenLength[1m]{ResourceId = "openai.gpt-5.5"}.grouping().sum()`, got[0])
}

func TestMetricQueries_ClassificationFixedUnfiltered(t *testing.T) {
	t.Parallel()
	// Filter is ignored for the content-moderation capabilities.
	want := []string{`ContentModeration.TotalInvocation.Count[1m].grouping().sum()`}
	assert.Equal(t, want, metricQueries(CapabilityTextClassification, Filter{Key: FilterResourceId, Value: "x"}))
}

func TestMetricQueries_ImageContentModerationFixedUnfiltered(t *testing.T) {
	t.Parallel()
	want := []string{
		`ImageContentModeration.Latency.ChatInput[1m].grouping().sum()`,
		`ImageContentModeration.Latency.ApplyGuardrails[1m].grouping().sum()`,
	}
	assert.Equal(t, want, metricQueries(CapabilityImageContentModeration, Filter{Key: FilterDacId, Value: "x"}))
}
```

- [ ] **Step 3: Run the tests — expect compile failure / FAIL**

Run: `go test ./internal/infra/telemetry/ -run 'TestMetric' -count=1`
Expected: build error (`metricQueries`/`MetricsURL` signatures, `Filter`, new capability consts not defined).

- [ ] **Step 4: Implement the telemetry changes**

Replace the body of `internal/infra/telemetry/mql.go` (keep the package, imports, `exploreBaseURL`, `Project`) with:

```go
// Capability selects which metric set a dashboard shows, derived from the
// capability of the served model.
type Capability int

const (
	// CapabilityChat is the default — the chat/chatCompletions/responses
	// token-length grid. Also the fallback for finetune / unresolved /
	// unknown models.
	CapabilityChat Capability = iota
	// CapabilityTextRerank is a single rerankText query.
	CapabilityTextRerank
	// CapabilityTextEmbeddings is a single embedText query.
	CapabilityTextEmbeddings
	// CapabilityTextClassification is the on-demand content-moderation
	// invocation count. On-demand only; fixed, unfiltered query.
	CapabilityTextClassification
	// CapabilityImageContentModeration is the on-demand image-moderation
	// latency pair. On-demand only; fixed, unfiltered queries.
	CapabilityImageContentModeration
)

// Filter dimension keys.
const (
	FilterDacId      = "DacId"
	FilterResourceId = "ResourceId"
)

// Filter scopes capability-driven metric queries to one resource: Key is the
// MQL dimension (FilterDacId or FilterResourceId), Value its value (a DAC
// OCID or a model display name). The two content-moderation capabilities
// ignore the filter — their queries are fixed and unfiltered.
type Filter struct {
	Key   string
	Value string
}

func (f Filter) suffix() string {
	return `[1m]{` + f.Key + ` = "` + f.Value + `"}.grouping().sum()`
}

// metricGroups × metricKinds form the 3×3 chat token-length metric grid.
var (
	metricGroups = []string{"chat", "chatCompletions", "responses"}
	metricKinds  = []string{"Input", "Output", "Reasoning"}
)

// metricQueries returns the MQL query strings for a capability and filter.
// The content-moderation capabilities return fixed, unfiltered queries; the
// others apply filter.suffix().
func metricQueries(capability Capability, filter Filter) []string {
	switch capability {
	case CapabilityTextClassification:
		return []string{`ContentModeration.TotalInvocation.Count[1m].grouping().sum()`}
	case CapabilityImageContentModeration:
		return []string{
			`ImageContentModeration.Latency.ChatInput[1m].grouping().sum()`,
			`ImageContentModeration.Latency.ApplyGuardrails[1m].grouping().sum()`,
		}
	case CapabilityTextRerank:
		return []string{`GenerativeAiService.rerankText.InputTokenLength` + filter.suffix()}
	case CapabilityTextEmbeddings:
		return []string{`GenerativeAiService.embedText.InputTokenLength` + filter.suffix()}
	default: // CapabilityChat
		suffix := filter.suffix()
		queries := make([]string, 0, len(metricGroups)*len(metricKinds))
		for _, g := range metricGroups {
			for _, k := range metricKinds {
				queries = append(queries, `GenerativeAiService.`+g+`.`+k+`TokenLength`+suffix)
			}
		}
		return queries
	}
}

// MetricsURL builds the full OCI Telemetry MQL Explore URL: a Zipson
// dashboard payload, base64-std-encoded and URL-escaped.
func MetricsURL(filter Filter, capability Capability, regionID, project, fleet string, start, end time.Time) string {
	var e Encoder
	e.BeginObject()
	e.Key("panels").BeginArray()
	e.BeginObject().Key("legendType").Int(1).Key("queries").BeginArray()
	for _, q := range metricQueries(capability, filter) {
		e.BeginObject().
			Key("regionId").Str(regionID).
			Key("project").Str(project).
			Key("fleet").Str(fleet).
			Key("tql").Str(q).
			Key("visible").Bool(true).
			Key("expanded").Bool(false).
			EndObject()
	}
	e.EndArray().EndObject()
	e.EndArray()
	e.Key("searchPanelState").BeginObject().
		Key("regionId").Str(regionID).
		Key("project").Str(project).
		Key("fleet").Str(fleet).
		EndObject()
	e.Key("layout").Str("full")
	e.Key("startMs").Int(start.UnixMilli())
	e.Key("endMs").Int(end.UnixMilli())
	e.EndObject()

	data := base64.StdEncoding.EncodeToString([]byte(e.String()))
	return exploreBaseURL + "?data=" + url.QueryEscape(data)
}
```

- [ ] **Step 5: Run the tests — expect PASS**

Run: `go test ./internal/infra/telemetry/ -count=1`
Expected: `ok` (golden `wantZipson` still matches; new tests pass).

- [ ] **Step 6: Commit**

```bash
git add internal/infra/telemetry/mql.go internal/infra/telemetry/mql_test.go
git commit -m "feat(telemetry): generalize metrics queries to Filter + on-demand capabilities"
```

---

### Task 2: Models — new capability constants and base-only finder

**Files:**
- Modify: `pkg/models/base_model.go:57-61` (the capability const block)
- Modify: `pkg/models/dataset.go` (add finder after `FindModelByName`)
- Test: `pkg/models/capability_test.go`, `pkg/models/dataset_test.go` (create if absent)

**Interfaces:**
- Produces:
  - `const CapabilityTextClassification = "TEXT_CLASSIFICATION"`
  - `const CapabilityImageContentModeration = "IMAGE_CONTENT_MODERATION"`
  - `func (d *Dataset) FindBaseModelByName(name string) *BaseModel`

- [ ] **Step 1: Write failing tests**

Append to `pkg/models/capability_test.go`:

```go
func TestCapabilityConstants(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "TEXT_CLASSIFICATION", CapabilityTextClassification)
	assert.Equal(t, "IMAGE_CONTENT_MODERATION", CapabilityImageContentModeration)
}
```

Create/append `pkg/models/dataset_test.go`:

```go
package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFindBaseModelByName(t *testing.T) {
	t.Parallel()
	d := &Dataset{
		BaseModels: []BaseModel{{Name: "base-a", DisplayName: "openai.gpt-5.5"}},
		ImportedModelMap: map[string][]ImportedModel{
			"t": {{BaseModel: BaseModel{Name: "imported-b"}}},
		},
	}
	assert.Equal(t, "openai.gpt-5.5", d.FindBaseModelByName("base-a").DisplayName)
	assert.Nil(t, d.FindBaseModelByName("imported-b"), "imported models are not in the base catalog")
	assert.Nil(t, d.FindBaseModelByName("missing"))
	assert.Nil(t, d.FindBaseModelByName(""))
}
```

(If `pkg/models/capability_test.go` lacks the testify import, it already uses `assert` — confirm the import block has `"github.com/stretchr/testify/assert"`.)

- [ ] **Step 2: Run — expect FAIL**

Run: `go test ./pkg/models/ -run 'TestCapabilityConstants|TestFindBaseModelByName' -count=1`
Expected: build error (consts and method undefined).

- [ ] **Step 3: Implement**

In `pkg/models/base_model.go`, replace the const block:

```go
const (
	CapabilityChat                   = "CHAT"
	CapabilityTextRerank             = "TEXT_RERANK"
	CapabilityTextEmbeddings         = "TEXT_EMBEDDINGS"
	CapabilityTextClassification     = "TEXT_CLASSIFICATION"
	CapabilityImageContentModeration = "IMAGE_CONTENT_MODERATION"
)
```

In `pkg/models/dataset.go`, add after `FindModelByName`:

```go
// FindBaseModelByName returns the BaseModel whose Name matches name from the
// shared BaseModels catalog only (imported models are excluded). Returns nil
// on empty name or no match. Used to resolve an on-demand GPU workload's
// model to the public base model whose display name scopes its metrics.
func (d *Dataset) FindBaseModelByName(name string) *BaseModel {
	if name == "" {
		return nil
	}
	for i := range d.BaseModels {
		if d.BaseModels[i].Name == name {
			return &d.BaseModels[i]
		}
	}
	return nil
}
```

- [ ] **Step 4: Run — expect PASS**

Run: `go test ./pkg/models/ -count=1`
Expected: `ok`.

- [ ] **Step 5: Commit**

```bash
git add pkg/models/base_model.go pkg/models/dataset.go pkg/models/capability_test.go pkg/models/dataset_test.go
git commit -m "feat(models): add on-demand capability consts and base-only model finder"
```

---

### Task 3: TUI — refactor metrics flow to the generalized Filter/plan pipeline (DAC behavior preserved)

**Files:**
- Modify: `internal/ui/tui/reducer_actions.go`
- Modify: `internal/ui/tui/model_update.go` (the `openMetricsTriggerMsg` case already calls `m.handleOpenMetricsTrigger(msg)` — no change needed; verify)
- Test: `internal/ui/tui/dac_metrics_test.go`, `internal/ui/tui/model_capability_test.go`

**Interfaces:**
- Consumes: Task 1 (`telemetry.Filter`, `FilterDacId`, `MetricsURL(filter,…)`), existing `m.bumpGen`, `m.beginTask`, `m.catalogLoadCmd`, `m.catalogLoaded`, `m.showToast`, `actions.OpenURL`, `capabilityForModel`.
- Produces (used by Task 4):
  - `func (m *Model) openMetrics(item any) tea.Cmd`
  - `func (m *Model) metricsCatalog(item any) (domain.Category, bool)`
  - `func modelCatalog(modelName string) domain.Category`
  - `func (m *Model) finishMetrics(item any) tea.Cmd`
  - `func (m *Model) resolveMetricsPlan(item any) (telemetry.Filter, telemetry.Capability, bool, string)`
  - `func (m *Model) dedicatedPlan(filter telemetry.Filter, model *models.BaseModel) (telemetry.Filter, telemetry.Capability, bool, string)`
  - `func (m *Model) launchMetrics(filter telemetry.Filter, capability telemetry.Capability) tea.Cmd`
  - `func metricsURL(env models.Environment, filter telemetry.Filter, capability telemetry.Capability, now time.Time) string`
  - `openMetricsTriggerMsg struct { item any; cat domain.Category; gen int }`

- [ ] **Step 1: Update the dispatch and existing tests to the new names/signatures (compile-fail first)**

In `internal/ui/tui/reducer_actions.go`, change the action dispatch (currently `return m.openDacMetrics(item)`):

```go
	case key.Matches(msg, keys.OpenMetrics):
		return m.openMetrics(item)
```

In `internal/ui/tui/dac_metrics_test.go`:

```go
func TestMetricsURL_Build(t *testing.T) {
	t.Parallel()
	env := models.Environment{Realm: "oc1", Region: "me-abudhabi-1", Type: "prod"}
	got := metricsURL(env, telemetry.Filter{Key: telemetry.FilterDacId, Value: "ocid1.dac.oc1.me-abudhabi-1.x"}, telemetry.CapabilityChat, time.UnixMilli(1781832733444))
	require.True(t, strings.HasPrefix(got,
		"https://devops.oci.oraclecorp.com/telemetry/mql/explore?data="),
		"unexpected URL: %s", got)
	assert.Greater(t, len(got), 256)
}

func TestOpenMetrics_UnknownIsNoOp(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	assert.Nil(t, m.openMetrics("not a dac"))
	assert.Nil(t, m.openMetrics((*models.DedicatedAICluster)(nil)))
}

func TestOpenMetrics_DACReturnsCmd(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	require.NotNil(t, m.openMetrics(&models.DedicatedAICluster{Name: "dac1"}))
}
```

In `internal/ui/tui/model_capability_test.go`, (a) delete `TestModelCapability` (the `modelCapability` method is removed — its logic is inlined into `resolveMetricsPlan`); (b) rename the four `m.openDacMetrics(` calls to `m.openMetrics(`; (c) rewrite the three `TestHandleOpenMetricsTrigger_*` to pass an `item`:

```go
func TestHandleOpenMetricsTrigger_StaleGenNoOpen(t *testing.T) {
	t.Parallel()
	m := makeTestModel()
	m.dataset = &models.Dataset{BaseModels: []models.BaseModel{{Name: "r", Capabilities: []string{"TEXT_RERANK"}}}}
	got := m.handleOpenMetricsTrigger(openMetricsTriggerMsg{
		item: &models.DedicatedAICluster{Name: "d", ModelName: "r"}, cat: domain.BaseModel, gen: m.gen + 1,
	})
	assert.Nil(t, got, "stale generation does not open")
}

func TestHandleOpenMetricsTrigger_CatalogNotLoadedNoOpen(t *testing.T) {
	t.Parallel()
	m := makeTestModel() // dataset nil → catalog load must have failed/dropped
	got := m.handleOpenMetricsTrigger(openMetricsTriggerMsg{
		item: &models.DedicatedAICluster{Name: "d", ModelName: "r"}, cat: domain.BaseModel, gen: m.gen,
	})
	assert.Nil(t, got, "no open when the catalog isn't loaded")
}

func TestHandleOpenMetricsTrigger_OpensWhenLoaded(t *testing.T) {
	t.Parallel()
	m := makeTestModel()
	m.dataset = &models.Dataset{BaseModels: []models.BaseModel{{Name: "r", Capabilities: []string{"TEXT_RERANK"}}}}
	got := m.handleOpenMetricsTrigger(openMetricsTriggerMsg{
		item: &models.DedicatedAICluster{Name: "d", ModelName: "r"}, cat: domain.BaseModel, gen: m.gen,
	})
	require.NotNil(t, got, "opens the dashboard once the catalog is applied")
}
```

Leave `TestCapabilityForModel`, `TestCatalogLoaded`, and the `TestOpenDacMetrics_*` resolution tests in place but rename their `m.openDacMetrics(` calls to `m.openMetrics(`. (They keep their `TestOpenDacMetrics_*` names; only the method call changes.)

- [ ] **Step 2: Run — expect compile failure**

Run: `go test ./internal/ui/tui/ -run 'TestOpenMetrics|TestHandleOpenMetricsTrigger|TestMetricsURL_Build' -count=1`
Expected: build error (`openMetrics`, new `openMetricsTriggerMsg` shape, `metricsURL` signature, removed `modelCapability`).

- [ ] **Step 3: Rewrite the metrics block in `reducer_actions.go`**

Replace everything from `// metricsOpenErrMsg reports…` through the end of `capabilityForModel` (the current lines 146-300) with the following. `capabilityForModel` stays at three capabilities in this task — Task 4 extends it.

```go
// metricsOpenErrMsg reports a failure to launch the metrics dashboard.
type metricsOpenErrMsg struct{ err error }

// metricsWindow is how far back the metrics dashboard looks.
const metricsWindow = 7 * 24 * time.Hour

// importedModelNamePrefix is the OCID resource-id prefix carried by
// tenant-owned imported/finetune model names and by DAC names. A model name
// with this prefix resolves against the imported catalog, anything else
// against the base catalog. A workload/imported-model namespace with this
// prefix is a DAC name.
const importedModelNamePrefix = "amaaaaaa"

// openMetricsTriggerMsg is the second step of openMetrics's sequence: it
// fires after the model catalog has been loaded and applied, so its handler
// resolves the plan against the now-populated dataset on the Update loop. gen
// pins it to the load it followed; item is the selection to resolve.
type openMetricsTriggerMsg struct {
	item any
	cat  domain.Category
	gen  int
}

// openMetrics opens the OCI Telemetry MQL dashboard for the selected item
// (DAC, ImportedModel, or GPUWorkload). If the plan needs a model catalog
// that isn't loaded yet, the catalog is fetched (and cached for later
// navigation) before the dashboard opens. Items that can't produce metrics
// are a no-op or an error toast (resolveMetricsPlan).
func (m *Model) openMetrics(item any) tea.Cmd {
	cat, need := m.metricsCatalog(item)
	if !need || m.catalogLoaded(cat) {
		return m.finishMetrics(item)
	}
	gen := m.bumpGen()
	return tea.Sequence(
		tea.Batch(m.beginTask(), m.catalogLoadCmd(cat, gen)),
		func() tea.Msg { return openMetricsTriggerMsg{item: item, cat: cat, gen: gen} },
	)
}

// metricsCatalog reports which model catalog must be loaded before the
// item's plan can be resolved. need is false when no catalog is required
// (DAC without a model) or the item can't produce metrics here (default).
// Task 4 adds the ImportedModel and GPUWorkload cases.
func (m *Model) metricsCatalog(item any) (domain.Category, bool) {
	switch it := item.(type) {
	case *models.DedicatedAICluster:
		if it == nil || it.ModelName == "" {
			return domain.BaseModel, false
		}
		return modelCatalog(it.ModelName), true
	default:
		return domain.BaseModel, false
	}
}

// modelCatalog routes a model NAME to the catalog that holds it: imported/
// finetune names carry importedModelNamePrefix, everything else is base.
func modelCatalog(modelName string) domain.Category {
	if strings.HasPrefix(modelName, importedModelNamePrefix) {
		return domain.ImportedModel
	}
	return domain.BaseModel
}

// catalogLoaded reports whether the given model catalog is present on the
// dataset (nil means not loaded yet).
func (m *Model) catalogLoaded(cat domain.Category) bool {
	if m.dataset == nil {
		return false
	}
	switch cat { //nolint:exhaustive // only the two model catalogs are relevant here
	case domain.ImportedModel:
		return m.dataset.ImportedModelMap != nil
	default:
		return m.dataset.BaseModels != nil
	}
}

// catalogLoadCmd returns the shared loader command for one model catalog; its
// *LoadedMsg is applied by the normal handler, caching the catalog on the
// dataset for later navigation.
func (m *Model) catalogLoadCmd(cat domain.Category, gen int) tea.Cmd {
	switch cat { //nolint:exhaustive // only the two model catalogs are loadable here
	case domain.ImportedModel:
		return loadImportedModelsCmd(m.loadCtx, m.loader, m.kubeConfig, m.environment, gen)
	default:
		return loadBaseModelsCmd(m.loadCtx, m.loader, m.kubeConfig, m.environment, gen)
	}
}

// handleOpenMetricsTrigger resolves and opens the dashboard once the catalog
// load has been applied. It declines on a stale generation (a later load
// superseded this one) or when the catalog still isn't loaded (load failed —
// its errMsg toast already fired — or was stale-dropped).
func (m *Model) handleOpenMetricsTrigger(msg openMetricsTriggerMsg) tea.Cmd {
	if msg.gen != m.gen || !m.catalogLoaded(msg.cat) {
		return nil
	}
	return m.finishMetrics(msg.item)
}

// finishMetrics resolves the item's plan and either launches the dashboard
// or shows an error toast. A no-op item (empty reason) yields nil.
func (m *Model) finishMetrics(item any) tea.Cmd {
	filter, capability, ok, reason := m.resolveMetricsPlan(item)
	if !ok {
		if reason == "" {
			return nil
		}
		return m.showToast(reason, toastError)
	}
	return m.launchMetrics(filter, capability)
}

// resolveMetricsPlan maps a selected item to its metrics plan: the MQL filter
// and capability, plus ok/reason. ok=false with a non-empty reason is a
// user-facing error toast; ok=false with empty reason is a silent no-op
// (unknown/nil item). Reads m.dataset, which is non-nil whenever a catalog
// was required (guaranteed loaded before this runs). Task 4 adds the
// ImportedModel and GPUWorkload cases.
func (m *Model) resolveMetricsPlan(item any) (telemetry.Filter, telemetry.Capability, bool, string) {
	realm, region := m.environment.Realm, m.environment.Region
	switch it := item.(type) {
	case *models.DedicatedAICluster:
		if it == nil {
			return telemetry.Filter{}, 0, false, ""
		}
		filter := telemetry.Filter{Key: telemetry.FilterDacId, Value: it.OCID(realm, region)}
		if it.ModelName == "" {
			return filter, telemetry.CapabilityChat, true, ""
		}
		return m.dedicatedPlan(filter, m.dataset.FindModelByName(it.ModelName))
	default:
		return telemetry.Filter{}, 0, false, ""
	}
}

// dedicatedPlan finalizes a DacId-filtered (dedicated-mode) plan. The two
// on-demand-only capabilities are unreachable in dedicated mode: a model that
// resolves to one yields a no-op error toast rather than a meaningless
// dashboard. (capabilityForModel only returns those two once Task 4 extends
// it; until then this switch's first case is inert.)
func (m *Model) dedicatedPlan(filter telemetry.Filter, model *models.BaseModel) (telemetry.Filter, telemetry.Capability, bool, string) {
	capability := capabilityForModel(model)
	switch capability { //nolint:exhaustive // only the two on-demand caps are special-cased
	case telemetry.CapabilityTextClassification, telemetry.CapabilityImageContentModeration:
		return telemetry.Filter{}, 0, false, "metrics not available for this model"
	default:
		return filter, capability, true, ""
	}
}

// launchMetrics opens the dashboard URL in the browser off the UI goroutine,
// reporting a launch failure as an error toast.
func (m *Model) launchMetrics(filter telemetry.Filter, capability telemetry.Capability) tea.Cmd {
	target := metricsURL(m.environment, filter, capability, time.Now())
	return func() tea.Msg {
		if err := actions.OpenURL(target); err != nil {
			return metricsOpenErrMsg{err: err}
		}
		return nil
	}
}

// metricsURL builds the OCI Telemetry MQL dashboard URL from the environment
// (region/type), an MQL filter, a capability, and a window ending at now.
// Pure; unit-testable without launching a browser.
func metricsURL(env models.Environment, filter telemetry.Filter, capability telemetry.Capability, now time.Time) string {
	fleet := "generative-ai-service-api-" + env.Type
	return telemetry.MetricsURL(filter, capability, env.Region, telemetry.Project, fleet,
		now.Add(-metricsWindow), now)
}

// capabilityForModel maps a resolved model to its metric capability,
// defaulting to chat for nil/finetune/unrecognized. Precedence:
// CHAT > TEXT_RERANK > TEXT_EMBEDDINGS.
func capabilityForModel(model *models.BaseModel) telemetry.Capability {
	if model == nil || model.Type == "Fine-tuning" {
		return telemetry.CapabilityChat
	}
	switch {
	case model.HasCapability(models.CapabilityChat):
		return telemetry.CapabilityChat
	case model.HasCapability(models.CapabilityTextRerank):
		return telemetry.CapabilityTextRerank
	case model.HasCapability(models.CapabilityTextEmbeddings):
		return telemetry.CapabilityTextEmbeddings
	default:
		return telemetry.CapabilityChat
	}
}
```

Note: the `selectedItem` helper and everything above line 146 in the current file stay unchanged.

- [ ] **Step 4: Run — expect PASS**

Run: `go test ./internal/ui/tui/ -run 'TestOpenMetrics|TestHandleOpenMetricsTrigger|TestMetricsURL_Build|TestCapabilityForModel|TestCatalogLoaded|TestOpenDacMetrics' -count=1`
Expected: `ok`.

- [ ] **Step 5: Build the whole module and run the TUI package**

Run: `go build ./... && go test ./internal/ui/tui/ -count=1`
Expected: `ok`.

- [ ] **Step 6: Commit**

```bash
git add internal/ui/tui/reducer_actions.go internal/ui/tui/dac_metrics_test.go internal/ui/tui/model_capability_test.go
git commit -m "refactor(tui): generalize metrics flow to Filter/plan pipeline"
```

---

### Task 4: TUI — ImportedModel & GPUWorkload resolution, new capabilities, keybindings

**Files:**
- Modify: `internal/ui/tui/reducer_actions.go` (`capabilityForModel`, `metricsCatalog`, `resolveMetricsPlan`)
- Modify: `internal/ui/tui/keys/registry.go:276-284` (catContext for GPUWorkload, ImportedModel)
- Test: `internal/ui/tui/model_capability_test.go`, `internal/ui/tui/metrics_resolve_test.go` (create)

**Interfaces:**
- Consumes: Task 3 (`resolveMetricsPlan`, `metricsCatalog`, `dedicatedPlan`, `modelCatalog`, `openMetrics`), Task 2 (`models.CapabilityTextClassification`, `models.CapabilityImageContentModeration`, `Dataset.FindBaseModelByName`), Task 1 (`telemetry.CapabilityTextClassification`, `telemetry.CapabilityImageContentModeration`, `telemetry.FilterResourceId`).
- Produces: `<m>` reachable on GPUWorkload and ImportedModel list views.

- [ ] **Step 1: Write failing tests for capability precedence and the new resolution cases**

Append to `internal/ui/tui/model_capability_test.go` (extend `TestCapabilityForModel`'s table by adding cases — add these rows to the existing `cases` slice):

```go
		{"classification", &models.BaseModel{Capabilities: []string{"TEXT_CLASSIFICATION"}}, telemetry.CapabilityTextClassification},
		{"imagemod", &models.BaseModel{Capabilities: []string{"IMAGE_CONTENT_MODERATION"}}, telemetry.CapabilityImageContentModeration},
		{"chat-over-classification", &models.BaseModel{Capabilities: []string{"TEXT_CLASSIFICATION", "CHAT"}}, telemetry.CapabilityChat},
		{"embed-over-classification", &models.BaseModel{Capabilities: []string{"TEXT_CLASSIFICATION", "TEXT_EMBEDDINGS"}}, telemetry.CapabilityTextEmbeddings},
		{"classification-over-imagemod", &models.BaseModel{Capabilities: []string{"IMAGE_CONTENT_MODERATION", "TEXT_CLASSIFICATION"}}, telemetry.CapabilityTextClassification},
```

Create `internal/ui/tui/metrics_resolve_test.go`:

```go
package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/infra/telemetry"
	"github.com/jingle2008/toolkit/pkg/models"
)

func newResolveModel(t *testing.T) *Model {
	t.Helper()
	m := makeTestModel()
	m.environment = models.Environment{Realm: "oc1", Region: "me-abudhabi-1", Type: "prod"}
	m.dataset = &models.Dataset{
		BaseModels: []models.BaseModel{
			{Name: "gpt", DisplayName: "openai.gpt-5.5", Capabilities: []string{"CHAT"}},
			{Name: "mod", DisplayName: "openai.mod", Capabilities: []string{"TEXT_CLASSIFICATION"}},
		},
	}
	return m
}

func TestResolveMetricsPlan_ImportedModelDedicated(t *testing.T) {
	t.Parallel()
	m := newResolveModel(t)
	im := &models.ImportedModel{
		BaseModel: models.BaseModel{Name: "amaaaaaaim", Capabilities: []string{"TEXT_RERANK"}},
		Namespace: "amaaaaaadac1",
	}
	filter, cap, ok, reason := m.resolveMetricsPlan(im)
	require.True(t, ok, reason)
	assert.Equal(t, telemetry.FilterDacId, filter.Key)
	assert.Equal(t, "ocid1.generativeaidedicatedaicluster.oc1.me-abudhabi-1.amaaaaaadac1", filter.Value)
	assert.Equal(t, telemetry.CapabilityTextRerank, cap)
}

func TestResolveMetricsPlan_ImportedModelNotADAC(t *testing.T) {
	t.Parallel()
	m := newResolveModel(t)
	im := &models.ImportedModel{BaseModel: models.BaseModel{Name: "x"}, Namespace: "team-x"}
	_, _, ok, reason := m.resolveMetricsPlan(im)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestResolveMetricsPlan_WorkloadEmptyModel(t *testing.T) {
	t.Parallel()
	m := newResolveModel(t)
	_, _, ok, reason := m.resolveMetricsPlan(&models.GPUWorkload{Name: "p", Namespace: "amaaaaaadac1"})
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestResolveMetricsPlan_WorkloadDedicated(t *testing.T) {
	t.Parallel()
	m := newResolveModel(t)
	w := &models.GPUWorkload{Name: "p", Namespace: "amaaaaaadac1", Model: "gpt"}
	filter, cap, ok, reason := m.resolveMetricsPlan(w)
	require.True(t, ok, reason)
	assert.Equal(t, telemetry.FilterDacId, filter.Key)
	assert.Equal(t, "ocid1.generativeaidedicatedaicluster.oc1.me-abudhabi-1.amaaaaaadac1", filter.Value)
	assert.Equal(t, telemetry.CapabilityChat, cap)
}

func TestResolveMetricsPlan_WorkloadOnDemand(t *testing.T) {
	t.Parallel()
	m := newResolveModel(t)
	w := &models.GPUWorkload{Name: "p", Namespace: "team-x", Model: "gpt"}
	filter, cap, ok, reason := m.resolveMetricsPlan(w)
	require.True(t, ok, reason)
	assert.Equal(t, telemetry.FilterResourceId, filter.Key)
	assert.Equal(t, "openai.gpt-5.5", filter.Value)
	assert.Equal(t, telemetry.CapabilityChat, cap)
}

func TestResolveMetricsPlan_OnDemandClassification(t *testing.T) {
	t.Parallel()
	m := newResolveModel(t)
	w := &models.GPUWorkload{Name: "p", Namespace: "team-x", Model: "mod"}
	filter, cap, ok, reason := m.resolveMetricsPlan(w)
	require.True(t, ok, reason)
	assert.Equal(t, telemetry.FilterResourceId, filter.Key)
	assert.Equal(t, telemetry.CapabilityTextClassification, cap)
}

func TestResolveMetricsPlan_OnDemandModelNotFound(t *testing.T) {
	t.Parallel()
	m := newResolveModel(t)
	w := &models.GPUWorkload{Name: "p", Namespace: "team-x", Model: "missing"}
	_, _, ok, reason := m.resolveMetricsPlan(w)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestResolveMetricsPlan_DedicatedClassificationNoOp(t *testing.T) {
	t.Parallel()
	m := newResolveModel(t)
	// Dedicated workload whose model resolves to a content-moderation
	// capability is unreachable in dedicated mode → toast, no open.
	w := &models.GPUWorkload{Name: "p", Namespace: "amaaaaaadac1", Model: "mod"}
	_, _, ok, reason := m.resolveMetricsPlan(w)
	assert.False(t, ok)
	assert.NotEmpty(t, reason)
}

func TestMetricsCatalog_NewCategories(t *testing.T) {
	t.Parallel()
	m := newResolveModel(t)
	_, need := m.metricsCatalog(&models.ImportedModel{Namespace: "amaaaaaadac1"})
	assert.False(t, need, "imported model carries its own capabilities")

	cat, need := m.metricsCatalog(&models.GPUWorkload{Namespace: "team-x", Model: "gpt"})
	assert.True(t, need)
	assert.Equal(t, domain.BaseModel, cat, "on-demand matches the base catalog")

	cat, need = m.metricsCatalog(&models.GPUWorkload{Namespace: "amaaaaaadac1", Model: "amaaaaaaimp"})
	assert.True(t, need)
	assert.Equal(t, domain.ImportedModel, cat, "dedicated workload with an imported model name")

	_, need = m.metricsCatalog(&models.GPUWorkload{Namespace: "team-x"}) // empty model
	assert.False(t, need)
}

func TestKeys_OpenMetricsOnNewCategories(t *testing.T) {
	t.Parallel()
	for _, cat := range []domain.Category{domain.GPUWorkload, domain.ImportedModel, domain.DedicatedAICluster} {
		assert.Contains(t, keyHelpDescs(cat), "Open Metrics", "category %v", cat)
	}
}
```

For `keyHelpDescs`, use the existing keys-registry resolver. If no helper exists, inline the check in the test:

```go
func keyHelpDescs(cat domain.Category) []string {
	var out []string
	for _, b := range keys.ResolveKeys(cat, common.ListView).Context {
		out = append(out, b.Help().Desc)
	}
	return out
}
```

(Place `keyHelpDescs` in `metrics_resolve_test.go`; add imports `keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"` and `"github.com/jingle2008/toolkit/internal/ui/tui/common"`. Verify `ResolveKeys` returns a value exposing `.Context []key.Binding` — it is used in `reducer_category.go:39` as `keys.ResolveKeys(m.category, m.viewMode)` assigned to `m.keys`, and `m.keys.Context` is iterated in `handleAdditionalKeys`.)

- [ ] **Step 2: Run — expect FAIL**

Run: `go test ./internal/ui/tui/ -run 'TestResolveMetricsPlan|TestMetricsCatalog_NewCategories|TestCapabilityForModel|TestKeys_OpenMetricsOnNewCategories' -count=1`
Expected: FAIL — new caps not returned by `capabilityForModel`; ImportedModel/GPUWorkload return silent no-op; `<m>` not bound on the new categories.

- [ ] **Step 3: Extend `capabilityForModel`**

In `internal/ui/tui/reducer_actions.go`, replace the `capabilityForModel` switch body so the precedence chain includes the two new capabilities and update the doc comment:

```go
// capabilityForModel maps a resolved model to its metric capability,
// defaulting to chat for nil/finetune/unrecognized. Precedence:
// CHAT > TEXT_RERANK > TEXT_EMBEDDINGS > TEXT_CLASSIFICATION >
// IMAGE_CONTENT_MODERATION.
func capabilityForModel(model *models.BaseModel) telemetry.Capability {
	if model == nil || model.Type == "Fine-tuning" {
		return telemetry.CapabilityChat
	}
	switch {
	case model.HasCapability(models.CapabilityChat):
		return telemetry.CapabilityChat
	case model.HasCapability(models.CapabilityTextRerank):
		return telemetry.CapabilityTextRerank
	case model.HasCapability(models.CapabilityTextEmbeddings):
		return telemetry.CapabilityTextEmbeddings
	case model.HasCapability(models.CapabilityTextClassification):
		return telemetry.CapabilityTextClassification
	case model.HasCapability(models.CapabilityImageContentModeration):
		return telemetry.CapabilityImageContentModeration
	default:
		return telemetry.CapabilityChat
	}
}
```

- [ ] **Step 4: Add the ImportedModel and GPUWorkload cases to `metricsCatalog`**

Replace `metricsCatalog` with:

```go
func (m *Model) metricsCatalog(item any) (domain.Category, bool) {
	switch it := item.(type) {
	case *models.DedicatedAICluster:
		if it == nil || it.ModelName == "" {
			return domain.BaseModel, false
		}
		return modelCatalog(it.ModelName), true
	case *models.GPUWorkload:
		if it == nil || it.Model == "" {
			return domain.BaseModel, false
		}
		if strings.HasPrefix(it.Namespace, importedModelNamePrefix) {
			return modelCatalog(it.Model), true // dedicated
		}
		return domain.BaseModel, true // on-demand matches the base catalog
	default: // *models.ImportedModel (capabilities inline) or unknown
		return domain.BaseModel, false
	}
}
```

- [ ] **Step 5: Add the ImportedModel and GPUWorkload cases to `resolveMetricsPlan`**

Replace `resolveMetricsPlan` with:

```go
func (m *Model) resolveMetricsPlan(item any) (telemetry.Filter, telemetry.Capability, bool, string) {
	realm, region := m.environment.Realm, m.environment.Region
	switch it := item.(type) {
	case *models.DedicatedAICluster:
		if it == nil {
			return telemetry.Filter{}, 0, false, ""
		}
		filter := telemetry.Filter{Key: telemetry.FilterDacId, Value: it.OCID(realm, region)}
		if it.ModelName == "" {
			return filter, telemetry.CapabilityChat, true, ""
		}
		return m.dedicatedPlan(filter, m.dataset.FindModelByName(it.ModelName))
	case *models.ImportedModel:
		if it == nil {
			return telemetry.Filter{}, 0, false, ""
		}
		if !strings.HasPrefix(it.Namespace, importedModelNamePrefix) {
			return telemetry.Filter{}, 0, false, "imported model is not tied to a dedicated AI cluster"
		}
		ocid := models.DedicatedAICluster{Name: it.Namespace}.OCID(realm, region)
		filter := telemetry.Filter{Key: telemetry.FilterDacId, Value: ocid}
		return m.dedicatedPlan(filter, &it.BaseModel)
	case *models.GPUWorkload:
		if it == nil {
			return telemetry.Filter{}, 0, false, ""
		}
		if it.Model == "" {
			return telemetry.Filter{}, 0, false, "workload has no model"
		}
		if strings.HasPrefix(it.Namespace, importedModelNamePrefix) {
			ocid := models.DedicatedAICluster{Name: it.Namespace}.OCID(realm, region)
			filter := telemetry.Filter{Key: telemetry.FilterDacId, Value: ocid}
			return m.dedicatedPlan(filter, m.dataset.FindModelByName(it.Model))
		}
		bm := m.dataset.FindBaseModelByName(it.Model)
		if bm == nil {
			return telemetry.Filter{}, 0, false, "model not found in base catalog"
		}
		return telemetry.Filter{Key: telemetry.FilterResourceId, Value: bm.DisplayName}, capabilityForModel(bm), true, ""
	default:
		return telemetry.Filter{}, 0, false, ""
	}
}
```

- [ ] **Step 6: Bind `<m>` on the two new categories**

In `internal/ui/tui/keys/registry.go`, update the two catContext entries:

```go
	domain.GPUWorkload: {
		common.ListView: {Parent, SortTenant, SortAge, OpenMetrics, ToggleFaulty, Refresh},
	},
```

```go
	domain.ImportedModel: {
		common.ListView: {Parent, SortTenant, SortSize, SortContext, SortVendor, CopyTenant, EditTenant, OpenMetrics, Refresh},
	},
```

- [ ] **Step 7: Run — expect PASS**

Run: `go test ./internal/ui/tui/ -run 'TestResolveMetricsPlan|TestMetricsCatalog_NewCategories|TestCapabilityForModel|TestKeys_OpenMetricsOnNewCategories|TestOpenMetrics|TestHandleOpenMetricsTrigger' -count=1`
Expected: `ok`.

- [ ] **Step 8: Full CI**

Run: `make ci`
Expected: exit 0 (all packages `ok`, lint clean, coverage gate ≥ 80%). If `golangci-lint` flags `cyclop` on `resolveMetricsPlan`, add a single `//nolint:cyclop // per-item-type metrics resolution; the switch is the routing surface` directive on the function.

- [ ] **Step 9: Commit**

```bash
git add internal/ui/tui/reducer_actions.go internal/ui/tui/keys/registry.go internal/ui/tui/model_capability_test.go internal/ui/tui/metrics_resolve_test.go
git commit -m "feat(tui): metrics shortcut for ImportedModel and GPUWorkload"
```

---

## Self-Review

**Spec coverage:**
- Rule 1 (ImportedModel namespace=DAC) → Task 4 `resolveMetricsPlan` ImportedModel case + `TestResolveMetricsPlan_ImportedModelDedicated`.
- Rule 2 (GPUWorkload empty / dedicated / on-demand) → Task 4 GPUWorkload case + the five workload tests.
- ResourceId filter + DisplayName → Task 1 `Filter` + Task 4 on-demand branch + `TestResolveMetricsPlan_WorkloadOnDemand`.
- Two new capabilities (fixed/unfiltered) → Task 1 `metricQueries` + tests; Task 2 consts; Task 4 `capabilityForModel`.
- Decisions: dedicated classification no-op (Task 4 `dedicatedPlan` + `TestResolveMetricsPlan_DedicatedClassificationNoOp`); error-toast on unresolvable (Task 3 `finishMetrics` + Task 4 toast tests); GPUWorkload.Model as dedicated capability source (Task 4 GPUWorkload dedicated branch); precedence (Task 4 `capabilityForModel` + `TestCapabilityForModel` rows); keybindings (Task 4 Step 6 + `TestKeys_OpenMetricsOnNewCategories`).

**Placeholder scan:** none — every code step shows complete code.

**Type consistency:** `Filter{Key,Value}`, `FilterDacId`/`FilterResourceId`, `MetricsURL(filter, capability, …)`, `metricsURL(env, filter, capability, now)`, `openMetricsTriggerMsg{item, cat, gen}`, `resolveMetricsPlan(item) (telemetry.Filter, telemetry.Capability, bool, string)`, `dedicatedPlan(filter, model)`, `metricsCatalog(item) (domain.Category, bool)`, `modelCatalog(name) domain.Category` — consistent across Tasks 1, 3, 4. `FindBaseModelByName` (Task 2) used only in Task 4. The two new `telemetry.Capability` and `models.Capability*` consts are defined in Tasks 1–2 and consumed in Task 4.
