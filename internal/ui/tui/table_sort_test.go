package tui

import (
	"reflect"
	"slices"
	"testing"

	"github.com/charmbracelet/bubbles/table"
)

func TestSortByInt(t *testing.T) {
	t.Parallel()
	rows := []table.Row{
		{"foo", "10"},
		{"bar", "2"},
		{"baz", "30"},
	}
	expectedAsc := []table.Row{
		{"bar", "2"},
		{"foo", "10"},
		{"baz", "30"},
	}
	expectedDesc := []table.Row{
		{"baz", "30"},
		{"foo", "10"},
		{"bar", "2"},
	}
	sortByInt(rows, 1, true)
	if !reflect.DeepEqual(rows, expectedAsc) {
		t.Errorf("sortByInt asc failed: got %v, want %v", rows, expectedAsc)
	}
	sortByInt(rows, 1, false)
	if !reflect.DeepEqual(rows, expectedDesc) {
		t.Errorf("sortByInt desc failed: got %v, want %v", rows, expectedDesc)
	}
}

func TestSortByPercent(t *testing.T) {
	t.Parallel()
	rows := []table.Row{
		{"foo", "10%"},
		{"bar", "2%"},
		{"baz", "30%"},
	}
	expectedAsc := []table.Row{
		{"bar", "2%"},
		{"foo", "10%"},
		{"baz", "30%"},
	}
	expectedDesc := []table.Row{
		{"baz", "30%"},
		{"foo", "10%"},
		{"bar", "2%"},
	}
	sortByPercent(rows, 1, true)
	if !reflect.DeepEqual(rows, expectedAsc) {
		t.Errorf("sortByPercent asc failed: got %v, want %v", rows, expectedAsc)
	}
	sortByPercent(rows, 1, false)
	if !reflect.DeepEqual(rows, expectedDesc) {
		t.Errorf("sortByPercent desc failed: got %v, want %v", rows, expectedDesc)
	}
}

func TestSortByString(t *testing.T) {
	t.Parallel()
	rows := []table.Row{
		{"foo", "b"},
		{"bar", "a"},
		{"baz", "c"},
	}
	expectedAsc := []table.Row{
		{"bar", "a"},
		{"foo", "b"},
		{"baz", "c"},
	}
	expectedDesc := []table.Row{
		{"baz", "c"},
		{"foo", "b"},
		{"bar", "a"},
	}
	sortByString(rows, 1, true)
	if !reflect.DeepEqual(rows, expectedAsc) {
		t.Errorf("sortByString asc failed: got %v, want %v", rows, expectedAsc)
	}
	sortByString(rows, 1, false)
	if !reflect.DeepEqual(rows, expectedDesc) {
		t.Errorf("sortByString desc failed: got %v, want %v", rows, expectedDesc)
	}
}

func TestParseSize(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in   string
		want int64
	}{
		{"6B", 6000000000},
		{"3.5M", 3500000},
		{"1.2T", 1200000000000},
		{"42", 42},
		{"", 0},
		{"  7m ", 7000000},
		{"bad", 0}, // should error, returns 0
	}
	for _, tt := range tests {
		got, _ := parseSize(tt.in)
		if got != tt.want {
			t.Errorf("parseSize(%q) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestSortByAgeAndSize(t *testing.T) {
	t.Parallel()
	// Fake k8stime.ParseAge returns 0 for "", so use numbers as string
	rows := []table.Row{
		{"foo", "10h", "6B"},
		{"bar", "2h", "3.5M"},
		{"baz", "30h", "1.2T"},
	}
	headers := []header{
		{text: "Name"},
		{text: "Age"},
		{text: "Size"},
	}
	// sort by Age ascending
	sortRows(rows, headers, "Age", true)
	if rows[0][0] != "bar" || rows[2][0] != "baz" {
		t.Errorf("sortRows by Age asc failed: got %v", rows)
	}
	// sort by Size descending
	sortRows(rows, headers, "Size", false)
	if rows[0][0] != "baz" || rows[2][0] != "bar" {
		t.Errorf("sortRows by Size desc failed: got %v", rows)
	}
}

// TestSortByGPUs pins numeric ordering for the GPU-count column. Under
// the default string sort these order 1, 16, 2, 32, 4, 8 — plausible
// enough at a glance to be missed, which is why it's pinned rather
// than left to the generic sort test.
func TestSortByGPUs(t *testing.T) {
	t.Parallel()
	rows := []table.Row{
		{"a", "8"},
		{"b", "16"},
		{"c", "2"},
		{"d", "32"},
	}
	headers := []header{{text: "Name"}, {text: "GPUs"}}

	sortRows(rows, headers, "GPUs", true)
	got := []string{rows[0][1], rows[1][1], rows[2][1], rows[3][1]}
	if want := []string{"2", "8", "16", "32"}; !slices.Equal(got, want) {
		t.Errorf("GPUs asc = %v, want %v", got, want)
	}

	sortRows(rows, headers, "GPUs", false)
	got = []string{rows[0][1], rows[1][1], rows[2][1], rows[3][1]}
	if want := []string{"32", "16", "8", "2"}; !slices.Equal(got, want) {
		t.Errorf("GPUs desc = %v, want %v", got, want)
	}
}

// A runtime declaring neither a GPU limit nor a request renders an
// empty cell; it must sort as zero rather than landing between "1" and
// "2" the way a string compare would put it.
func TestSortByGPUs_BlankSortsAsZero(t *testing.T) {
	t.Parallel()
	rows := []table.Row{{"a", "4"}, {"b", ""}, {"c", "1"}}
	headers := []header{{text: "Name"}, {text: "GPUs"}}

	sortRows(rows, headers, "GPUs", true)
	if rows[0][0] != "b" {
		t.Errorf("blank should sort first ascending, got %v", rows)
	}
}
