package keys

import (
	"strings"
	"testing"
	"unicode"

	"github.com/stretchr/testify/assert"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
)

// The spec's sort-key table: every key is its column's first letter.
func TestSortKeys_FirstLetters(t *testing.T) {
	t.Parallel()
	for cat, want := range map[domain.Category]map[string]string{
		domain.DedicatedAICluster: {"Name": "n", "Tenant": "t", "Internal": "i", "Usage": "u", "Size": "s", "Age": "a"},
		domain.ImportedModel:      {"Name": "n", "Tenant": "t", "Size": "s", "Context": "c", "Vendor": "v"},
		domain.GPUNode:            {"Name": "n", "Free": "f", "Type": "t", "Age": "a"},
		domain.GPUPool:            {"Name": "n", "Size": "s", "GPUs": "g"},
		domain.Tenant:             {"Name": "n", "Internal": "i", "Status": "s"},
		domain.Environment:        {"Name": "n", "Type": "t"},
	} {
		assert.Equal(t, want, SortKeys(cat), "%s", cat)
	}
}

// A clash takes the next free letter of the later column's name.
func TestDeriveSortKeys_Fallback(t *testing.T) {
	t.Parallel()
	assert.Equal(t,
		map[string]string{"Status": "s", "Size": "i", "Sizzle": "z"},
		deriveSortKeys([]string{"Status", "Size", "Sizzle"}))
}

// Every sort key is unique within its category, so s⟨key⟩ is unambiguous.
func TestSortKeys_UniquePerCategory(t *testing.T) {
	t.Parallel()
	for _, cat := range domain.Categories {
		cols := sortColumns(cat)
		keys := SortKeys(cat)
		assert.Len(t, keys, len(cols), "%s: a column has no key", cat)
		seen := map[string]string{}
		for col, k := range keys {
			if prev, dup := seen[k]; dup {
				t.Errorf("%s: %q and %q both sort on s %s", cat, prev, col, k)
			}
			seen[k] = col
		}
	}
}

// Shift+letter acts on the row, and means the same action in every
// category (no more R = Restart here, Sort Regions there).
func TestShiftLetters_OneMeaningEverywhere(t *testing.T) {
	t.Parallel()
	meaning := map[string]string{}
	for _, cat := range domain.Categories {
		for _, mode := range []common.ViewMode{common.ListView, common.DetailsView} {
			km := ResolveKeys(cat, mode)
			for _, b := range append(append(km.Global, km.Mode...), km.Context...) {
				for _, k := range b.Keys() {
					r := []rune(k)
					if len(r) != 1 || !unicode.IsUpper(r[0]) {
						continue
					}
					if prev, ok := meaning[k]; ok && prev != b.Help().Desc {
						t.Errorf("%s means %q and %q", k, prev, b.Help().Desc)
					}
					meaning[k] = b.Help().Desc
				}
			}
		}
	}
}

// Sort sequences are shown as the key in help, and no single-key binding
// is a sequence leader.
func TestSortSequences(t *testing.T) {
	t.Parallel()
	km := ResolveKeys(domain.DedicatedAICluster, common.ListView)
	found := false
	for _, b := range append(append(km.Global, km.Mode...), km.Context...) {
		for _, k := range b.Keys() {
			assert.NotEqual(t, SortLeader, k, "%s binds the sort leader", b.Help().Desc)
		}
		if b.Help().Desc == SortPrefix+common.TenantCol {
			found = true
			assert.Equal(t, "s t", b.Help().Key)
		}
		if strings.HasPrefix(b.Help().Desc, SortPrefix) {
			assert.True(t, strings.HasPrefix(b.Help().Key, SortLeader+" "), b.Help().Desc)
		}
	}
	assert.True(t, found)
}
