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

// Capability selects which metric set a DAC's dashboard shows, derived
// from the capability of the model the DAC serves.
type Capability int

const (
	// CapabilityChat is the default — the chat/chatCompletions/responses
	// token-length grid. Also used as the fallback for finetune models,
	// unresolved models, and unknown capabilities.
	CapabilityChat Capability = iota
	// CapabilityTextRerank is a single rerankText query.
	CapabilityTextRerank
	// CapabilityTextEmbeddings is a single embedText query.
	CapabilityTextEmbeddings
)

// metricGroups × metricKinds form the 3×3 chat token-length metric grid.
var (
	metricGroups = []string{"chat", "chatCompletions", "responses"}
	metricKinds  = []string{"Input", "Output", "Reasoning"}
)

// metricQueries returns the MQL query strings for a DAC OCID and
// capability: the 9-query chat grid for CapabilityChat, or a single
// token-length query for rerank / embeddings.
func metricQueries(dacOCID string, capability Capability) []string {
	suffix := `[1m]{DacId = "` + dacOCID + `"}.grouping().sum()`
	switch capability {
	case CapabilityTextRerank:
		return []string{`GenerativeAiService.rerankText.InputTokenLength` + suffix}
	case CapabilityTextEmbeddings:
		return []string{`GenerativeAiService.embedText.InputTokenLength` + suffix}
	default: // CapabilityChat
		queries := make([]string, 0, len(metricGroups)*len(metricKinds))
		for _, g := range metricGroups {
			for _, k := range metricKinds {
				queries = append(queries, `GenerativeAiService.`+g+`.`+k+`TokenLength`+suffix)
			}
		}
		return queries
	}
}

// MetricsURL builds the full OCI Telemetry MQL Explore URL for a DAC: a
// Zipson dashboard payload, base64-std-encoded and URL-escaped.
func MetricsURL(dacOCID string, capability Capability, regionID, project, fleet string, start, end time.Time) string {
	var e Encoder
	e.BeginObject()
	e.Key("panels").BeginArray()
	e.BeginObject().Key("legendType").Int(1).Key("queries").BeginArray()
	for _, q := range metricQueries(dacOCID, capability) {
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
