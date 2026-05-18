/*
Package output renders categorized toolkit data to stdout in
machine-friendly formats (json / jsonl / yaml) or a human table.

It is intentionally TUI-free: it depends only on stdlib + yaml so
the headless `toolkit get` command can be used in scripts and from
LLM agents without paying the Bubble Tea cost.
*/
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"

	"gopkg.in/yaml.v3"
)

// Format is the on-the-wire encoding for `toolkit get`.
type Format string

// Supported output formats for `toolkit get` and consumers that
// share the same encoding contract.
const (
	FormatTable Format = "table"
	FormatJSON  Format = "json"
	FormatJSONL Format = "jsonl"
	FormatYAML  Format = "yaml"
)

// ParseFormat returns the Format for s, or an error listing valid choices.
func ParseFormat(s string) (Format, error) {
	switch Format(strings.ToLower(s)) {
	case FormatTable:
		return FormatTable, nil
	case FormatJSON:
		return FormatJSON, nil
	case FormatJSONL:
		return FormatJSONL, nil
	case FormatYAML:
		return FormatYAML, nil
	default:
		return "", fmt.Errorf("invalid output format %q (valid: table|json|jsonl|yaml)", s)
	}
}

// Options controls how renderers emit data.
type Options struct {
	Format    Format
	NoHeaders bool // table only: omit header row
	Pretty    bool // json/yaml: pretty-print
}

// WriteJSON emits items as a single JSON value (typically an array).
// A nil items value emits "[]" so pipelines like `| jq '.[]'` never
// see a null document.
func WriteJSON(w io.Writer, items any, opts Options) error {
	if items == nil {
		_, err := io.WriteString(w, "[]\n")
		return err
	}
	enc := json.NewEncoder(w)
	if opts.Pretty {
		enc.SetIndent("", "  ")
	}
	return enc.Encode(items)
}

// WriteJSONL emits one JSON object per line. items must be a slice
// (each element becomes a line) or any single JSON-encodable value
// (emitted as one line).
//
// Grouped/map data should be flattened with FlattenWithKey before
// reaching this function — callers pick the group field name
// (`pool`, `tenant`, `model`, …) explicitly rather than relying on
// a magic key.
func WriteJSONL(w io.Writer, items any, _ Options) error {
	if items == nil {
		return nil
	}
	raw, err := json.Marshal(items)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(w)
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err == nil {
		for _, msg := range arr {
			if err := enc.Encode(msg); err != nil {
				return err
			}
		}
		return nil
	}
	return enc.Encode(json.RawMessage(raw))
}

// WriteYAML emits items as a YAML document.
func WriteYAML(w io.Writer, items any, opts Options) error {
	enc := yaml.NewEncoder(w)
	defer func() { _ = enc.Close() }()
	if opts.Pretty {
		enc.SetIndent(2)
	}
	return enc.Encode(items)
}

// FlattenWithKey turns a map[string][]T into []map[string]any, injecting
// the map key into each element under groupField. Used to expose grouped
// loader data (e.g. GpuNodeMap, DedicatedAIClusterMap) as a uniform
// array of objects without leaking the underlying map shape to MCP /
// JSON consumers.
//
// The output is stable: map keys are sorted before iteration. The
// implementation round-trips through JSON so the caller's struct tags
// (omitempty, custom names, etc.) are honored.
//
// Collision rule: if T's JSON encoding already contains a field whose
// name equals groupField, the map key wins — the existing value is
// silently overwritten. Callers should choose a groupField that doesn't
// collide with any of T's tagged fields. The current production callers
// pick "pool" / "tenant" / "model", none of which clash with the
// underlying pkg/models types. Test coverage pins this behavior
// (TestFlattenWithKey_CollisionOverwrites).
func FlattenWithKey[T any](grouped map[string][]T, groupField string) []map[string]any {
	keys := make([]string, 0, len(grouped))
	for k := range grouped {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var out []map[string]any
	for _, k := range keys {
		for _, v := range grouped[k] {
			raw, err := json.Marshal(v)
			if err != nil {
				continue
			}
			var m map[string]any
			if err := json.Unmarshal(raw, &m); err != nil {
				continue
			}
			m[groupField] = k
			out = append(out, m)
		}
	}
	return out
}

// WriteTable emits a tab-aligned table. opts.NoHeaders suppresses the
// header row even when headers is non-empty.
func WriteTable(w io.Writer, headers []string, rows [][]string, opts Options) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if !opts.NoHeaders && len(headers) > 0 {
		if _, err := fmt.Fprintln(tw, strings.Join(headers, "\t")); err != nil {
			return err
		}
	}
	for _, row := range rows {
		if _, err := fmt.Fprintln(tw, strings.Join(row, "\t")); err != nil {
			return err
		}
	}
	return tw.Flush()
}
