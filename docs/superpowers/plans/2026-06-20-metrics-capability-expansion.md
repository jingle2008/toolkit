# Metrics Capability Expansion Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Expand the metrics dashboard's capability→query mapping to the full model-capability set, with new token-length grids, a redefined embeddings set, two capability synonyms, and an "unsupported capability" error outcome.

**Architecture:** Make `telemetry.metricQueries` table-driven (`map[Capability]queryShape`), derive `Supported()`/`Filterable()` predicates from the table, and replace the TUI's `capabilityForModel` precedence switch with an ordered string→capability list. Plan finalization checks the predicates to error on unsupported (any mode) and unfilterable-in-dedicated capabilities.

**Tech Stack:** Go, Bubble Tea TUI, testify. Spec: `docs/superpowers/specs/2026-06-20-metrics-capability-expansion-design.md`.

## Global Constraints

- Token-length query format (filtered): `GenerativeAiService.<group>.<kind>TokenLength[1m]{<Key> = "<Value>"}.grouping().sum()` (spaces around `=`). Kinds: `Input`, `Output`, `Reasoning`.
- Fixed (unfiltered) query sets, verbatim:
  - TextClassification: `ContentModeration.TotalInvocation.Count[1m].grouping().sum()`
  - ImageContentModeration: `ImageContentModeration.Latency.ChatInput[1m].grouping().sum()`, `ImageContentModeration.Latency.ApplyGuardrails[1m].grouping().sum()`
- Capability→shape (groups × kinds): Chat=`chat,chatCompletions,responses`×IOR; TextToText=`chatCompletions,responses`×IOR; TextRerank=`rerankText`×Input; TextEmbeddings=`embedText,embeddings`×Input; TextToImage=`imagesGenerations`×IOR; ImageTextToImage=`imagesEdits`×IOR; TextToAudio=`audioSpeech`×IOR; AudioToText=`audioTranscriptions`×IOR.
- `Supported()` = capability present in the shape table (false for `CapabilityUnsupported`). `Filterable()` = present AND not a fixed set (false for the two moderation caps and Unsupported).
- Precedence (capabilityForModel): CHAT → TEXT_TO_TEXT → IMAGE_TEXT_TO_TEXT → TEXT_RERANK → TEXT_EMBEDDINGS → EMBEDDING → TEXT_TO_IMAGE → IMAGE_TEXT_TO_IMAGE → TEXT_TO_AUDIO → AUDIO_TO_TEXT → TEXT_CLASSIFICATION → CONTENT_MODERATION → IMAGE_CONTENT_MODERATION → TEXT_GENERATION → AUDIO_TO_AUDIO → REALTIME → PROMPT_INJECTION_PROTECTION; first declared match wins. nil / `Fine-tuning` / no recognized capability → `CapabilityChat`. The four unsupported strings are last (supported-wins).
- Error toasts (open nothing): Unsupported (any mode) → "metrics not supported for this model"; unfilterable cap in dedicated mode → "metrics not available for this model in dedicated mode".
- New `Capability` enum values are appended (existing ordinals CapabilityChat=0 … CapabilityImageContentModeration=4 unchanged).
- Run `make ci` green before the final commit of each task.

---

### Task 1: Telemetry — table-driven query shapes, new capabilities, Supported/Filterable

**Files:**
- Modify: `internal/infra/telemetry/mql.go`
- Test: `internal/infra/telemetry/mql_test.go`

**Interfaces:**
- Consumes: existing `Filter`, `Filter.suffix()`, `Encoder`, `MetricsURL`.
- Produces:
  - New `Capability` values: `CapabilityTextToText`, `CapabilityTextToImage`, `CapabilityImageTextToImage`, `CapabilityTextToAudio`, `CapabilityAudioToText`, `CapabilityUnsupported`.
  - `func (c Capability) Supported() bool`
  - `func (c Capability) Filterable() bool`
  - `metricQueries(capability Capability, filter Filter) []string` (same signature; table-driven; returns `nil` for unsupported/unknown).

