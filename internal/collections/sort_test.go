package collections

import (
	"reflect"
	"testing"
)

type testItem struct {
	Key   string
	Value int
}

func (ti testItem) GetKey() string {
	return ti.Key
}

func TestSortKeyedItems_EmptySlice(t *testing.T) {
	var items []testItem
	SortKeyedItems(items)
	if len(items) != 0 {
		t.Errorf("expected empty slice, got %v", items)
	}
}

func TestSortKeyedItems_SingleElement(t *testing.T) {
	items := []testItem{{Key: "a", Value: 1}}
	SortKeyedItems(items)
	want := []testItem{{Key: "a", Value: 1}}
	if !reflect.DeepEqual(items, want) {
		t.Errorf("expected %v, got %v", want, items)
	}
}

func TestSortKeyedItems_AlreadySorted(t *testing.T) {
	items := []testItem{
		{Key: "a", Value: 1},
		{Key: "b", Value: 2},
		{Key: "c", Value: 3},
	}
	want := append([]testItem(nil), items...)
	SortKeyedItems(items)
	if !reflect.DeepEqual(items, want) {
		t.Errorf("expected %v, got %v", want, items)
	}
}

func TestSortKeyedItems_ReverseOrder(t *testing.T) {
	items := []testItem{
		{Key: "c", Value: 3},
		{Key: "b", Value: 2},
		{Key: "a", Value: 1},
	}
	want := []testItem{
		{Key: "a", Value: 1},
		{Key: "b", Value: 2},
		{Key: "c", Value: 3},
	}
	SortKeyedItems(items)
	if !reflect.DeepEqual(items, want) {
		t.Errorf("expected %v, got %v", want, items)
	}
}

func TestSortKeyedItems_DuplicateKeys(t *testing.T) {
	items := []testItem{
		{Key: "b", Value: 1},
		{Key: "a", Value: 2},
		{Key: "b", Value: 3},
	}
	want := []testItem{
		{Key: "a", Value: 2},
		{Key: "b", Value: 1},
		{Key: "b", Value: 3},
	}
	SortKeyedItems(items)
	if !reflect.DeepEqual(items, want) {
		t.Errorf("expected %v, got %v", want, items)
	}
}
