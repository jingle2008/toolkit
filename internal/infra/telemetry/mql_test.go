package telemetry

import (
	"encoding/base64"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testOCID = "ocid1.generativeaidedicatedaicluster.oc1.me-abudhabi-1.amaaaaaatestdac"

func TestMetricQueries(t *testing.T) {
	t.Parallel()
	got := metricQueries(CapabilityChat, Filter{Key: FilterDacID, Value: testOCID})
	want := []string{
		`GenerativeAiService.chat.InputTokenLength[1m]{DacId = "` + testOCID + `"}.grouping().sum()`,
		`GenerativeAiService.chat.OutputTokenLength[1m]{DacId = "` + testOCID + `"}.grouping().sum()`,
		`GenerativeAiService.chat.ReasoningTokenLength[1m]{DacId = "` + testOCID + `"}.grouping().sum()`,
		`GenerativeAiService.chatCompletions.InputTokenLength[1m]{DacId = "` + testOCID + `"}.grouping().sum()`,
		`GenerativeAiService.chatCompletions.OutputTokenLength[1m]{DacId = "` + testOCID + `"}.grouping().sum()`,
		`GenerativeAiService.chatCompletions.ReasoningTokenLength[1m]{DacId = "` + testOCID + `"}.grouping().sum()`,
		`GenerativeAiService.responses.InputTokenLength[1m]{DacId = "` + testOCID + `"}.grouping().sum()`,
		`GenerativeAiService.responses.OutputTokenLength[1m]{DacId = "` + testOCID + `"}.grouping().sum()`,
		`GenerativeAiService.responses.ReasoningTokenLength[1m]{DacId = "` + testOCID + `"}.grouping().sum()`,
	}
	assert.Equal(t, want, got)
}

// wantZipson is the exact Zipson payload for the fixed inputs below,
// generated and cross-checked against the OCI MQL reference encoding.
const wantZipson = `{¨panels¨|{¨legendType¨¢1¨queries¨|{¨regionId¨¨me-abudhabi-1¨¨project¨¨GenerativeAIService¨¨fleet¨¨generative-ai-service-api-prod¨¨tql¨¨GenerativeAiService.chat.InputTokenLength[1m]{DacId = "ocid1.generativeaidedicatedaicluster.oc1.me-abudhabi-1.amaaaaaatestdac"}.grouping().sum()¨¨visible¨»¨expanded¨«}{ß3ß4ß5ß6ß7ß8ß9¨GenerativeAiService.chat.OutputTokenLength[1m]{DacId = "ocid1.generativeaidedicatedaicluster.oc1.me-abudhabi-1.amaaaaaatestdac"}.grouping().sum()¨ßB»ßC«}{ß3ß4ß5ß6ß7ß8ß9¨GenerativeAiService.chat.ReasoningTokenLength[1m]{DacId = "ocid1.generativeaidedicatedaicluster.oc1.me-abudhabi-1.amaaaaaatestdac"}.grouping().sum()¨ßB»ßC«}{ß3ß4ß5ß6ß7ß8ß9¨GenerativeAiService.chatCompletions.InputTokenLength[1m]{DacId = "ocid1.generativeaidedicatedaicluster.oc1.me-abudhabi-1.amaaaaaatestdac"}.grouping().sum()¨ßB»ßC«}{ß3ß4ß5ß6ß7ß8ß9¨GenerativeAiService.chatCompletions.OutputTokenLength[1m]{DacId = "ocid1.generativeaidedicatedaicluster.oc1.me-abudhabi-1.amaaaaaatestdac"}.grouping().sum()¨ßB»ßC«}{ß3ß4ß5ß6ß7ß8ß9¨GenerativeAiService.chatCompletions.ReasoningTokenLength[1m]{DacId = "ocid1.generativeaidedicatedaicluster.oc1.me-abudhabi-1.amaaaaaatestdac"}.grouping().sum()¨ßB»ßC«}{ß3ß4ß5ß6ß7ß8ß9¨GenerativeAiService.responses.InputTokenLength[1m]{DacId = "ocid1.generativeaidedicatedaicluster.oc1.me-abudhabi-1.amaaaaaatestdac"}.grouping().sum()¨ßB»ßC«}{ß3ß4ß5ß6ß7ß8ß9¨GenerativeAiService.responses.OutputTokenLength[1m]{DacId = "ocid1.generativeaidedicatedaicluster.oc1.me-abudhabi-1.amaaaaaatestdac"}.grouping().sum()¨ßB»ßC«}{ß3ß4ß5ß6ß7ß8ß9¨GenerativeAiService.responses.ReasoningTokenLength[1m]{DacId = "ocid1.generativeaidedicatedaicluster.oc1.me-abudhabi-1.amaaaaaatestdac"}.grouping().sum()¨ßB»ßC«}÷}÷¨searchPanelState¨{ß3ß4ß5ß6ß7ß8}¨layout¨¨full¨¨startMs¨¢VMtrWcG¨endMs¨¢VMwuYuC}`

func TestMetricsURL(t *testing.T) {
	t.Parallel()
	start := time.UnixMilli(1781787680652)
	end := time.UnixMilli(1781832733444)
	got := MetricsURL(Filter{Key: FilterDacID, Value: testOCID}, CapabilityChat, "me-abudhabi-1", "GenerativeAIService", "generative-ai-service-api-prod", start, end)

	const prefix = exploreBaseURL + "?data="
	require.True(t, strings.HasPrefix(got, prefix), "URL prefix")

	escaped := strings.TrimPrefix(got, prefix)
	unescaped, err := url.QueryUnescape(escaped)
	require.NoError(t, err)
	raw, err := base64.StdEncoding.DecodeString(unescaped)
	require.NoError(t, err)
	assert.Equal(t, wantZipson, string(raw))
}

func TestMetricQueries_Rerank(t *testing.T) {
	t.Parallel()
	assert.Equal(t, []string{
		`GenerativeAiService.rerankText.InputTokenLength[1m]{DacId = "` + testOCID + `"}.grouping().sum()`,
	}, metricQueries(CapabilityTextRerank, Filter{Key: FilterDacID, Value: testOCID}))
}

// decodeData extracts and decodes the Zipson payload from a MetricsURL.
func decodeData(t *testing.T, got string) string {
	t.Helper()
	unescaped, err := url.QueryUnescape(strings.TrimPrefix(got, exploreBaseURL+"?data="))
	require.NoError(t, err)
	raw, err := base64.StdEncoding.DecodeString(unescaped)
	require.NoError(t, err)
	return string(raw)
}

func TestMetricsURL_RerankSingleQuery(t *testing.T) {
	t.Parallel()
	got := MetricsURL(Filter{Key: FilterDacID, Value: testOCID}, CapabilityTextRerank, "me-abudhabi-1",
		"GenerativeAIService", "generative-ai-service-api-prod",
		time.UnixMilli(1781787680652), time.UnixMilli(1781832733444))
	z := decodeData(t, got)
	assert.Contains(t, z, `GenerativeAiService.rerankText.InputTokenLength[1m]{DacId = "`+testOCID+`"}.grouping().sum()`)
	assert.NotContains(t, z, "chat.InputTokenLength")
}

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

func TestMetricsURL_ImageContentModerationUnfilteredRoundTrip(t *testing.T) {
	t.Parallel()
	// On-demand moderation: the ResourceId filter must be ignored, and both
	// fixed queries must survive the Zipson encode + URL round-trip.
	got := MetricsURL(Filter{Key: FilterResourceID, Value: "openai.mod"}, CapabilityImageContentModeration,
		"me-abudhabi-1", "GenerativeAIService", "generative-ai-service-api-prod",
		time.UnixMilli(1781787680652), time.UnixMilli(1781832733444))
	z := decodeData(t, got)
	assert.Contains(t, z, `ImageContentModeration.Latency.ChatInput[1m].grouping().sum()`)
	assert.Contains(t, z, `ImageContentModeration.Latency.ApplyGuardrails[1m].grouping().sum()`)
	// Each assertion proves one thing. First: no token-length metric leaked —
	// its prefix "GenerativeAiService" (lower "Ai") is case-distinct from the
	// "GenerativeAIService" project value, so this won't trip on the project.
	// Second: the ResourceId filter key is absent — that alone confirms the
	// moderation queries are unfiltered.
	assert.NotContains(t, z, "GenerativeAiService", "no token-length metric prefix in moderation queries")
	assert.NotContains(t, z, "ResourceId", "filter key absent — moderation queries are unfiltered")
}

func TestMetricQueries_ResourceIdFilter(t *testing.T) {
	t.Parallel()
	f := Filter{Key: FilterResourceID, Value: "openai.gpt-5.5"}
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
	assert.Equal(t, want, metricQueries(CapabilityTextClassification, Filter{Key: FilterResourceID, Value: "x"}))
}

func TestMetricQueries_ImageContentModerationFixedUnfiltered(t *testing.T) {
	t.Parallel()
	want := []string{
		`ImageContentModeration.Latency.ChatInput[1m].grouping().sum()`,
		`ImageContentModeration.Latency.ApplyGuardrails[1m].grouping().sum()`,
	}
	assert.Equal(t, want, metricQueries(CapabilityImageContentModeration, Filter{Key: FilterDacID, Value: "x"}))
}

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

func TestMetricQueries_FixedReturnsCopy(t *testing.T) {
	t.Parallel()
	first := metricQueries(CapabilityImageContentModeration, Filter{Key: FilterDacID, Value: "x"})
	require.NotEmpty(t, first)
	first[0] = "MUTATED"
	second := metricQueries(CapabilityImageContentModeration, Filter{Key: FilterDacID, Value: "x"})
	assert.NotEqual(t, "MUTATED", second[0], "metricQueries must not return the shared table slice")
}
