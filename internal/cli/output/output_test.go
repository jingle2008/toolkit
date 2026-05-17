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

func TestFlattenWithKey_NilAndEmpty(t *testing.T) {
	t.Parallel()
	type T struct{ Name string }

	// Nil map.
	got := FlattenWithKey[T](nil, "key")
	assert.Empty(t, got)

	// Empty map.
	got = FlattenWithKey(map[string][]T{}, "key")
	assert.Empty(t, got)

	// Key with empty value slice contributes zero rows.
	got = FlattenWithKey(map[string][]T{"k": {}}, "key")
	assert.Empty(t, got)
}

func TestFlattenWithKey_SingleEntry(t *testing.T) {
	t.Parallel()
	type Pool struct {
		Name string `json:"name"`
		Size int    `json:"size"`
	}
	got := FlattenWithKey(map[string][]Pool{"alpha": {{Name: "n1", Size: 4}}}, "group")
	require.Len(t, got, 1)
	assert.Equal(t, "alpha", got[0]["group"])
	assert.Equal(t, "n1", got[0]["name"])
	assert.EqualValues(t, 4, got[0]["size"])
}

func TestFlattenWithKey_SortStable(t *testing.T) {
	t.Parallel()
	type T struct {
		Name string `json:"name"`
	}
	// Insertion order would be undefined for a map; FlattenWithKey
	// must sort keys for a deterministic output.
	in := map[string][]T{
		"charlie": {{Name: "c1"}},
		"alpha":   {{Name: "a1"}, {Name: "a2"}},
		"bravo":   {{Name: "b1"}},
	}
	got := FlattenWithKey(in, "group")
	require.Len(t, got, 4)
	groups := []string{got[0]["group"].(string), got[1]["group"].(string), got[2]["group"].(string), got[3]["group"].(string)}
	assert.Equal(t, []string{"alpha", "alpha", "bravo", "charlie"}, groups)
}

func TestFlattenWithKey_OmitemptyRespected(t *testing.T) {
	t.Parallel()
	type T struct {
		Name string `json:"name"`
		Note string `json:"note,omitempty"`
	}
	got := FlattenWithKey(map[string][]T{"k": {{Name: "n"}}}, "group")
	require.Len(t, got, 1)
	_, hasNote := got[0]["note"]
	assert.False(t, hasNote, "omitempty field should not appear when empty")
	assert.Equal(t, "n", got[0]["name"])
}

func TestFlattenWithKey_CollisionOverwrites(t *testing.T) {
	t.Parallel()
	// A type whose JSON field name collides with groupField.
	type T struct {
		Group string `json:"group"`
		Name  string `json:"name"`
	}
	in := map[string][]T{"injected-key": {{Group: "original-value", Name: "n"}}}
	got := FlattenWithKey(in, "group")
	require.Len(t, got, 1)
	// Map key wins; T's "group" field is silently overwritten. This is
	// documented in FlattenWithKey's godoc and exists primarily to
	// prevent silent corruption from going unnoticed.
	assert.Equal(t, "injected-key", got[0]["group"])
	assert.Equal(t, "n", got[0]["name"])
}
