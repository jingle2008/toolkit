# Metrics Capability Expansion — Design

**Date:** 2026-06-20

**Goal:** Extend the metrics dashboard's capability→query mapping from 5 capabilities to the full model-capability set, adding new token-length grids, redefining embeddings, and introducing an "unsupported capability" error outcome.

**Builds on:** `2026-06-19-metrics-imported-gpuworkload-design.md` (the generalized `Filter`/capability pipeline) and `2026-06-19-dac-metrics-shortcut-design.md`.

---

## Background

`telemetry.metricQueries(capability, filter)` maps a `telemetry.Capability` to its MQL query set; `capabilityForModel` (TUI) maps a model's `Capabilities` strings to one `telemetry.Capability` by precedence. Today only CHAT, TEXT_RERANK, TEXT_EMBEDDINGS, TEXT_CLASSIFICATION, IMAGE_CONTENT_MODERATION are handled. This adds the remaining model capabilities.

## Capability inventory (authoritative)

Model-capability string(s) → telemetry capability → query set. Token-length queries are `GenerativeAiService.<group>.<kind>TokenLength` + the filter suffix (`[1m]{<Key> = "<Value>"}.grouping().sum()`); fixed queries are verbatim and unfiltered.

| Model capability(ies) | Telemetry capability | Groups | Kinds | Count |
|---|---|---|---|---|
| `CHAT` | Chat | `chat, chatCompletions, responses` | Input, Output, Reasoning | 9 (unchanged) |
| `TEXT_TO_TEXT`, `IMAGE_TEXT_TO_TEXT` | TextToText | `chatCompletions, responses` | Input, Output, Reasoning | 6 |
| `TEXT_RERANK` | TextRerank | `rerankText` | Input | 1 (unchanged) |
| `TEXT_EMBEDDINGS`, `EMBEDDING` | TextEmbeddings | `embedText, embeddings` | Input | 2 (**changed**) |
| `TEXT_TO_IMAGE` | TextToImage | `imagesGenerations` | Input, Output, Reasoning | 3 |
| `IMAGE_TEXT_TO_IMAGE` | ImageTextToImage | `imagesEdits` | Input, Output, Reasoning | 3 |
| `TEXT_TO_AUDIO` | TextToAudio | `audioSpeech` | Input, Output, Reasoning | 3 |
| `AUDIO_TO_TEXT` | AudioToText | `audioTranscriptions` | Input, Output, Reasoning | 3 |
| `TEXT_CLASSIFICATION`, `CONTENT_MODERATION` | TextClassification | — | — | fixed: `ContentModeration.TotalInvocation.Count[1m].grouping().sum()` |
| `IMAGE_CONTENT_MODERATION` | ImageContentModeration | — | — | fixed pair: `ImageContentModeration.Latency.ChatInput[1m].grouping().sum()`, `ImageContentModeration.Latency.ApplyGuardrails[1m].grouping().sum()` |
| `TEXT_GENERATION`, `AUDIO_TO_AUDIO`, `REALTIME`, `PROMPT_INJECTION_PROTECTION` | Unsupported | — | — | error |

All token-length grids (including the new ones and TextToText) carry the same DacId/ResourceId filter as CHAT. Only the two fixed moderation sets are unfiltered.

## Resolved decisions