- [ ] **Step 1: Update existing rerank/embed tests + add new failing tests**

In `internal/infra/telemetry/mql_test.go`:

(a) Replace `TestMetricQueries_RerankAndEmbed` with a rerank-only test (embeddings gets its own test below, since its set changed):

```go
func TestMetricQueries_Rerank(t *testing.T) {
	t.Parallel()
	assert.Equal(t, []string{
		`GenerativeAiService.rerankText.InputTokenLength[1m]{DacId = "` + testOCID + `"}.grouping().sum()`,
	}, metricQueries(CapabilityTextRerank, Filter{Key: FilterDacID, Value: testOCID}))
}
```

(b) Replace `TestMetricsURL_EmbedSingleQuery` with a two-query version:

```go
func TestMetricsURL_Embed(t *testing.T) {
	t.Parallel()
	got := MetricsURL(Filter{Key: FilterDacID, Value: testOCID}, CapabilityTextEmbeddings, "me-abudhabi-1",
		"GenerativeAIService", "generative-ai-service-api-prod",
		time.UnixMilli(1781787680652), time.UnixMilli(1781832733444))
	z := decodeData(t, got)
	assert.Contains(t, z, `GenerativeAiService.embedText.InputTokenLength[1m]{DacId = "`+testOCID+`"}.grouping().sum()`)
	assert.Contains(t, z, `GenerativeAiService.embeddings.InputTokenLength[1m]{DacId = "`+testOCID+`"}.grouping().sum()`)
	assert.NotContains(t, z, "chat.InputTokenLength")
}
```

(c) Append new tests:

```go
func TestMetricQueries_TextToText(t *testing.T) {
	t.Parallel()
	f := Filter{Key: FilterResourceID, Value: "m"}
	want := []string{
		`GenerativeAiService.chatCompletions.InputTokenLength[1m]{ResourceId = "m"}.grouping().sum()`,
		`GenerativeAiService.chatCompletions.OutputTokenLength[1m]{ResourceId = "m"}.grouping().sum()`,
		`GenerativeAiService.chatCompletions.ReasoningTokenLength[1m]{ResourceId = "m"}.grouping().sum()`,
		`GenerativeAiService.responses.InputTokenLength[1m]{ResourceId = "m"}.grouping().sum()`,
		`GenerativeAiService.responses.OutputTokenLength[1m]{ResourceId = "m"}.grouping().sum()`,
		`GenerativeAiService.responses.ReasoningTokenLength[1m]{ResourceId = "m"}.grouping().sum()`,
	}
	assert.Equal(t, want, metricQueries(CapabilityTextToText, f))
}

func TestMetricQueries_Embeddings(t *testing.T) {
	t.Parallel()
	assert.Equal(t, []string{
		`GenerativeAiService.embedText.InputTokenLength[1m]{DacId = "x"}.grouping().sum()`,
		`GenerativeAiService.embeddings.InputTokenLength[1m]{DacId = "x"}.grouping().sum()`,
	}, metricQueries(CapabilityTextEmbeddings, Filter{Key: FilterDacID, Value: "x"}))
}

func TestMetricQueries_Modalities(t *testing.T) {
	t.Parallel()
	f := Filter{Key: FilterDacID, Value: "x"}
	cases := map[Capability]string{
		CapabilityTextToImage:      "imagesGenerations",
		CapabilityImageTextToImage: "imagesEdits",
		CapabilityTextToAudio:      "audioSpeech",
		CapabilityAudioToText:      "audioTranscriptions",
	}
	for capability, group := range cases {
		want := []string{
			`GenerativeAiService.` + group + `.InputTokenLength[1m]{DacId = "x"}.grouping().sum()`,
			`GenerativeAiService.` + group + `.OutputTokenLength[1m]{DacId = "x"}.grouping().sum()`,
			`GenerativeAiService.` + group + `.ReasoningTokenLength[1m]{DacId = "x"}.grouping().sum()`,
		}
		assert.Equal(t, want, metricQueries(capability, f), "capability %d", capability)
	}
}

func TestMetricQueries_UnsupportedReturnsNil(t *testing.T) {
	t.Parallel()
	assert.Nil(t, metricQueries(CapabilityUnsupported, Filter{Key: FilterDacID, Value: "x"}))
}

func TestCapabilitySupportedAndFilterable(t *testing.T) {
	t.Parallel()
	for _, c := range []Capability{
		CapabilityChat, CapabilityTextToText, CapabilityTextRerank, CapabilityTextEmbeddings,
		CapabilityTextToImage, CapabilityImageTextToImage, CapabilityTextToAudio, CapabilityAudioToText,
	} {
		assert.True(t, c.Supported(), "supported %d", c)
		assert.True(t, c.Filterable(), "filterable %d", c)
	}
	for _, c := range []Capability{CapabilityTextClassification, CapabilityImageContentModeration} {
		assert.True(t, c.Supported(), "moderation supported %d", c)
		assert.False(t, c.Filterable(), "moderation not filterable %d", c)
	}
	assert.False(t, CapabilityUnsupported.Supported())
	assert.False(t, CapabilityUnsupported.Filterable())
}

func TestMetricsURL_TextToAudioFiltered(t *testing.T) {
	t.Parallel()
	got := MetricsURL(Filter{Key: FilterResourceID, Value: "openai.tts"}, CapabilityTextToAudio,
		"me-abudhabi-1", "GenerativeAIService", "generative-ai-service-api-prod",
		time.UnixMilli(1781787680652), time.UnixMilli(1781832733444))
	z := decodeData(t, got)
	assert.Contains(t, z, `GenerativeAiService.audioSpeech.InputTokenLength[1m]{ResourceId = "openai.tts"}.grouping().sum()`)
	assert.Contains(t, z, `GenerativeAiService.audioSpeech.ReasoningTokenLength[1m]{ResourceId = "openai.tts"}.grouping().sum()`)
}
```

