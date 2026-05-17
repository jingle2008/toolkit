package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseFormat(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want Format
		ok   bool
	}{
		{"json", FormatJSON, true},
		{"JSON", FormatJSON, true},
		{"jsonl", FormatJSONL, true},
		{"yaml", FormatYAML, true},
		{"table", FormatTable, true},
		{"", "", false},
		{"toml", "", false},
	}
	for _, tc := range cases {
		got, err := ParseFormat(tc.in)
		if tc.ok {
			require.NoError(t, err, "ParseFormat(%q)", tc.in)
			assert.Equal(t, tc.want, got)
		} else {
			assert.Error(t, err, "ParseFormat(%q) should fail", tc.in)
		}
	}
}

func TestWriteJSON_NilEmitsArray(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	require.NoError(t, WriteJSON(&buf, nil, Options{Pretty: true}))
	assert.Equal(t, "[]\n", buf.String())
}

func TestWriteJSON_Pretty(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	in := []map[string]any{{"name": "a"}, {"name": "b"}}
	require.NoError(t, WriteJSON(&buf, in, Options{Pretty: true}))
	assert.Contains(t, buf.String(), "  \"name\": \"a\"")
}

func TestWriteJSONL_Array(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	in := []map[string]any{{"name": "a"}, {"name": "b"}}
	require.NoError(t, WriteJSONL(&buf, in, Options{}))
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	require.Len(t, lines, 2)
	for _, line := range lines {
		var obj map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &obj))
		assert.Contains(t, obj, "name")
	}
}

func TestWriteJSONL_MapAddsGroup(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	in := map[string][]map[string]any{
		"alpha": {{"name": "a1"}, {"name": "a2"}},
	}
	require.NoError(t, WriteJSONL(&buf, in, Options{}))
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	require.Len(t, lines, 2)
	for _, line := range lines {
		var obj map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &obj))
		assert.Equal(t, "alpha", obj["_group"])
	}
}

func TestWriteYAML(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	in := []map[string]any{{"name": "a"}}
	require.NoError(t, WriteYAML(&buf, in, Options{Pretty: true}))
	assert.Contains(t, buf.String(), "name: a")
}

func TestWriteTable_HeadersAndRows(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	headers := []string{"NAME", "AGE"}
	rows := [][]string{{"alice", "30"}, {"bob", "40"}}
	require.NoError(t, WriteTable(&buf, headers, rows, Options{}))
	out := buf.String()
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	require.Len(t, lines, 3)
	assert.Contains(t, lines[0], "NAME")
	assert.Contains(t, lines[0], "AGE")
	assert.Contains(t, lines[1], "alice")
	assert.Contains(t, lines[2], "bob")
}

func TestWriteTable_NoHeaders(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	require.NoError(t, WriteTable(&buf, []string{"NAME"}, [][]string{{"alice"}}, Options{NoHeaders: true}))
	assert.NotContains(t, buf.String(), "NAME")
	assert.Contains(t, buf.String(), "alice")
}