- **CHAT wins** over TEXT_TO_TEXT / IMAGE_TEXT_TO_TEXT when a model declares both (CHAT is the superset; highest precedence).
- **Supported wins** over unsupported: a model declaring any supported capability resolves to that capability's dashboard; the Unsupported error fires only when the model declares *only* unsupported capabilities.
- **Filtering:** all token-length capabilities are filtered (DacId in dedicated mode, ResourceId on-demand); the fixed moderation queries ignore the filter.
- **Embeddings redefinition:** `TEXT_EMBEDDINGS` now emits `embedText.InputTokenLength` **and** `embeddings.InputTokenLength` (was `embedText` only); `EMBEDDING` is a synonym.
- **Two error outcomes**, both open nothing:
  - Unsupported capability (any mode) → toast "metrics not supported for this model".
  - Unfilterable capability (moderation/classification) in **dedicated** mode → toast "metrics not available for this model in dedicated mode" (today's dedicated no-op, message clarified). On-demand still shows the fixed moderation queries.

## Architecture

### Component 1 — Telemetry (`internal/infra/telemetry/mql.go`)

- Extend the `Capability` enum (append, preserving existing ordinals): `CapabilityTextToText`, `CapabilityTextToImage`, `CapabilityImageTextToImage`, `CapabilityTextToAudio`, `CapabilityAudioToText`, `CapabilityUnsupported`.
- Replace the `metricQueries` switch with a table:

```go
type queryShape struct {
    groups []string // GenerativeAiService.<group>
    kinds  []string // <kind>TokenLength
    fixed  []string // verbatim, unfiltered (moderation); nil for token-length shapes
}

var (
    tokenKinds = []string{"Input", "Output", "Reasoning"}
    inputOnly  = []string{"Input"}
)

var capabilityShapes = map[Capability]queryShape{
    CapabilityChat:                   {groups: []string{"chat", "chatCompletions", "responses"}, kinds: tokenKinds},
    CapabilityTextToText:             {groups: []string{"chatCompletions", "responses"}, kinds: tokenKinds},
    CapabilityTextRerank:             {groups: []string{"rerankText"}, kinds: inputOnly},
    CapabilityTextEmbeddings:         {groups: []string{"embedText", "embeddings"}, kinds: inputOnly},
    CapabilityTextToImage:            {groups: []string{"imagesGenerations"}, kinds: tokenKinds},
    CapabilityImageTextToImage:       {groups: []string{"imagesEdits"}, kinds: tokenKinds},
    CapabilityTextToAudio:            {groups: []string{"audioSpeech"}, kinds: tokenKinds},
    CapabilityAudioToText:            {groups: []string{"audioTranscriptions"}, kinds: tokenKinds},
    CapabilityTextClassification:     {fixed: []string{`ContentModeration.TotalInvocation.Count[1m].grouping().sum()`}},
    CapabilityImageContentModeration: {fixed: []string{
        `ImageContentModeration.Latency.ChatInput[1m].grouping().sum()`,
        `ImageContentModeration.Latency.ApplyGuardrails[1m].grouping().sum()`,
    }},
    // CapabilityUnsupported intentionally absent.
}
```

- `metricQueries(capability, filter)` looks up the shape: missing → `nil`; `fixed != nil` → `fixed`; else build `groups × kinds` with `filter.suffix()`.
- Two predicates derived from the table:
  - `func (c Capability) Supported() bool` — present in `capabilityShapes`.
  - `func (c Capability) Filterable() bool` — present and `fixed == nil` (gates dedicated mode).

### Component 2 — Models (`pkg/models/base_model.go`)

Add capability string constants next to the existing ones (exact values): `CapabilityTextToText = "TEXT_TO_TEXT"`, `CapabilityImageTextToText = "IMAGE_TEXT_TO_TEXT"`, `CapabilityEmbedding = "EMBEDDING"`, `CapabilityTextToImage = "TEXT_TO_IMAGE"`, `CapabilityImageTextToImage = "IMAGE_TEXT_TO_IMAGE"`, `CapabilityTextToAudio = "TEXT_TO_AUDIO"`, `CapabilityAudioToText = "AUDIO_TO_TEXT"`, `CapabilityContentModeration = "CONTENT_MODERATION"`, `CapabilityTextGeneration = "TEXT_GENERATION"`, `CapabilityAudioToAudio = "AUDIO_TO_AUDIO"`, `CapabilityRealtime = "REALTIME"`, `CapabilityPromptInjectionProtection = "PROMPT_INJECTION_PROTECTION"`.

### Component 3 — `capabilityForModel` (`internal/ui/tui/reducer_actions.go`)

Replace the precedence switch with an ordered list returning the first declared match:

```go
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

The unsupported entries are last, so a supported capability always wins; a purely-unsupported model returns `Unsupported`.

### Component 4 — Plan finalization (`internal/ui/tui/reducer_actions.go`)

- `dedicatedPlan(filter, model)`:
  - `!capability.Supported()` → toast "metrics not supported for this model"
  - `!capability.Filterable()` → toast "metrics not available for this model in dedicated mode"
  - else → ok.
- On-demand branch (GPUWorkload): after resolving `capability` from the base model, `!capability.Supported()` → toast "metrics not supported for this model"; else ok with the ResourceId filter. (Moderation/classification remain valid on-demand — their fixed queries are returned.)

The DAC `ModelName==""` path still returns CHAT directly.

## Error handling

| Condition | Mode | Result |
|---|---|---|
| Unsupported capability | any | toast "metrics not supported for this model", no open |
| Moderation/classification (unfilterable) | dedicated | toast "metrics not available for this model in dedicated mode", no open |
| Moderation/classification | on-demand | fixed unfiltered queries shown |
| Model not in base catalog (on-demand) | on-demand | toast "model not found in base catalog" (unchanged) |

## Testing

- **Telemetry:** a query test per shape — TextToText (6, filtered, no `chat.` group), TextEmbeddings (2: embedText + embeddings Input, filtered), TextToImage/ImageTextToImage/TextToAudio/AudioToText (3 each, correct group, filtered); `Supported()` true for all table caps + false for Unsupported; `Filterable()` true for token grids + false for the two moderation caps + false for Unsupported; `metricQueries(CapabilityUnsupported, …) == nil`; one `MetricsURL` round-trip for a new grid (e.g. TextToAudio) confirming the filter is applied. Update the existing embed test for the new two-query set (still excludes `chat`).
- **Models:** assert the new string constant values.
- **TUI:** `capabilityForModel` — CHAT beats TEXT_TO_TEXT; supported beats unsupported (CHAT + TEXT_GENERATION → Chat); each synonym pair maps correctly; purely-unsupported (e.g. only REALTIME) → Unsupported; unknown string → Chat; each new mapping. `resolveMetricsPlan` — an Unsupported model toasts in both dedicated and on-demand; a new modality (e.g. TEXT_TO_IMAGE base model) resolves on-demand to a ResourceId-filtered plan; dedicated moderation still toasts.

## Out of scope

- Changing TEXT_RERANK's metric set, the metric window, project, fleet derivation, or the keybindings.
- Any UI beyond the metric set selection (the resolution pipeline and entry points are unchanged).