- [ ] **Step 2: Run — expect FAIL/compile error**

Run: `go test ./internal/infra/telemetry/ -count=1`
Expected: build errors (new `Capability` values, `Supported`/`Filterable` undefined) and the updated embed assertion failing.

- [ ] **Step 3: Implement the table-driven telemetry**

In `internal/infra/telemetry/mql.go`, replace the `Capability` const block (lines ~20-35) with:

```go
const (
	// CapabilityChat is the default chat/chatCompletions/responses token-length
	// grid. Also the fallback for finetune / unresolved / unknown models.
	CapabilityChat Capability = iota
	CapabilityTextRerank
	CapabilityTextEmbeddings
	// CapabilityTextClassification and CapabilityImageContentModeration are
	// fixed, unfiltered content-moderation query sets.
	CapabilityTextClassification
	CapabilityImageContentModeration
	// Appended below (existing ordinals above are unchanged).
	CapabilityTextToText
	CapabilityTextToImage
	CapabilityImageTextToImage
	CapabilityTextToAudio
	CapabilityAudioToText
	// CapabilityUnsupported has no dashboard; callers must guard via Supported().
	CapabilityUnsupported
)
```

Replace the `metricGroups`/`metricKinds` vars and the `metricQueries` function (lines ~56-88) with:

