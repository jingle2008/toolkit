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
