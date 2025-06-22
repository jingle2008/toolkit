package collections

import (
	"reflect"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

type testItem struct {
	Key   string
	Value int
}

func (ti testItem) GetKey() string {
	return ti.Key
}

func TestSortKeyedItems_EmptySlice(t *testing.T) {
	t.Parallel()
	var items []testItem
	SortKeyedItems(items)
	if len(items) != 0 {
		t.Errorf("expected empty slice, got %v", items)
	}
}

func TestSortKeyedItems_SingleElement(t *testing.T) {
	t.Parallel()
	items := []testItem{{Key: "a", Value: 1}}
	SortKeyedItems(items)
	want := []testItem{{Key: "a", Value: 1}}
	if !reflect.DeepEqual(items, want) {
		t.Errorf("expected %v, got %v", want, items)
	}
}

func TestSortKeyedItems_AlreadySorted(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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

func TestSortKeyedItems_Stability(t *testing.T) {
	t.Parallel()
	items := []testItem{
		{Key: "a", Value: 1},
		{Key: "a", Value: 2},
		{Key: "b", Value: 3},
	}
	want := []testItem{
		{Key: "a", Value: 1},
		{Key: "a", Value: 2},
		{Key: "b", Value: 3},
	}
	SortKeyedItems(items)
	if !reflect.DeepEqual(items, want) {
		t.Errorf("expected stable sort, got %v", items)
	}
}

func TestSortKeyedItems_NilSlice(t *testing.T) {
	t.Parallel()
	var items []testItem
	SortKeyedItems(items)
	if len(items) != 0 {
		t.Errorf("expected empty slice, got %v", items)
	}
}

type named struct {
	name string
}

func (n named) GetName() string { return n.name }

type keyed struct {
	key string
}

func (k keyed) GetKey() string { return k.key }

func TestSortNamedItems(t *testing.T) {
	t.Parallel()
	items := []named{
		{name: "zeta"},
		{name: "alpha"},
		{name: "gamma"},
	}
	SortNamedItems(items)
	require.Equal(t, "alpha", items[0].GetName())
	require.Equal(t, "gamma", items[1].GetName())
	require.Equal(t, "zeta", items[2].GetName())
}

func TestSortKeyedItems(t *testing.T) {
	t.Parallel()
	items := []keyed{
		{key: "b"},
		{key: "a"},
		{key: "c"},
	}
	SortKeyedItems(items)
	require.Equal(t, "a", items[0].GetKey())
	require.Equal(t, "b", items[1].GetKey())
	require.Equal(t, "c", items[2].GetKey())
}

// Benchmarks

type benchKeyedItem struct{ key string }

func (bi benchKeyedItem) GetKey() string { return bi.key }

func BenchmarkSortKeyedItems(b *testing.B) {
	sizes := []int{10, 100, 1000, 10000}
	for _, size := range sizes {
		b.Run("size="+strconv.Itoa(size), func(b *testing.B) {
			// Prepare input slice
			items := make([]benchKeyedItem, size)
			for i := range size {
				items[i] = benchKeyedItem{key: strconv.Itoa(size - i)}
			}
			b.ResetTimer()
			for b.Loop() {
				tmp := make([]benchKeyedItem, size)
				copy(tmp, items)
				SortKeyedItems(tmp)
			}
		})
	}
}

type benchNamedItem struct{ name string }

func (bi benchNamedItem) GetName() string { return bi.name }

func BenchmarkSortNamedItems(b *testing.B) {
	sizes := []int{10, 100, 1000, 10000}
	for _, size := range sizes {
		b.Run("size="+strconv.Itoa(size), func(b *testing.B) {
			// Prepare input slice
			items := make([]benchNamedItem, size)
			for i := range size {
				items[i] = benchNamedItem{name: strconv.Itoa(size - i)}
			}
			b.ResetTimer()
			for b.Loop() {
				tmp := make([]benchNamedItem, size)
				copy(tmp, items)
				SortNamedItems(tmp)
			}
		})
	}
}
