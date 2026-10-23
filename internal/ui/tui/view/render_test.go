package view

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stripANSI removes ANSI escape codes from a string.
func stripANSI(input string) string {
	re := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	return re.ReplaceAllString(input, "")
}

func TestProductionRenderer_RenderJSON_Success(t *testing.T) {
	t.Parallel()
	r := ProductionRenderer{}
	data := map[string]any{
		"foo": "bar",
		"num": 42,
	}
	out, err := r.RenderJSON(data, 40)
	require.NoError(t, err)
	out = stripANSI(out)
	assert.Contains(t, out, "foo")
	assert.Contains(t, out, "bar")
	assert.Contains(t, out, "num")
}

// glamour.NewTermRenderer does not error for negative width, so we do not test error path here.

// trimMargins drops glamour's blank lines and shared indent but keeps
// relative indent and styling.
// A JSON line too long for the width wraps under its own text: no line
// gains or loses the left margin, and the tail of a long value isn't
// left at column one.
func TestProductionRenderer_WrapsUnderItsLine(t *testing.T) {
	t.Parallel()
	json := "{\n    \"name\": \"x\",\n    \"id\": \"ocid1.tenancy.oc1..aaaaaaaaktp7zsg4rekxv3vakzy4h7l3ftungvlrzmwnqwglvpwefohc5pma\"\n}"
	out, err := ProductionRenderer{}.RenderJSON(json, 40)
	require.NoError(t, err)
	lines := strings.Split(stripANSI(out), "\n")
	assert.Equal(t, "{", lines[0], "no margin on the first line")
	assert.Equal(t, `    "name": "x",`, lines[1])
	for _, l := range lines {
		assert.LessOrEqual(t, len(l), 40, "%q fits", l)
	}
	assert.True(t, strings.HasPrefix(lines[2], `    "id": "ocid1.`), lines[2])
	assert.True(t, strings.HasPrefix(lines[3], "      "), "the continuation hangs under its line: %q", lines[3])
	assert.Equal(t, "}", lines[len(lines)-1])
}

func TestTrimMargins(t *testing.T) {
	t.Parallel()
	in := "\n  \n    \x1b[31m{\x1b[0m\n      \"a\": 1\n    }\n\n"
	assert.Equal(t, "\x1b[31m{\x1b[0m\n  \"a\": 1\n}", trimMargins(in))
}

// RenderJSON starts at the first column on its first line, in either
// style.
func TestProductionRenderer_NoTopMargin(t *testing.T) {
	t.Parallel()
	for _, r := range []ProductionRenderer{{}, {Light: true}} {
		out, err := r.RenderJSON(`{"a": 1}`, 40)
		require.NoError(t, err)
		first := strings.Split(stripANSI(out), "\n")[0]
		assert.True(t, strings.HasPrefix(first, "{"), "light=%v: %q", r.Light, first)
		assert.Equal(t, strings.TrimRight(first, " "), first, "no trailing padding")
	}
}
