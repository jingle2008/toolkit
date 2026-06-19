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
	got := metricQueries(testOCID)
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
const wantZipson = `{¨panels¨|{¨legendType¨¢1¨queries¨|{¨regionId¨¨me-abudhabi-1¨¨project¨¨GenerativeAIService¨¨fleet¨¨generative-ai-service-api-prod¨¨tql¨¨GenerativeAiService.chat.InputTokenLength[1m]{DacId = "ocid1.generativeaidedicatedaicluster.oc1.me-abudhabi-1.amaaaaaatestdac"}.grouping().sum()¨¨visible¨»¨expanded¨«}{ß3ß4ß5ß6ß7ß8ß9¨GenerativeAiService.chat.OutputTokenLength[1m]{DacId = "ocid1.generativeaidedicatedaicluster.oc1.me-abudhabi-1.amaaaaaatestdac"}.grouping().sum()¨ßB»ßC«}{ß3ß4ß5ß6ß7ß8ß9¨GenerativeAiService.chat.ReasoningTokenLength[1m]{DacId = "ocid1.generativeaidedicatedaicluster.oc1.me-abudhabi-1.amaaaaaatestdac"}.grouping().sum()¨ßB»ßC«}{ß3ß4ß5ß6ß7ß8ß9¨GenerativeAiService.chatCompletions.InputTokenLength[1m]{DacId = "ocid1.generativeaidedicatedaicluster.oc1.me-abudhabi-1.amaaaaaatestdac"}.grouping().sum()¨ßB»ßC«}{ß3ß4ß5ß6ß7ß8ß9¨GenerativeAiService.chatCompletions.OutputTokenLength[1m]{DacId = "ocid1.generativeaidedicatedaicluster.oc1.me-abudhabi-1.amaaaaaatestdac"}.grouping().sum()¨ßB»ßC«}{ß3ß4ß5ß6ß7ß8ß9¨GenerativeAiService.chatCompletions.ReasoningTokenLength[1m]{DacId = "ocid1.generativeaidedicatedaicluster.oc1.me-abudhabi-1.amaaaaaatestdac"}.grouping().sum()¨ßB»ßC«}{ß3ß4ß5ß6ß7ß8ß9¨GenerativeAiService.responses.InputTokenLength[1m]{DacId = "ocid1.generativeaidedicatedaicluster.oc1.me-abudhabi-1.amaaaaaatestdac"}.grouping().sum()¨ßB»ßC«}{ß3ß4ß5ß6ß7ß8ß9¨GenerativeAiService.responses.OutputTokenLength[1m]{DacId = "ocid1.generativeaidedicatedaicluster.oc1.me-abudhabi-1.amaaaaaatestdac"}.grouping().sum()¨ßB»ßC«}{ß3ß4ß5ß6ß7ß8ß9¨GenerativeAiService.responses.ReasoningTokenLength[1m]{DacId = "ocid1.generativeaidedicatedaicluster.oc1.me-abudhabi-1.amaaaaaatestdac"}.grouping().sum()¨ßB»ßC«}÷}÷¨searchPanelState¨{ß3ß4ß5ß6}¨layout¨¨full¨¨startMs¨¢VMtrWcG¨endMs¨¢VMwuYuC}`

func TestMetricsURL(t *testing.T) {
	t.Parallel()
	start := time.UnixMilli(1781787680652)
	end := time.UnixMilli(1781832733444)
	got := MetricsURL(testOCID, "me-abudhabi-1", "GenerativeAIService", "generative-ai-service-api-prod", start, end)

	const prefix = exploreBaseURL + "?data="
	require.True(t, strings.HasPrefix(got, prefix), "URL prefix")

	escaped := strings.TrimPrefix(got, prefix)
	unescaped, err := url.QueryUnescape(escaped)
	require.NoError(t, err)
	raw, err := base64.StdEncoding.DecodeString(unescaped)
	require.NoError(t, err)
	assert.Equal(t, wantZipson, string(raw))
}
