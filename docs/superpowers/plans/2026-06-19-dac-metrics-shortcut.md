# DAC "Open Metrics" Shortcut Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an `m` ("Open Metrics") shortcut to the DedicatedAICluster list view that opens an OCI Telemetry MQL Explore dashboard (9 token-length queries scoped to the selected DAC) in the browser.

**Architecture:** A new self-contained `internal/infra/telemetry` package owns a minimal Zipson encoder and a `MetricsURL` builder (Zipson → base64-std → `url.QueryEscape`). The TUI wires a new keybinding to a thin handler that builds the URL from the selected DAC + environment and launches it via the existing `actions.OpenURL` browser seam, exactly mirroring the existing `ctrl+o` "Open in portal" path.

**Tech Stack:** Go, Bubble Tea (charmbracelet), testify.

## Global Constraints

- URL: `https://devops.oci.oraclecorp.com/telemetry/mql/explore?data=<urlEncode(base64Std(zipson(state)))>`.
- Zipson tokens: string `¨…¨` (U+00A8), integer `¢`+base62 (U+00A2), bool true `»` (U+00BB) / false `«` (U+00AB), array start `|` / end `÷` (U+00F7), object `{` / `}`. No string-reference (`ß`) or small-int (`Ë`) tokens — full strings and `¢`+base62 ints are valid Zipson and decode correctly.
- base62 alphabet: `0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz`.
- Metric grid: groups `{chat, chatCompletions, responses}` × kinds `{Input, Output, Reasoning}` = 9 queries. tql template: `GenerativeAiService.<group>.<kind>TokenLength[1m]{DacId = "<ocid>"}.grouping().sum()` (note the spaces around `=`).
- Inputs: `regionId` = `environment.Region`; `dacOCID` = `dac.OCID(realm, region)`; `project` = `"GenerativeAIService"` (constant); `fleet` = `"generative-ai-service-api-" + environment.Type` (no `ppe` remap); window = `now-24h` → `now` (epoch ms).
- The encoder does NOT escape an embedded `¨`; our data never contains one.
- Follow existing patterns: testify assertions, `t.Parallel()`, table-free golden assertions where small.

---

### Task 1: base62 integer encoder

**Files:**
- Create: `internal/infra/telemetry/zipson.go`
- Test: `internal/infra/telemetry/zipson_test.go`

**Interfaces:**
- Produces: `func base62(n int64) string` (package-private), `const base62Alphabet`.

- [ ] **Step 1: Write the failing test**

```go
package telemetry

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBase62(t *testing.T) {
	t.Parallel()
	cases := map[int64]string{
		0:             "0",
		1:             "1",
		61:            "z",
		62:            "10",
		-1:            "-1",
		1781787680652: "VMtrWcG", // from the OCI MQL reference (startMs)
		1781832733444: "VMwuYuC", // from the OCI MQL reference (endMs)
	}
	for n, want := range cases {
		assert.Equal(t, want, base62(n), "base62(%d)", n)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/infra/telemetry/ -run TestBase62 -v`
Expected: FAIL — `undefined: base62`.

- [ ] **Step 3: Write minimal implementation**

```go
// Package telemetry builds OCI Telemetry MQL Explore dashboard URLs.
package telemetry

// base62Alphabet is Zipson's integer alphabet.
const base62Alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// base62 encodes n using Zipson's base62 alphabet. Negative values are
// prefixed with '-'. Used for Zipson integer tokens (e.g. epoch-ms
// timestamps).
func base62(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [16]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = base62Alphabet[n%62]
		n /= 62
	}
	s := string(buf[i:])
	if neg {
		return "-" + s
	}
	return s
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/infra/telemetry/ -run TestBase62 -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/infra/telemetry/zipson.go internal/infra/telemetry/zipson_test.go
git commit -m "feat(telemetry): add Zipson base62 integer encoder"
```

---

### Task 2: Zipson Encoder

**Files:**
- Modify: `internal/infra/telemetry/zipson.go`
- Test: `internal/infra/telemetry/zipson_test.go`

**Interfaces:**
- Consumes: `base62` (Task 1).
- Produces: `type Encoder struct{...}` with methods returning `*Encoder` for chaining: `BeginObject()`, `EndObject()`, `BeginArray()`, `EndArray()`, `Key(string)`, `Str(string)`, `Int(int64)`, `Bool(bool)`, and `String() string`.

- [ ] **Step 1: Write the failing test**

