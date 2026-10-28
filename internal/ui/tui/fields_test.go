package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
)

var plainFields = fieldStyles{label: lipgloss.NewStyle(), section: lipgloss.NewStyle()}

func TestHumanize(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"name":           "Name",
		"totalReplicas":  "Total replicas",
		"is_internal":    "Is internal",
		"tenantId":       "Tenant ID",
		"gpuCount":       "GPU count",
		"OCID":           "OCID",
		"HTTPServerPort": "HTTP server port",
		"k8sVersion":     "k8s version",
		"":               "",
	} {
		assert.Equal(t, want, humanize(in), in)
	}
}

func TestParseFields_KeepsOrderAndKinds(t *testing.T) {
	t.Parallel()
	n, err := parseFields([]byte(`{"z":1,"a":"","b":null,"c":{"x":true},"d":[1,2],"e":[{"k":"v"}]}`))
	require.NoError(t, err)
	keys := make([]string, 0, len(n.fields))
	for _, f := range n.fields {
		keys = append(keys, f.key)
	}
	assert.Equal(t, []string{"z", "a", "b", "c", "d", "e"}, keys, "document order, not sorted")
	assert.Equal(t, "1", n.fields[0].val.text)
	assert.Equal(t, "—", n.fields[1].val.text, "empty string")
	assert.Equal(t, "—", n.fields[2].val.text, "null")
	assert.Equal(t, objectNode, n.fields[3].val.kind)
	assert.Equal(t, arrayNode, n.fields[4].val.kind)
}

const sampleItem = `{"name":"n1","tags":["a","b"],"owner":{"name":"acme","meta":{"deep":1}},"pods":[{"name":"p1"},{"name":"p2"}],"empty":{},"none":[]}`

// Plain values come first, then sections; scalar arrays join into one
// value; arrays of objects list each one.
func TestRenderFields_Unlimited(t *testing.T) {
	t.Parallel()
	n, err := parseFields([]byte(sampleItem))
	require.NoError(t, err)
	got := renderFields(n, 60, 0, false, true, plainFields)
	want := strings.Join([]string{
		"Name   n1",
		"Tags   a, b",
		"Empty  —",
		"None   —",
		"",
		"Owner",
		"  Name  acme",
		"  Meta",
		"    Deep  1",
		"",
		"Pods (2)",
		"  #1",
		"    Name  p1",
		"  #2",
		"    Name  p2",
	}, "\n")
	assert.Equal(t, want, got)
}

// Depth 1 (the preview) opens one level and collapses the rest.
func TestRenderFields_DepthOneCollapsesDeeper(t *testing.T) {
	t.Parallel()
	n, err := parseFields([]byte(sampleItem))
	require.NoError(t, err)
	got := renderFields(n, 60, 1, false, false, plainFields)
	assert.Contains(t, got, "Owner\n  Name  acme\n  Meta  {…}")
	assert.Contains(t, got, "#1\n    Name  p1")
	assert.NotContains(t, got, "Deep")
}

// Long values are cut to the width in the middle, keeping their end, or
// wrapped under themselves.
func TestRenderFields_CutOrWrap(t *testing.T) {
	t.Parallel()
	n, err := parseFields([]byte(`{"id":"aaaa bbbb cccc dddd"}`))
	require.NoError(t, err)
	assert.Equal(t, "ID  aaaa… dddd", renderFields(n, 14, 0, false, false, plainFields))
	assert.Equal(t, "ID  aaaa bbbb\n    cccc dddd", renderFields(n, 14, 0, true, false, plainFields))
}

// Wrapping breaks only at spaces: an OCID with hyphens fills each line
// rather than breaking at "us-chicago-" and leaving a gap.
func TestWrapAtSpaces(t *testing.T) {
	t.Parallel()
	assert.Equal(t, []string{"ocid1.instance.oc1.us-chic", "ago-1.anxxeljrql3powickfj6"},
		wrapAtSpaces("ocid1.instance.oc1.us-chicago-1.anxxeljrql3powickfj6", 26))
	assert.Equal(t, []string{"a long", "value", "wrapped"}, wrapAtSpaces("a long value wrapped", 7))
	assert.Equal(t, []string{"short", "abcdefghijkl", "mn"}, wrapAtSpaces("short abcdefghijklmn", 12),
		"a word wider than a line starts the next one and is cut at the width")
	assert.Equal(t, []string{"fits"}, wrapAtSpaces("fits", 10))
}

// A scalar array whose join would be ambiguous (an item holds ", ") or
// overflow the value column lists one bulleted item per row, each wrapped
// under its own text; short lists, and single items, stay one value.
func TestRenderFields_StackedLists(t *testing.T) {
	t.Parallel()
	render := func(doc string, width int, wrap bool) string {
		n, err := parseFields([]byte(doc))
		require.NoError(t, err)
		return renderFields(n, width, 0, wrap, false, plainFields)
	}
	assert.Equal(t, "Issues  • GPU bus error, on 3\n        • Disk full",
		render(`{"issues":["GPU bus error, on 3","Disk full"]}`, 40, true), "an item holds a comma")
	assert.Equal(t, "Issues  • aaaa bbbb\n          cccc\n        • dddd",
		render(`{"issues":["aaaa bbbb cccc","dddd"]}`, 20, true), "join overflows; items wrap under their text")
	assert.Equal(t, "Issues  • aaaa… cccc\n        • dddd",
		render(`{"issues":["aaaa bbbb cccc","dddd"]}`, 20, false), "cut, not wrapped")
	assert.Equal(t, "Tags  a, b", render(`{"tags":["a","b"]}`, 40, true))
	assert.Equal(t, "Issues  one, two", render(`{"issues":["one, two"]}`, 40, true), "a single item can't be ambiguous")
}

// tab switches details between fields and JSON; the breadcrumb says which.
func TestDetails_ToggleJSON(t *testing.T) {
	t.Parallel()
	m := newGoldenModel(t, 120, 40)
	showDetails(t, m)
	assert.False(t, m.detail.json)
	assert.Contains(t, ansi.Strip(m.viewport.View()), "Tenant ID")
	assert.Equal(t, "fields", m.detailPosition())

	_, cmd := m.updateDetailView(keyPress("tab"))
	require.NotNil(t, cmd)
	m.handleDetailContentRenderedMsg(cmd().(detailContentRenderedMsg))
	assert.True(t, m.detail.json)
	assert.Contains(t, ansi.Strip(m.viewport.View()), `"tenantId": "acme"`)
	assert.Equal(t, "JSON", m.detailPosition())
	assert.Equal(t, common.DetailsView, m.viewMode)
}

// A details body taller than the panel shows how far down it is.
func TestDetailPosition_Scrolled(t *testing.T) {
	t.Parallel()
	m := newGoldenModel(t, 80, 10)
	showCategory(t, m, domain.DedicatedAICluster)
	require.NotNil(t, m.enterDetailView())
	m.viewport.SetContent(strings.Repeat("line\n", 40))
	assert.Equal(t, "fields · 0%", m.detailPosition())
	m.viewport.GotoBottom()
	assert.Equal(t, "fields · 100%", m.detailPosition())
}
