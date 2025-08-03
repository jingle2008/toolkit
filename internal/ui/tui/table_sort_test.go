package tui

import (
	"reflect"
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
