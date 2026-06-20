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