```go
func TestEncoder_Object(t *testing.T) {
	t.Parallel()
	var e Encoder
	e.BeginObject().Key("a").Str("b").EndObject()
	assert.Equal(t, "{¨a¨¨b¨}", e.String()) // {¨a¨¨b¨}
}

func TestEncoder_ArrayIntBool(t *testing.T) {
	t.Parallel()
	var e Encoder
	e.BeginArray().Int(1).Bool(true).Bool(false).EndArray()
	assert.Equal(t, "|¢1»«÷", e.String()) // |¢1»«÷
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/infra/telemetry/ -run TestEncoder -v`
Expected: FAIL — `undefined: Encoder`.

- [ ] **Step 3: Write minimal implementation**

Append to `internal/infra/telemetry/zipson.go`:

```go
import "strings"

// Zipson serialization tokens (see OCI Telemetry MQL Decode notes).
const (
	tokenString = "¨" // ¨  string delimiter
	tokenInt    = "¢" // ¢  integer token
	tokenTrue   = "»" // »  boolean true
	tokenFalse  = "«" // «  boolean false
	tokenArrEnd = "÷" // ÷  array end
)

// Encoder builds a Zipson payload. Strings are emitted in full (no
// reference compression), which is still valid Zipson and decodes
// correctly. It does NOT escape the string-delimiter rune (U+00A8); the
// caller must not pass values containing it.
type Encoder struct {
	b strings.Builder
}

// BeginObject writes the object-start token '{'.
func (e *Encoder) BeginObject() *Encoder { e.b.WriteByte('{'); return e }

// EndObject writes the object-end token '}'.
func (e *Encoder) EndObject() *Encoder { e.b.WriteByte('}'); return e }

// BeginArray writes the array-start token '|'.
func (e *Encoder) BeginArray() *Encoder { e.b.WriteByte('|'); return e }

// EndArray writes the array-end token '÷'.
func (e *Encoder) EndArray() *Encoder { e.b.WriteString(tokenArrEnd); return e }

// Str writes a delimited Zipson string.
func (e *Encoder) Str(s string) *Encoder {
	e.b.WriteString(tokenString)
	e.b.WriteString(s)
	e.b.WriteString(tokenString)
	return e
}

// Key writes an object key (same wire form as a string).
func (e *Encoder) Key(s string) *Encoder { return e.Str(s) }

// Int writes a Zipson integer token (¢ + base62).
func (e *Encoder) Int(n int64) *Encoder {
	e.b.WriteString(tokenInt)
	e.b.WriteString(base62(n))
	return e
}

// Bool writes a Zipson boolean token.
func (e *Encoder) Bool(v bool) *Encoder {
	if v {
		e.b.WriteString(tokenTrue)
	} else {
		e.b.WriteString(tokenFalse)
	}
	return e
}

// String returns the serialized Zipson payload.
func (e *Encoder) String() string { return e.b.String() }
```