```go
// queryShape describes a capability's MQL queries: a token-length grid
// (groups × kinds, filtered) or a fixed, unfiltered set.
type queryShape struct {
	groups []string // GenerativeAiService.<group>
	kinds  []string // <kind>TokenLength
	fixed  []string // verbatim, unfiltered; nil for token-length shapes
}

var (
	tokenKinds = []string{"Input", "Output", "Reasoning"}
	inputOnly  = []string{"Input"}
)

// capabilityShapes maps each supported capability to its query shape.
// CapabilityUnsupported is intentionally absent (Supported() == false).
var capabilityShapes = map[Capability]queryShape{
	CapabilityChat:             {groups: []string{"chat", "chatCompletions", "responses"}, kinds: tokenKinds},
	CapabilityTextToText:       {groups: []string{"chatCompletions", "responses"}, kinds: tokenKinds},
	CapabilityTextRerank:       {groups: []string{"rerankText"}, kinds: inputOnly},
	CapabilityTextEmbeddings:   {groups: []string{"embedText", "embeddings"}, kinds: inputOnly},
	CapabilityTextToImage:      {groups: []string{"imagesGenerations"}, kinds: tokenKinds},
	CapabilityImageTextToImage: {groups: []string{"imagesEdits"}, kinds: tokenKinds},
	CapabilityTextToAudio:      {groups: []string{"audioSpeech"}, kinds: tokenKinds},
	CapabilityAudioToText:      {groups: []string{"audioTranscriptions"}, kinds: tokenKinds},
	CapabilityTextClassification: {fixed: []string{
		`ContentModeration.TotalInvocation.Count[1m].grouping().sum()`,
	}},
	CapabilityImageContentModeration: {fixed: []string{
		`ImageContentModeration.Latency.ChatInput[1m].grouping().sum()`,
		`ImageContentModeration.Latency.ApplyGuardrails[1m].grouping().sum()`,
	}},
}

// Supported reports whether the capability has a metric dashboard at all.
// False for CapabilityUnsupported (and any capability without a shape).
func (c Capability) Supported() bool {
	_, ok := capabilityShapes[c]
	return ok
}

// Filterable reports whether the capability's queries carry the DacId/
// ResourceId filter (token-length grids). False for the fixed, unfiltered
// moderation capabilities and for unsupported capabilities. Dedicated-mode
// dashboards require a filterable capability (DacId scoping).
func (c Capability) Filterable() bool {
	shape, ok := capabilityShapes[c]
	return ok && shape.fixed == nil
}

// metricQueries returns the MQL query strings for a capability and filter.
// Fixed (moderation) shapes ignore the filter; token-length shapes apply
// filter.suffix(). Returns nil for capabilities with no shape (Unsupported);
// callers guard via Supported() before reaching this.
func metricQueries(capability Capability, filter Filter) []string {
	shape, ok := capabilityShapes[capability]
	if !ok {
		return nil
	}
	if shape.fixed != nil {
		return shape.fixed
	}
	suffix := filter.suffix()
	queries := make([]string, 0, len(shape.groups)*len(shape.kinds))
	for _, g := range shape.groups {
		for _, k := range shape.kinds {
			queries = append(queries, `GenerativeAiService.`+g+`.`+k+`TokenLength`+suffix)
		}
	}
	return queries
}
```

- [ ] **Step 4: Run — expect PASS**

Run: `go test ./internal/infra/telemetry/ -count=1`
Expected: `ok` (golden `wantZipson` chat test still byte-identical; new tests pass).

- [ ] **Step 5: Commit**

```bash
git add internal/infra/telemetry/mql.go internal/infra/telemetry/mql_test.go
git commit -m "feat(telemetry): table-driven capability query shapes + new capabilities"
```

---

### Task 2: Models — new capability string constants

**Files:**
- Modify: `pkg/models/base_model.go` (the capability const block)
- Test: `pkg/models/capability_test.go`

**Interfaces:**
- Produces: `CapabilityTextToText`, `CapabilityImageTextToText`, `CapabilityEmbedding`, `CapabilityTextToImage`, `CapabilityImageTextToImage`, `CapabilityTextToAudio`, `CapabilityAudioToText`, `CapabilityContentModeration`, `CapabilityTextGeneration`, `CapabilityAudioToAudio`, `CapabilityRealtime`, `CapabilityPromptInjectionProtection` (all `string`).

- [ ] **Step 1: Write the failing test**

Append to `pkg/models/capability_test.go`:

