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
	"strings"
	"text/tabwriter"

	"gopkg.in/yaml.v3"
)

// Format is the on-the-wire encoding for `toolkit get`.
type Format string

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

// WriteJSONL emits one JSON object per line. items must be either a
// slice (each element becomes a line) or a map[string][]T (each value
// element becomes a line, with the key carried in a "_group" field
// when keyed input is detected).
//
// "_group" is reserved by this writer for keyed-input flattening:
// callers must not name a JSON field "_group" on any model that may
// reach this function, otherwise the key would be silently overwritten.
//
// TODO(perf): the map path currently marshals the input once, then
// unmarshals into map[string][]json.RawMessage, then re-marshals each
// element with the injected "_group" key — roughly 3× the steady-state
// memory of a streaming writer. Acceptable for current dataset sizes;
// revisit with a reflect-based streaming implementation if profiles
// show it as a hotspot.
func WriteJSONL(w io.Writer, items any, _ Options) error {
	if items == nil {
		return nil
	}
	raw, err := json.Marshal(items)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(w)
	// Try array first.
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err == nil {
		for _, msg := range arr {
			if err := enc.Encode(msg); err != nil {
				return err
			}
		}
		return nil
	}
	// Then map[string][]any — flatten with a "_group" key.
	var obj map[string][]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err == nil {
		for k, msgs := range obj {
			for _, msg := range msgs {
				var fields map[string]json.RawMessage
				if err := json.Unmarshal(msg, &fields); err != nil {
					fields = map[string]json.RawMessage{}
				}
				groupJSON, _ := json.Marshal(k)
				fields["_group"] = groupJSON
				out, err := json.Marshal(fields)
				if err != nil {
					return err
				}
				if err := enc.Encode(json.RawMessage(out)); err != nil {
					return err
				}
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
