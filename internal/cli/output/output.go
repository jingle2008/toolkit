/*
Package output renders categorized toolkit data to stdout in
machine-friendly formats (json / jsonl / yaml / csv / tsv) or a
human table.

It is intentionally TUI-free: it depends only on stdlib + yaml so
the headless `toolkit get` command can be used in scripts and from
LLM agents without paying the Bubble Tea cost.
*/
package output

import (
	"encoding/csv"
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
	FormatCSV   Format = "csv"
	FormatTSV   Format = "tsv"
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
	case FormatCSV:
		return FormatCSV, nil
	case FormatTSV:
		return FormatTSV, nil
	default:
		return "", fmt.Errorf("invalid output format %q (valid: table|json|jsonl|yaml|csv|tsv)", s)
	}
}

// Options controls how renderers emit data.
type Options struct {
	Format    Format
	NoHeaders bool // table/csv/tsv: omit header row
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
// Grouped/map data should be flattened with Flatten (the group key
// is expected to be a struct field on the value) before reaching
// this function.
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

// Flatten concatenates a grouped map[string][]T into a flat []T with
// deterministic key ordering. Use when the group key is already
// preserved on each value (so injecting it again would just
// duplicate — see GpuNode.NodePool / ModelArtifact.ModelName). The
// returned slice preserves T's full type so the caller can keep
// using struct tags / custom JSON marshaling without the
// FlattenWithKey JSON round-trip.
func Flatten[T any](grouped map[string][]T) []T {
	keys := make([]string, 0, len(grouped))
	for k := range grouped {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]T, 0)
	for _, k := range keys {
		out = append(out, grouped[k]...)
	}
	return out
}

// WriteDelimited emits headers + rows as delimiter-separated values
// using encoding/csv, which handles quoting for fields containing the
// separator, double quotes, or newlines. Pass ',' for CSV or '\t' for
// TSV. opts.NoHeaders suppresses the header row.
func WriteDelimited(w io.Writer, headers []string, rows [][]string, opts Options, sep rune) error {
	cw := csv.NewWriter(w)
	cw.Comma = sep
	if !opts.NoHeaders && len(headers) > 0 {
		if err := cw.Write(headers); err != nil {
			return err
		}
	}
	if err := cw.WriteAll(rows); err != nil {
		return err
	}
	cw.Flush()
	return cw.Error()
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