Note: move the `import "strings"` into the file's existing import block (the file from Task 1 has no imports yet, so add `import "strings"` near the top, after the package clause).

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/infra/telemetry/ -run TestEncoder -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/infra/telemetry/zipson.go internal/infra/telemetry/zipson_test.go
git commit -m "feat(telemetry): add minimal Zipson encoder"
```

---

### Task 3: MetricsURL builder

**Files:**
- Create: `internal/infra/telemetry/mql.go`
- Test: `internal/infra/telemetry/mql_test.go`

**Interfaces:**
- Consumes: `Encoder` (Task 2).
- Produces: `func MetricsURL(dacOCID, regionID, project, fleet string, start, end time.Time) string`; package-private `func metricQueries(dacOCID string) []string`; `const exploreBaseURL`.

- [ ] **Step 1: Write the failing test**

```go
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
const wantZipson = `{¨panels¨|{¨legendType¨¢1¨queries¨|{¨regionId¨¨me-abudhabi-1¨¨project¨¨GenerativeAIService¨¨fleet¨¨generative-ai-service-api-prod¨¨tql¨¨GenerativeAiService.chat.InputTokenLength[1m]{DacId = "ocid1.generativeaidedicatedaicluster.oc1.me-abudhabi-1.amaaaaaatestdac"}.grouping().sum()¨¨visible¨»¨expanded¨«}{¨regionId¨¨me-abudhabi-1¨¨project¨¨GenerativeAIService¨¨fleet¨¨generative-ai-service-api-prod¨¨tql¨¨GenerativeAiService.chat.OutputTokenLength[1m]{DacId = "ocid1.generativeaidedicatedaicluster.oc1.me-abudhabi-1.amaaaaaatestdac"}.grouping().sum()¨¨visible¨»¨expanded¨«}{¨regionId¨¨me-abudhabi-1¨¨project¨¨GenerativeAIService¨¨fleet¨¨generative-ai-service-api-prod¨¨tql¨¨GenerativeAiService.chat.ReasoningTokenLength[1m]{DacId = "ocid1.generativeaidedicatedaicluster.oc1.me-abudhabi-1.amaaaaaatestdac"}.grouping().sum()¨¨visible¨»¨expanded¨«}{¨regionId¨¨me-abudhabi-1¨¨project¨¨GenerativeAIService¨¨fleet¨¨generative-ai-service-api-prod¨¨tql¨¨GenerativeAiService.chatCompletions.InputTokenLength[1m]{DacId = "ocid1.generativeaidedicatedaicluster.oc1.me-abudhabi-1.amaaaaaatestdac"}.grouping().sum()¨¨visible¨»¨expanded¨«}{¨regionId¨¨me-abudhabi-1¨¨project¨¨GenerativeAIService¨¨fleet¨¨generative-ai-service-api-prod¨¨tql¨¨GenerativeAiService.chatCompletions.OutputTokenLength[1m]{DacId = "ocid1.generativeaidedicatedaicluster.oc1.me-abudhabi-1.amaaaaaatestdac"}.grouping().sum()¨¨visible¨»¨expanded¨«}{¨regionId¨¨me-abudhabi-1¨¨project¨¨GenerativeAIService¨¨fleet¨¨generative-ai-service-api-prod¨¨tql¨¨GenerativeAiService.chatCompletions.ReasoningTokenLength[1m]{DacId = "ocid1.generativeaidedicatedaicluster.oc1.me-abudhabi-1.amaaaaaatestdac"}.grouping().sum()¨¨visible¨»¨expanded¨«}{¨regionId¨¨me-abudhabi-1¨¨project¨¨GenerativeAIService¨¨fleet¨¨generative-ai-service-api-prod¨¨tql¨¨GenerativeAiService.responses.InputTokenLength[1m]{DacId = "ocid1.generativeaidedicatedaicluster.oc1.me-abudhabi-1.amaaaaaatestdac"}.grouping().sum()¨¨visible¨»¨expanded¨«}{¨regionId¨¨me-abudhabi-1¨¨project¨¨GenerativeAIService¨¨fleet¨¨generative-ai-service-api-prod¨¨tql¨¨GenerativeAiService.responses.OutputTokenLength[1m]{DacId = "ocid1.generativeaidedicatedaicluster.oc1.me-abudhabi-1.amaaaaaatestdac"}.grouping().sum()¨¨visible¨»¨expanded¨«}{¨regionId¨¨me-abudhabi-1¨¨project¨¨GenerativeAIService¨¨fleet¨¨generative-ai-service-api-prod¨¨tql¨¨GenerativeAiService.responses.ReasoningTokenLength[1m]{DacId = "ocid1.generativeaidedicatedaicluster.oc1.me-abudhabi-1.amaaaaaatestdac"}.grouping().sum()¨¨visible¨»¨expanded¨«}÷}÷¨searchPanelState¨{¨regionId¨¨me-abudhabi-1¨¨project¨¨GenerativeAIService¨}¨layout¨¨full¨¨startMs¨¢VMtrWcG¨endMs¨¢VMwuYuC}`

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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/infra/telemetry/ -run 'TestMetric|TestMetricsURL' -v`
Expected: FAIL — `undefined: metricQueries` / `undefined: MetricsURL` / `undefined: exploreBaseURL`.

- [ ] **Step 3: Write minimal implementation**

```go
package telemetry

import (
	"encoding/base64"
	"net/url"
	"time"
)

// exploreBaseURL is the OCI Telemetry MQL Explore page.
const exploreBaseURL = "https://devops.oci.oraclecorp.com/telemetry/mql/explore"

// metricGroups × metricKinds form the 3×3 token-length metric grid.
var (
	metricGroups = []string{"chat", "chatCompletions", "responses"}
	metricKinds  = []string{"Input", "Output", "Reasoning"}
)

// metricQueries returns the 9 MQL query strings for a DAC OCID.
func metricQueries(dacOCID string) []string {
	queries := make([]string, 0, len(metricGroups)*len(metricKinds))
	for _, g := range metricGroups {
		for _, k := range metricKinds {
			queries = append(queries,
				`GenerativeAiService.`+g+`.`+k+`TokenLength[1m]{DacId = "`+dacOCID+`"}.grouping().sum()`)
		}
	}
	return queries
}

