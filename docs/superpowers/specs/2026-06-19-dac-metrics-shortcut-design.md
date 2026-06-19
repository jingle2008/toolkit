# DAC "Open Metrics" Shortcut — Design

**Date:** 2026-06-19
**Status:** Approved (pending spec review)

## 1. Goal

Add a list-view keyboard shortcut to the **DedicatedAICluster** category that opens
an OCI Telemetry MQL Explore dashboard, pre-loaded with token-length metrics scoped
to the selected DAC, in the user's default browser.

Reuses the existing browser-launch mechanism already powering the `ctrl+o`
"Open in portal" shortcut (`actions.OpenURL` + an off-UI-goroutine `tea.Cmd`).

## 2. URL & Payload Format

The target URL is:

```
https://devops.oci.oraclecorp.com/telemetry/mql/explore?data=<X>
```

where `X = urlEncode( base64Std( zipson(dashboardState) ) )`.

- **Zipson**: the dashboard state is serialized with the Zipson format (compact
  JSON-like encoding). String-reference compression is intentionally **not** used —
  emitting every string in full is still valid Zipson and decodes correctly, which
  keeps the encoder self-contained.
- **base64**: standard alphabet (`A–Za–z0–9+/`, `=` padding), over the UTF-8 bytes
  of the Zipson string.
- **URL encode**: `url.QueryEscape` over the standard base64. Required because the
  base64 body can contain `+` (would decode to a space), `=` (key/value separator),
  and `/`. base64url is **not** an option — the console's `atob` expects standard
  base64. This matches the reference URL, where `==` padding appears as `%3D%3D`.

### dashboardState structure

```
{
  panels: [
    { legendType: 1,
      queries: [ <9 query objects> ] }
  ],
  searchPanelState: { regionId, project },
  layout: "full",
  startMs: <epoch ms>,
  endMs:   <epoch ms>
}
```

Each query object:

```
{ regionId, project, fleet, tql, visible: true, expanded: false }
```

### tql metric grid (9 queries)

The cross-product of three method groups and three token kinds:

- groups: `chat`, `chatCompletions`, `responses`
- kinds:  `Input`, `Output`, `Reasoning`

```
GenerativeAiService.<group>.<kind>TokenLength[1m]{DacId = "<dacOCID>"}.grouping().sum()
```

## 3. Variable Inputs (all already available on the model)

| Field      | Source                                                        |
|------------|---------------------------------------------------------------|
| `regionId` | `environment.Region` (full region name, e.g. `me-abudhabi-1`) |
| `dacOCID`  | `dac.OCID(realm, region)` — existing method on the model      |
| `project`  | constant `"GenerativeAIService"`                              |
| `fleet`    | `"generative-ai-service-api-" + environment.Type`             |
| `startMs`  | `now.Add(-24h).UnixMilli()`                                   |
| `endMs`    | `now.UnixMilli()`                                             |

`environment.Realm` and `environment.Region` feed `dac.OCID`. `fleet` uses the raw
`Type` (`dev`→`dev`, `preprod`→`preprod`, `prod`→`prod`) — no `ppe` remapping,
unlike `KubeContext`.

Epoch milliseconds are timezone-independent, so no timezone handling is needed
(the PDF's `-07:00` was only for human-readable display).

## 4. New Code (bounded units)

### `internal/infra/telemetry/zipson.go`

A minimal Zipson `Encoder` backed by a `strings.Builder`:

| Method                    | Emits                                  |
|---------------------------|----------------------------------------|
| `BeginObject` / `EndObject` | `{` / `}`                            |
| `BeginArray` / `EndArray`   | `\|` / `÷`                            |
| `Key(s)` / `Str(s)`         | `¨` + s + `¨`                         |
| `Int(n)`                    | `¢` + base62(n)                       |
| `Bool(b)`                   | `»` (true) / `«` (false)              |

base62 uses Zipson's alphabet
`0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz`.
No string-reference (`ß`) or small-integer (`Ë`…) tokens are produced; the general
integer token (`¢1`) is used for all integers including `legendType`.

### `internal/infra/telemetry/mql.go`

Owns the dashboard shape and URL assembly:

```go
const exploreBaseURL = "https://devops.oci.oraclecorp.com/telemetry/mql/explore"

func MetricsURL(dacOCID, regionID, project, fleet string, start, end time.Time) string
```

Builds the payload via the encoder, base64-std-encodes the bytes, `url.QueryEscape`es,
and returns the full URL. The metric grid (groups × kinds) is a package-level
constant.

## 5. Integration (mirrors `OpenPortal`)

- **`internal/ui/tui/keys/registry.go`**
  - Add `OpenMetrics = key.NewBinding(key.WithKeys("m"), key.WithHelp("<m>", "Open Metrics"))`.
  - Add `OpenMetrics` to `catContext[domain.DedicatedAICluster][common.ListView]`.
- **`internal/ui/tui/reducer_actions.go`** — in `handleItemActions`, add:
  ```go
  case key.Matches(msg, keys.OpenMetrics):
      return m.openDacMetrics(item)
  ```
- **New `openDacMetrics(item any) tea.Cmd`** (e.g. in `reducer_actions.go`):
  - type-asserts `*models.DedicatedAICluster`; non-DAC / nil → log + no-op.
  - computes inputs from `m.environment`, builds the URL via `telemetry.MetricsURL`.
  - launches `actions.OpenURL(url)` off the UI goroutine, returning
    `metricsOpenErrMsg{err}` on launch failure.
- **`internal/ui/tui/model_update.go`** — add a `case metricsOpenErrMsg:` toast,
  mirroring the existing `portalOpenErrMsg` handling.

## 6. Error Handling

- Non-DAC or nil selection → logged no-op (defensive; the binding is only registered
  for the DAC category).
- Browser-launch failure → error toast (same path as portal open failure).

## 7. Testing & Verification

- **Go unit tests**
  - Encoder: golden-string assertions for objects, arrays, strings, ints, bools, and
    nesting.
  - `MetricsURL`: for fixed inputs and timestamps, base64-decode the `data=` param and
    assert the Zipson string equals a hand-verified expected string (pins byte-exact
    output without needing a decoder).
- **Verification gate (manual / script)**
  - Round-trip the generated payload through the real `zipson` npm decoder via `npx`
    to confirm it parses to the intended JSON. Byte-matching the user's reference URL
    is not possible (our corrected metric grid is 9 queries; the reference had 8 with a
    duplicate and a typo), so a decoder round-trip is the correctness check.
  - Ideally open one generated URL in the console to confirm it renders.

## 8. Assumptions

- For IAD/PHX, `dac.OCID` normalizes the region to `iad`/`phx` inside the OCID, but
  `regionId` uses the full region string from `environment.Region`. This matches the
  reference for `me-abudhabi-1`; the IAD/PHX combination is unverified.
- `searchPanelState`, `legendType: 1`, and `layout: "full"` are replicated as fixed
  UI state. The per-query `highlighted` flag seen in the reference is omitted
  (defaults to false).
- The reference's `searchPanelState.project` was lower-cased (`GenerativeAIservice`);
  the canonical `GenerativeAIService` is used instead, as this is UI search state and
  not query-affecting.
