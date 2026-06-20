package telemetry

import (
	"encoding/base64"
	"net/url"
	"time"
)

// exploreBaseURL is the OCI Telemetry MQL Explore page.
const exploreBaseURL = "https://devops.oci.oraclecorp.com/telemetry/mql/explore"

// Project is the OCI Telemetry namespace (the "project" dimension) that
// scopes GenAI service metrics. Callers pass it to MetricsURL.
const Project = "GenerativeAIService"

// Capability selects which metric set a dashboard shows, derived from the
// capability of the served model.
type Capability int

const (
	// CapabilityChat is the default chat/chatCompletions/responses token-length
	// grid. Also the fallback for finetune / unresolved / unknown models.
	CapabilityChat Capability = iota
	// CapabilityTextRerank is the text re-ranking metric set.
	CapabilityTextRerank
	// CapabilityTextEmbeddings is the text embeddings metric set.
	CapabilityTextEmbeddings
	// CapabilityTextClassification is a fixed, unfiltered text content-moderation query set.
	CapabilityTextClassification
	// CapabilityImageContentModeration is a fixed, unfiltered image content-moderation query set.
	CapabilityImageContentModeration
	// CapabilityTextToText is the text-to-text (LLM inference) metric set.
	CapabilityTextToText
	// CapabilityTextToImage is the text-to-image generation metric set.
	CapabilityTextToImage
	// CapabilityImageTextToImage is the image-conditioned image generation metric set.
	CapabilityImageTextToImage
	// CapabilityTextToAudio is the text-to-audio (TTS) metric set.
	CapabilityTextToAudio
	// CapabilityAudioToText is the audio-to-text (STT/transcription) metric set.
	CapabilityAudioToText
	// CapabilityUnsupported has no dashboard; callers must guard via Supported().
	CapabilityUnsupported
)

// Filter dimension keys.
const (
	FilterDacID      = "DacId"
	FilterResourceID = "ResourceId"
)

// Filter scopes capability-driven metric queries to one resource: Key is the
// MQL dimension (FilterDacID or FilterResourceID), Value its value (a DAC
// OCID or a model display name). The two content-moderation capabilities
// ignore the filter — their queries are fixed and unfiltered.
type Filter struct {
	Key   string
	Value string
}

func (f Filter) suffix() string {
	return `[1m]{` + f.Key + ` = "` + f.Value + `"}.grouping().sum()`
}

// queryShape describes a capability's MQL queries: a token-length grid
// (groups × kinds, filtered) or a fixed, unfiltered set.
type queryShape struct {
	groups []string // GenerativeAiService.<group>
	kinds  []string // <kind>TokenLength
	// fixed holds verbatim, unfiltered queries. A non-nil fixed (even if
	// empty) marks the shape as fixed/unfiltered and makes the capability
	// non-Filterable; token-length shapes leave it nil and populate
	// groups/kinds instead.
	fixed []string
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
		return append([]string(nil), shape.fixed...)
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