```go
func TestExpandedCapabilityConstants(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "TEXT_TO_TEXT", CapabilityTextToText)
	assert.Equal(t, "IMAGE_TEXT_TO_TEXT", CapabilityImageTextToText)
	assert.Equal(t, "EMBEDDING", CapabilityEmbedding)
	assert.Equal(t, "TEXT_TO_IMAGE", CapabilityTextToImage)
	assert.Equal(t, "IMAGE_TEXT_TO_IMAGE", CapabilityImageTextToImage)
	assert.Equal(t, "TEXT_TO_AUDIO", CapabilityTextToAudio)
	assert.Equal(t, "AUDIO_TO_TEXT", CapabilityAudioToText)
	assert.Equal(t, "CONTENT_MODERATION", CapabilityContentModeration)
	assert.Equal(t, "TEXT_GENERATION", CapabilityTextGeneration)
	assert.Equal(t, "AUDIO_TO_AUDIO", CapabilityAudioToAudio)
	assert.Equal(t, "REALTIME", CapabilityRealtime)
	assert.Equal(t, "PROMPT_INJECTION_PROTECTION", CapabilityPromptInjectionProtection)
}
```

- [ ] **Step 2: Run — expect FAIL**

Run: `go test ./pkg/models/ -run TestExpandedCapabilityConstants -count=1`
Expected: build error (constants undefined).

- [ ] **Step 3: Implement**

In `pkg/models/base_model.go`, extend the capability const block to:

```go
const (
	CapabilityChat                      = "CHAT"
	CapabilityTextRerank                = "TEXT_RERANK"
	CapabilityTextEmbeddings            = "TEXT_EMBEDDINGS"
	CapabilityTextClassification        = "TEXT_CLASSIFICATION"
	CapabilityImageContentModeration    = "IMAGE_CONTENT_MODERATION"
	CapabilityTextToText                = "TEXT_TO_TEXT"
	CapabilityImageTextToText           = "IMAGE_TEXT_TO_TEXT"
	CapabilityEmbedding                 = "EMBEDDING"
	CapabilityTextToImage               = "TEXT_TO_IMAGE"
	CapabilityImageTextToImage          = "IMAGE_TEXT_TO_IMAGE"
	CapabilityTextToAudio               = "TEXT_TO_AUDIO"
	CapabilityAudioToText               = "AUDIO_TO_TEXT"
	CapabilityContentModeration         = "CONTENT_MODERATION"
	CapabilityTextGeneration            = "TEXT_GENERATION"
	CapabilityAudioToAudio              = "AUDIO_TO_AUDIO"
	CapabilityRealtime                  = "REALTIME"
	CapabilityPromptInjectionProtection = "PROMPT_INJECTION_PROTECTION"
)
```

- [ ] **Step 4: Run — expect PASS**

Run: `go test ./pkg/models/ -count=1`
Expected: `ok`.

- [ ] **Step 5: Commit**

```bash
git add pkg/models/base_model.go pkg/models/capability_test.go
git commit -m "feat(models): add expanded model-capability string constants"
```

---

### Task 3: TUI — precedence list, expanded mapping, unsupported/dedicated error handling

**Files:**
- Modify: `internal/ui/tui/reducer_actions.go` (`capabilityForModel`, `dedicatedPlan`, the GPUWorkload on-demand branch of `resolveMetricsPlan`)
- Test: `internal/ui/tui/model_capability_test.go`, `internal/ui/tui/metrics_resolve_test.go`

**Interfaces:**
- Consumes: Task 1 (`telemetry.CapabilityTextToText`/`…TextToImage`/`…ImageTextToImage`/`…TextToAudio`/`…AudioToText`/`…Unsupported`, `Capability.Supported()`, `Capability.Filterable()`), Task 2 (the new `models.Capability*` consts).
- Produces: full capability coverage in the metrics resolution pipeline.

- [ ] **Step 1: Write the failing tests**

