package output

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
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
		{"csv", FormatCSV, true},
		{"CSV", FormatCSV, true},
		{"tsv", FormatTSV, true},
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

func TestWriteJSON_TypedNilSliceEmitsArray(t *testing.T) {
	t.Parallel()
	// The case that actually reaches this in practice: a filter matched
	// nothing, so the caller hands over []T(nil) — a non-nil `any` that
	// encoding/json would render as "null", breaking `| jq '.[]'`.
	type item struct {
		Name string `json:"name"`
	}
	var nilSlice []item

	var buf bytes.Buffer
	require.NoError(t, WriteJSON(&buf, nilSlice, Options{}))
	assert.Equal(t, "[]", strings.TrimSpace(buf.String()))
}

func TestWriteJSON_EmptyNonNilSliceEmitsArray(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	require.NoError(t, WriteJSON(&buf, []string{}, Options{}))
	assert.Equal(t, "[]", strings.TrimSpace(buf.String()))
}

func TestWriteJSON_TypedNilMapEmitsObject(t *testing.T) {
	t.Parallel()
	var nilMap map[string][]string

	// A map's empty form is an object, not an array.
	var buf bytes.Buffer
	require.NoError(t, WriteJSON(&buf, nilMap, Options{}))
	assert.Equal(t, "{}", strings.TrimSpace(buf.String()))
}

func TestWriteJSON_StructIsNotNormalized(t *testing.T) {
	t.Parallel()
	// `toolkit config -o json` passes a struct view and wants an object;
	// normalizing that to "[]" would corrupt it.
	type view struct {
		RepoPath string `json:"repo_path"`
	}

	var buf bytes.Buffer
	require.NoError(t, WriteJSON(&buf, view{RepoPath: "/tmp"}, Options{}))
	assert.JSONEq(t, `{"repo_path":"/tmp"}`, buf.String())
}

func TestWriteJSON_NilPointerStillNull(t *testing.T) {
	t.Parallel()
	// "null" is the honest encoding for a missing object — only nil
	// collections are normalized.
	type view struct{}
	var p *view

	var buf bytes.Buffer
	require.NoError(t, WriteJSON(&buf, p, Options{}))
	assert.Equal(t, "null", strings.TrimSpace(buf.String()))
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

func TestWriteDelimited_CSVQuotesEmbeddedComma(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	headers := []string{"NAME", "NOTE"}
	rows := [][]string{{"alice", "hello, world"}, {"bob", `quoted "value"`}}
	require.NoError(t, WriteDelimited(&buf, headers, rows, Options{}, ','))

	// Round-trip through encoding/csv to confirm the bytes are valid CSV
	// and the fields decode back to the original strings.
	r := csv.NewReader(strings.NewReader(buf.String()))
	records, err := r.ReadAll()
	require.NoError(t, err)
	assert.Equal(t, [][]string{
		{"NAME", "NOTE"},
		{"alice", "hello, world"},
		{"bob", `quoted "value"`},
	}, records)
}

func TestWriteDelimited_TSVUsesTabSeparator(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	headers := []string{"NAME", "NOTE"}
	rows := [][]string{{"alice", "hello, world"}}
	require.NoError(t, WriteDelimited(&buf, headers, rows, Options{}, '\t'))

	out := buf.String()
	// Tab is the separator; the comma in "hello, world" must not trigger
	// quoting, since the field doesn't contain a tab.
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	require.Len(t, lines, 2)
	assert.Equal(t, "NAME\tNOTE", lines[0])
	assert.Equal(t, "alice\thello, world", lines[1])
}

func TestWriteDelimited_NoHeaders(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	require.NoError(t, WriteDelimited(&buf, []string{"NAME"}, [][]string{{"alice"}}, Options{NoHeaders: true}, ','))
	assert.Equal(t, "alice\n", buf.String())
}

// failWriter fails every write, so renderers' error paths can be
// exercised without a real broken pipe.
type failWriter struct{ err error }

func (f failWriter) Write([]byte) (int, error) { return 0, f.err }

// bigRows builds a payload past bufio's 4096-byte default so a buffered
// writer (encoding/csv) actually flushes to the underlying writer mid-run
// rather than only at the end.
func bigRows() [][]string {
	rows := make([][]string, 0, 64)
	for range 64 {
		rows = append(rows, []string{strings.Repeat("x", 128)})
	}
	return rows
}

func TestFlatten(t *testing.T) {
	t.Parallel()
	grouped := map[string][]string{
		"pool-c": {"c1", "c2"},
		"pool-a": {"a1"},
		"pool-b": {"b1", "b2"},
	}
	// Keys are visited in sorted order, so the flat slice is
	// deterministic regardless of Go's map iteration order.
	assert.Equal(t, []string{"a1", "b1", "b2", "c1", "c2"}, Flatten(grouped))
}

func TestFlatten_Empty(t *testing.T) {
	t.Parallel()
	// Non-nil empty slice, so `-o json` of an empty group emits [] not null.
	got := Flatten(map[string][]string{})
	assert.NotNil(t, got)
	assert.Empty(t, got)

	got = Flatten[string](nil)
	assert.NotNil(t, got)
	assert.Empty(t, got)
}

func TestFlatten_PreservesStructType(t *testing.T) {
	t.Parallel()
	type node struct {
		Name string `json:"name"`
		Pool string `json:"poolName"`
	}
	grouped := map[string][]node{
		"p2": {{Name: "n2", Pool: "p2"}},
		"p1": {{Name: "n1", Pool: "p1"}},
	}
	got := Flatten(grouped)
	require.Len(t, got, 2)
	assert.Equal(t, "n1", got[0].Name, "sorted by group key")
	assert.Equal(t, "n2", got[1].Name)
}

func TestWriteJSONL_Nil(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	require.NoError(t, WriteJSONL(&buf, nil, Options{}))
	assert.Empty(t, buf.String(), "nil should emit nothing, not a null line")
}

func TestWriteJSONL_SingleValueEmitsOneLine(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	// A map isn't a JSON array, so it takes the single-value branch.
	require.NoError(t, WriteJSONL(&buf, map[string]any{"name": "a"}, Options{}))
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	require.Len(t, lines, 1)
	var obj map[string]any
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &obj))
	assert.Equal(t, "a", obj["name"])
}