// MetricsURL builds the full OCI Telemetry MQL Explore URL for a DAC: a
// Zipson dashboard payload, base64-std-encoded and URL-escaped.
func MetricsURL(dacOCID, regionID, project, fleet string, start, end time.Time) string {
	var e Encoder
	e.BeginObject()
	e.Key("panels").BeginArray()
	e.BeginObject().Key("legendType").Int(1).Key("queries").BeginArray()
	for _, q := range metricQueries(dacOCID) {
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
		EndObject()
	e.Key("layout").Str("full")
	e.Key("startMs").Int(start.UnixMilli())
	e.Key("endMs").Int(end.UnixMilli())
	e.EndObject()

	data := base64.StdEncoding.EncodeToString([]byte(e.String()))
	return exploreBaseURL + "?data=" + url.QueryEscape(data)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/infra/telemetry/ -v`
Expected: PASS (all telemetry tests).

- [ ] **Step 5: Commit**

```bash
git add internal/infra/telemetry/mql.go internal/infra/telemetry/mql_test.go
git commit -m "feat(telemetry): build OCI MQL metrics URL for a DAC"
```

---

### Task 4: Wire the `m` shortcut into the DAC list view

**Files:**
- Modify: `internal/ui/tui/keys/registry.go` (add binding + add to DAC context)
- Test: `internal/ui/tui/keys/registry_test.go`
- Modify: `internal/ui/tui/reducer_actions.go` (dispatch + handler + msg type)
- Modify: `internal/ui/tui/model_update.go` (error toast)

**Interfaces:**
- Consumes: `telemetry.MetricsURL` (Task 3), `actions.OpenURL` (existing), `models.DedicatedAICluster.OCID` (existing).
- Produces: `keys.OpenMetrics` binding; `(*Model).openDacMetrics(item any) tea.Cmd`; `metricsOpenErrMsg` type.

- [ ] **Step 1: Write the failing test**

Add to `internal/ui/tui/keys/registry_test.go`:

```go
func TestResolveKeys_DacHasOpenMetrics(t *testing.T) {
	t.Parallel()
	km := ResolveKeys(domain.DedicatedAICluster, common.ListView)
	found := false
	for _, b := range km.Context {
		if b.Help().Key == OpenMetrics.Help().Key {
			found = true
			break
		}
	}
	if !found {
		t.Error("DedicatedAICluster/ListView context missing OpenMetrics binding")
	}
}
```

(If `domain` / `common` are not already imported in this test file, add `"github.com/jingle2008/toolkit/internal/domain"` and `"github.com/jingle2008/toolkit/internal/ui/tui/common"` — they are used by the existing `TestResolveKeys` tests, so they are already imported.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ui/tui/keys/ -run TestResolveKeys_DacHasOpenMetrics -v`
Expected: FAIL — `undefined: OpenMetrics`.

- [ ] **Step 3a: Add the binding**

In `internal/ui/tui/keys/registry.go`, add to the `var (...)` block that contains `OpenPortal` and `Parent`:

```go
	// OpenMetrics opens the selected DAC's OCI Telemetry MQL dashboard
	// in the browser. Bound to a plain letter (list view only) — no
	// text-entry view is active for the DAC list, so it cannot collide
	// with typing.
	OpenMetrics = key.NewBinding(
		key.WithKeys("m"),
		key.WithHelp("<m>", "Open Metrics"),
	)
```

- [ ] **Step 3b: Register it for the DAC list view**

In `internal/ui/tui/keys/registry.go`, update the `domain.DedicatedAICluster` entry of `catContext` to include `OpenMetrics`:

```go
	domain.DedicatedAICluster: {
		common.ListView: {Parent, SortTenant, SortInternal, SortUsage, SortSize, SortAge, CopyTenant, EditTenant, OpenMetrics, Refresh, ToggleFaulty, Delete},
	},
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/ui/tui/keys/ -v`
Expected: PASS.

- [ ] **Step 5: Add the dispatch case, handler, and message type**

In `internal/ui/tui/reducer_actions.go`, add a case to the `switch` in `handleItemActions` (after the `keys.EditTenant` case):

```go
	case key.Matches(msg, keys.OpenMetrics):
		return m.openDacMetrics(item)
```

Add the import for the telemetry package and `"time"` to the file's import block:

```go
	"time"

	"github.com/jingle2008/toolkit/internal/infra/telemetry"
```

Append to `internal/ui/tui/reducer_actions.go`:

```go
// metricsProject is the OCI Telemetry namespace for GenAI metrics.
const metricsProject = "GenerativeAIService"

// metricsOpenErrMsg reports a failure to launch the metrics dashboard.
type metricsOpenErrMsg struct{ err error }

// openDacMetrics builds the OCI Telemetry MQL dashboard URL for the
// selected DedicatedAICluster and opens it in the browser, off the UI
// goroutine. Non-DAC selections are a logged no-op. The fleet is derived
// from the environment type (dev/preprod/prod); the window is the last
// 24h.
func (m *Model) openDacMetrics(item any) tea.Cmd {
	dac, ok := item.(*models.DedicatedAICluster)
	if !ok || dac == nil {
		m.logger.Errorw("no dedicated AI cluster selected for metrics", "category", m.category)
		return nil
	}
	ocid := dac.OCID(m.environment.Realm, m.environment.Region)
	fleet := "generative-ai-service-api-" + m.environment.Type
	now := time.Now()
	target := telemetry.MetricsURL(ocid, m.environment.Region, metricsProject, fleet, now.Add(-24*time.Hour), now)
	return func() tea.Msg {
		if err := actions.OpenURL(target); err != nil {
			return metricsOpenErrMsg{err: err}
		}
		return nil
	}
}
```

- [ ] **Step 6: Add the error toast**

In `internal/ui/tui/model_update.go`, add a case next to the existing `portalOpenErrMsg` case:

```go
	case metricsOpenErrMsg:
		return m, m.showToast(fmt.Sprintf("failed to open metrics: %v", msg.err), toastError)
```

- [ ] **Step 7: Build and run the full suite**

Run: `go build ./... && go test ./internal/ui/... ./internal/infra/telemetry/...`
Expected: build OK; all tests PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/ui/tui/keys/registry.go internal/ui/tui/keys/registry_test.go internal/ui/tui/reducer_actions.go internal/ui/tui/model_update.go
git commit -m "feat(tui): add 'm' Open Metrics shortcut for DedicatedAICluster"
```

---

### Task 5: Lint and decoder round-trip verification

**Files:** none (verification only).

- [ ] **Step 1: Full build, vet, lint, test**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: all pass. If the repo uses `golangci-lint`, run it too: `golangci-lint run ./internal/infra/telemetry/... ./internal/ui/tui/...`.

- [ ] **Step 2: Real-decoder round-trip (network-capable machine)**

The implementation sandbox could not reach the npm registry (`UNABLE_TO_VERIFY_LEAF_SIGNATURE`). On a machine with npm access, confirm the generated payload decodes with the real Zipson library:

```bash
cat > /tmp/rt.mjs <<'JS'
import { parse } from 'zipson';
import { readFileSync } from 'fs';
const o = parse(readFileSync('/tmp/z.txt','utf8'));
console.log('queries', o.panels[0].queries.length, 'layout', o.layout, 'start', o.startMs, 'end', o.endMs);
console.log('tql0', o.panels[0].queries[0].tql);
JS
# Put the wantZipson string from Task 3 into /tmp/z.txt, then:
npx --yes --package=zipson node /tmp/rt.mjs
```

Expected: prints `queries 9 layout full start 1781787680652 end 1781832733444` and a well-formed `tql0`. If `parse` throws, the encoder diverges from real Zipson — stop and investigate before merging.

- [ ] **Step 3: Live console check (optional but recommended)**

Run the app against a real environment, select a DAC, press `m`, and confirm the OCI Telemetry MQL Explore page opens with the 9 panels populated for that DAC.

- [ ] **Step 4: Finalize**

No code changes expected here. If Steps 2–3 surface a divergence, fix the encoder and re-run Tasks 1–4 tests.

---

## Self-Review

- **Spec coverage:** URL/payload format → Tasks 1–3; metric grid → Task 3 (`metricQueries`); variable inputs (region/ocid/project/fleet/window) → Task 4 handler; keybinding + dispatch + toast → Task 4; encoder bounded units → Tasks 1–2; testing & verification gate → Tasks 1–5. All spec sections mapped.
- **Placeholder scan:** none — every step has concrete code/commands and expected output.
- **Type consistency:** `Encoder`, `base62`, `metricQueries`, `MetricsURL`, `exploreBaseURL`, `OpenMetrics`, `openDacMetrics`, `metricsOpenErrMsg`, `metricsProject` are used consistently across tasks; signatures match the spec.