(a) In `internal/ui/tui/model_capability_test.go`, add these rows to the existing `TestCapabilityForModel` `cases` slice:

```go
		{"text_to_text", &models.BaseModel{Capabilities: []string{"TEXT_TO_TEXT"}}, telemetry.CapabilityTextToText},
		{"image_text_to_text", &models.BaseModel{Capabilities: []string{"IMAGE_TEXT_TO_TEXT"}}, telemetry.CapabilityTextToText},
		{"chat-over-t2t", &models.BaseModel{Capabilities: []string{"TEXT_TO_TEXT", "CHAT"}}, telemetry.CapabilityChat},
		{"embedding-synonym", &models.BaseModel{Capabilities: []string{"EMBEDDING"}}, telemetry.CapabilityTextEmbeddings},
		{"content-moderation-synonym", &models.BaseModel{Capabilities: []string{"CONTENT_MODERATION"}}, telemetry.CapabilityTextClassification},
		{"text_to_image", &models.BaseModel{Capabilities: []string{"TEXT_TO_IMAGE"}}, telemetry.CapabilityTextToImage},
		{"image_text_to_image", &models.BaseModel{Capabilities: []string{"IMAGE_TEXT_TO_IMAGE"}}, telemetry.CapabilityImageTextToImage},
		{"text_to_audio", &models.BaseModel{Capabilities: []string{"TEXT_TO_AUDIO"}}, telemetry.CapabilityTextToAudio},
		{"audio_to_text", &models.BaseModel{Capabilities: []string{"AUDIO_TO_TEXT"}}, telemetry.CapabilityAudioToText},
		{"unsupported-only", &models.BaseModel{Capabilities: []string{"REALTIME"}}, telemetry.CapabilityUnsupported},
		{"supported-wins", &models.BaseModel{Capabilities: []string{"TEXT_GENERATION", "CHAT"}}, telemetry.CapabilityChat},
```

(b) In `internal/ui/tui/metrics_resolve_test.go`, extend `newResolveModel`'s dataset with two more base models (add to the `BaseModels` slice in that helper):

```go
			{Name: "gen", DisplayName: "openai.gen", Capabilities: []string{"TEXT_GENERATION"}},
			{Name: "img", DisplayName: "openai.img", Capabilities: []string{"TEXT_TO_IMAGE"}},
```

Then append these tests:

```go
func TestResolveMetricsPlan_UnsupportedOnDemand(t *testing.T) {
	t.Parallel()
	m := newResolveModel(t)
	w := &models.GPUWorkload{Name: "p", Namespace: "team-x", Model: "gen"}
	_, _, ok, reason := m.resolveMetricsPlan(w)
	assert.False(t, ok)
	assert.Equal(t, "metrics not supported for this model", reason)
}

func TestResolveMetricsPlan_UnsupportedDedicated(t *testing.T) {
	t.Parallel()
	m := newResolveModel(t)
	// Dedicated workload whose model resolves to an unsupported capability.
	w := &models.GPUWorkload{Name: "p", Namespace: "amaaaaaadac1", Model: "gen"}
	_, _, ok, reason := m.resolveMetricsPlan(w)
	assert.False(t, ok)
	assert.Equal(t, "metrics not supported for this model", reason)
}

func TestResolveMetricsPlan_NewModalityOnDemand(t *testing.T) {
	t.Parallel()
	m := newResolveModel(t)
	w := &models.GPUWorkload{Name: "p", Namespace: "team-x", Model: "img"}
	filter, capability, ok, reason := m.resolveMetricsPlan(w)
	require.True(t, ok, reason)
	assert.Equal(t, telemetry.FilterResourceID, filter.Key)
	assert.Equal(t, "openai.img", filter.Value)
	assert.Equal(t, telemetry.CapabilityTextToImage, capability)
}

func TestResolveMetricsPlan_DedicatedModerationStillErrors(t *testing.T) {
	t.Parallel()
	m := newResolveModel(t)
	// "mod" has TEXT_CLASSIFICATION (unfilterable) → dedicated mode rejects.
	w := &models.GPUWorkload{Name: "p", Namespace: "amaaaaaadac1", Model: "mod"}
	_, _, ok, reason := m.resolveMetricsPlan(w)
	assert.False(t, ok)
	assert.Equal(t, "metrics not available for this model in dedicated mode", reason)
}
```