func TestWriteJSONL_MarshalError(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	// A channel can't be marshaled, so the initial json.Marshal fails.
	err := WriteJSONL(&buf, make(chan int), Options{})
	require.Error(t, err)
}

func TestWriteJSONL_WriteError(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("pipe closed")
	err := WriteJSONL(failWriter{err: sentinel}, []map[string]any{{"name": "a"}}, Options{})
	require.ErrorIs(t, err, sentinel)
}

func TestWriteJSON_WriteError(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("pipe closed")

	// nil takes the "[]" short-circuit, which writes directly.
	err := WriteJSON(failWriter{err: sentinel}, nil, Options{})
	require.ErrorIs(t, err, sentinel)

	// non-nil goes through the encoder.
	err = WriteJSON(failWriter{err: sentinel}, []string{"a"}, Options{})
	require.ErrorIs(t, err, sentinel)
}

func TestWriteYAML_NilCollectionsMatchJSON(t *testing.T) {
	t.Parallel()
	type item struct {
		Name string `json:"name"`
	}
	var nilSlice []item
	var nilMap map[string][]item

	// -o yaml and -o json must agree on how an empty result looks;
	// yaml.Marshal on its own would emit "null" here.
	var buf bytes.Buffer
	require.NoError(t, WriteYAML(&buf, nilSlice, Options{}))
	assert.Equal(t, "[]", strings.TrimSpace(buf.String()))

	buf.Reset()
	require.NoError(t, WriteYAML(&buf, nilMap, Options{}))
	assert.Equal(t, "{}", strings.TrimSpace(buf.String()))
}

func TestWriteYAML_StructStillMarshals(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	require.NoError(t, WriteYAML(&buf, map[string]string{"name": "a"}, Options{}))
	assert.Contains(t, buf.String(), "name: a", "non-nil values must still go through yaml.Marshal")
}

func TestWriteYAML_MarshalError(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	// sigs.k8s.io/yaml marshals via JSON, so a channel fails there too.
	err := WriteYAML(&buf, make(chan int), Options{})
	require.Error(t, err)
}

func TestWriteYAML_WriteError(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("pipe closed")
	err := WriteYAML(failWriter{err: sentinel}, []string{"a"}, Options{})
	require.ErrorIs(t, err, sentinel)
}

func TestWriteDelimited_WriteError(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("pipe closed")

	// Header big enough to force a flush during the header write.
	bigHeader := []string{strings.Repeat("H", 8192)}
	err := WriteDelimited(failWriter{err: sentinel}, bigHeader, nil, Options{}, ',')
	require.ErrorIs(t, err, sentinel)

	// Rows big enough to force a flush during WriteAll.
	err = WriteDelimited(failWriter{err: sentinel}, nil, bigRows(), Options{}, ',')
	require.ErrorIs(t, err, sentinel)
}

func TestWriteTable_FlushError(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("pipe closed")
	// tabwriter buffers until Flush, so the error surfaces there.
	err := WriteTable(failWriter{err: sentinel}, []string{"NAME"}, [][]string{{"alice"}}, Options{})
	require.ErrorIs(t, err, sentinel)
}
