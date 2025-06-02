package utils

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetConfigPath(t *testing.T) {
	root := "/repo"
	realm := "oc1"
	name := "limits"
	expected := "/repo/limitss/oc1_limits.json"
	assert.Equal(t, expected, getConfigPath(root, realm, name))
}

func TestSortNamedItems(t *testing.T) {
	type named struct{ name string }
	// implement GetName for named
	items := []named{{"b"}, {"a"}, {"c"}}
	// sort using name field
	sort.Slice(items, func(i, j int) bool {
		return items[i].name < items[j].name
	})
	assert.Equal(t, "a", items[0].name)
	assert.Equal(t, "b", items[1].name)
	assert.Equal(t, "c", items[2].name)
}

func TestSortKeyedItems(t *testing.T) {
	type keyed struct{ key string }
	// implement GetKey for keyed
	items := []keyed{{"b"}, {"a"}, {"c"}}
	// sort using key field
	sort.Slice(items, func(i, j int) bool {
		return items[i].key < items[j].key
	})
	assert.Equal(t, "a", items[0].key)
	assert.Equal(t, "b", items[1].key)
	assert.Equal(t, "c", items[2].key)
}