- [ ] **Step 2: Run — expect FAIL**

Run: `go test ./internal/ui/tui/ -run 'TestCapabilityForModel|TestResolveMetricsPlan' -count=1`
Expected: FAIL — new capability values unmapped (`capabilityForModel` returns Chat for them), unsupported not detected, dedicated-moderation message mismatch.

- [ ] **Step 3: Replace `capabilityForModel` with a precedence list**

In `internal/ui/tui/reducer_actions.go`, replace the entire `capabilityForModel` function with:

```go
// capabilityPrecedence maps a model-capability string to its telemetry
// capability, in priority order: the first capability the model declares
// wins. CHAT is highest; the four unsupported capabilities are last, so any
// supported capability outranks them (a purely-unsupported model resolves to
// CapabilityUnsupported). Synonyms (EMBEDDING, CONTENT_MODERATION,
// IMAGE_TEXT_TO_TEXT) map to the same telemetry capability as their primary.
var capabilityPrecedence = []struct {
	flag string
	cap  telemetry.Capability
}{
	{models.CapabilityChat, telemetry.CapabilityChat},
	{models.CapabilityTextToText, telemetry.CapabilityTextToText},
	{models.CapabilityImageTextToText, telemetry.CapabilityTextToText},
	{models.CapabilityTextRerank, telemetry.CapabilityTextRerank},
	{models.CapabilityTextEmbeddings, telemetry.CapabilityTextEmbeddings},
	{models.CapabilityEmbedding, telemetry.CapabilityTextEmbeddings},
	{models.CapabilityTextToImage, telemetry.CapabilityTextToImage},
	{models.CapabilityImageTextToImage, telemetry.CapabilityImageTextToImage},
	{models.CapabilityTextToAudio, telemetry.CapabilityTextToAudio},
	{models.CapabilityAudioToText, telemetry.CapabilityAudioToText},
	{models.CapabilityTextClassification, telemetry.CapabilityTextClassification},
	{models.CapabilityContentModeration, telemetry.CapabilityTextClassification},
	{models.CapabilityImageContentModeration, telemetry.CapabilityImageContentModeration},
	{models.CapabilityTextGeneration, telemetry.CapabilityUnsupported},
	{models.CapabilityAudioToAudio, telemetry.CapabilityUnsupported},
	{models.CapabilityRealtime, telemetry.CapabilityUnsupported},
	{models.CapabilityPromptInjectionProtection, telemetry.CapabilityUnsupported},
}

// capabilityForModel maps a resolved model to its metric capability via
// capabilityPrecedence (first declared match wins). nil / finetune / a model
// declaring no recognized capability fall back to CapabilityChat.
func capabilityForModel(model *models.BaseModel) telemetry.Capability {
	if model == nil || model.Type == "Fine-tuning" {
		return telemetry.CapabilityChat
	}
	for _, p := range capabilityPrecedence {
		if model.HasCapability(p.flag) {
			return p.cap
		}
	}
	return telemetry.CapabilityChat
}
```

- [ ] **Step 4: Update `dedicatedPlan` to check Supported/Filterable**

Replace the entire `dedicatedPlan` function with:

```go
// dedicatedPlan finalizes a DacId-filtered (dedicated-mode) plan. A capability
// with no dashboard at all is unsupported; an unfilterable capability (the
// fixed, unfiltered moderation sets) cannot be DacId-scoped, so it has no
// dedicated-mode dashboard. Either yields a no-op error toast.
func (m *Model) dedicatedPlan(filter telemetry.Filter, model *models.BaseModel) (telemetry.Filter, telemetry.Capability, bool, string) {
	capability := capabilityForModel(model)
	switch {
	case !capability.Supported():
		return telemetry.Filter{}, telemetry.CapabilityChat, false, "metrics not supported for this model"
	case !capability.Filterable():
		return telemetry.Filter{}, telemetry.CapabilityChat, false, "metrics not available for this model in dedicated mode"
	default:
		return filter, capability, true, ""
	}
}
```

