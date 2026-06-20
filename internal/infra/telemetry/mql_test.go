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
	got := metricQueries(CapabilityChat, Filter{Key: FilterDacId, Value: testOCID})
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
	got := MetricsURL(Filter{Key: FilterDacId, Value: testOCID}, CapabilityChat, "me-abudhabi-1", "GenerativeAIService", "generative-ai-service-api-prod", start, end)

	const prefix = exploreBaseURL + "?data="
	require.True(t, strings.HasPrefix(got, prefix), "URL prefix")

	escaped := strings.TrimPrefix(got, prefix)
	unescaped, err := url.QueryUnescape(escaped)
	require.NoError(t, err)
	raw, err := base64.StdEncoding.DecodeString(unescaped)
	require.NoError(t, err)
	assert.Equal(t, wantZipson, string(raw))
}

func TestMetricQueries_RerankAndEmbed(t *testing.T) {
	t.Parallel()
	assert.Equal(t, []string{
		`GenerativeAiService.rerankText.InputTokenLength[1m]{DacId = "` + testOCID + `"}.grouping().sum()`,
	}, metricQueries(CapabilityTextRerank, Filter{Key: FilterDacId, Value: testOCID}))
	assert.Equal(t, []string{
		`GenerativeAiService.embedText.InputTokenLength[1m]{DacId = "` + testOCID + `"}.grouping().sum()`,
	}, metricQueries(CapabilityTextEmbeddings, Filter{Key: FilterDacId, Value: testOCID}))
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
	got := MetricsURL(Filter{Key: FilterDacId, Value: testOCID}, CapabilityTextRerank, "me-abudhabi-1",
		"GenerativeAIService", "generative-ai-service-api-prod",
		time.UnixMilli(1781787680652), time.UnixMilli(1781832733444))
	z := decodeData(t, got)
	assert.Contains(t, z, `GenerativeAiService.rerankText.InputTokenLength[1m]{DacId = "`+testOCID+`"}.grouping().sum()`)
	assert.NotContains(t, z, "chat.InputTokenLength")
}

func TestMetricsURL_EmbedSingleQuery(t *testing.T) {
	t.Parallel()
	got := MetricsURL(Filter{Key: FilterDacId, Value: testOCID}, CapabilityTextEmbeddings, "me-abudhabi-1",
		"GenerativeAIService", "generative-ai-service-api-prod",
		time.UnixMilli(1781787680652), time.UnixMilli(1781832733444))
	z := decodeData(t, got)
	assert.Contains(t, z, `GenerativeAiService.embedText.InputTokenLength[1m]{DacId = "`+testOCID+`"}.grouping().sum()`)
	assert.NotContains(t, z, "chat.InputTokenLength")
}

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