- [ ] **Step 5: Add the unsupported check to the on-demand branch**

In `resolveMetricsPlan`'s `*models.GPUWorkload` case, replace the on-demand tail (the lines after the dedicated-namespace `if` block) with:

```go
		bm := m.dataset.FindBaseModelByName(it.Model)
		if bm == nil {
			return telemetry.Filter{}, telemetry.CapabilityChat, false, "model not found in base catalog"
		}
		capability := capabilityForModel(bm)
		if !capability.Supported() {
			return telemetry.Filter{}, telemetry.CapabilityChat, false, "metrics not supported for this model"
		}
		return telemetry.Filter{Key: telemetry.FilterResourceID, Value: bm.DisplayName}, capability, true, ""
```

- [ ] **Step 6: Run — expect PASS**

Run: `go test ./internal/ui/tui/ -run 'TestCapabilityForModel|TestResolveMetricsPlan|TestFinishMetrics|TestMetricsCatalog' -count=1`
Expected: `ok`.

- [ ] **Step 7: Build + full CI**

Run: `go build ./... && make ci`
Expected: build clean; `make ci` exit 0 (all packages `ok`, lint clean, coverage ≥ 80%). If `golangci-lint` newly flags anything on the changed functions, address it minimally (e.g. a justified `//nolint` only if the lint is a false positive for a router/table).

- [ ] **Step 8: Commit**

```bash
git add internal/ui/tui/reducer_actions.go internal/ui/tui/model_capability_test.go internal/ui/tui/metrics_resolve_test.go
git commit -m "feat(tui): full capability coverage with unsupported + dedicated-mode errors"
```

---

## Self-Review

**Spec coverage:**
- New token grids (TextToText, the four modalities) + filtered → Task 1 table + `TestMetricQueries_TextToText`/`_Modalities`/`_TextToAudioFiltered`.
- Embeddings redefinition (embedText + embeddings) → Task 1 table + `TestMetricQueries_Embeddings`, `TestMetricsURL_Embed`.
- Fixed moderation sets unchanged, unfiltered → preserved in table; `Filterable()==false` asserted in `TestCapabilitySupportedAndFilterable`.
- Unsupported capabilities → `CapabilityUnsupported` (no shape) + `metricQueries==nil` + `Supported()==false`; precedence list maps the four strings; errors in both modes (Task 3 `dedicatedPlan` + on-demand check; `TestResolveMetricsPlan_Unsupported{OnDemand,Dedicated}`).
- Synonyms (EMBEDDING, IMAGE_TEXT_TO_TEXT, CONTENT_MODERATION) → precedence list + `TestCapabilityForModel` rows.
- CHAT-wins / supported-wins → precedence ordering + `chat-over-t2t` / `supported-wins` rows.
- Two error messages → Task 3 exact strings + resolve tests; dedicated-moderation message clarified (`TestResolveMetricsPlan_DedicatedModerationStillErrors`).

**Placeholder scan:** none — all steps contain complete code.

**Type consistency:** `queryShape{groups,kinds,fixed}`, `capabilityShapes map[Capability]queryShape`, `Supported()`/`Filterable()`, `capabilityPrecedence []struct{flag string; cap telemetry.Capability}`, the new `telemetry.Capability*` and `models.Capability*` names are consistent across Tasks 1–3. The four-tuple return of `resolveMetricsPlan`/`dedicatedPlan` (`telemetry.Filter, telemetry.Capability, bool, string`) is unchanged. `newResolveModel` extension adds models used only by the new Task 3 tests.
