package tui

import (
	"slices"
	"testing"

	"charm.land/bubbles/v2/key"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"
	"github.com/jingle2008/toolkit/pkg/models"
)

// binds reports whether cat's list view binds a key that b is pressed
// with (Terminate is Delete under GPU nodes' name: the same key).
func binds(cat domain.Category, b key.Binding) bool {
	return slices.ContainsFunc(keys.ResolveKeys(cat, common.ListView).Context, func(c key.Binding) bool {
		return slices.Equal(c.Keys(), b.Keys())
	})
}

// A category binds Delete or Restart exactly when it says what that key
// confirms: a binding without an entry would open nothing, and an entry
// without a binding is dead.
func TestItemActionTables_MatchBindings(t *testing.T) {
	t.Parallel()
	for _, cat := range domain.Categories() {
		_, del := deleteConfirms[cat]
		assert.Equalf(t, binds(cat, keys.Delete), del, "%s: Delete binding vs deleteConfirms", cat)
		_, restart := restartConfirms[cat]
		assert.Equalf(t, binds(cat, keys.Restart), restart, "%s: Restart binding vs restartConfirms", cat)
	}
}

// A category with no entry opens no modal — rather than confirming
// another category's action, as a default branch once would have.
func TestItemActions_UnmappedCategoryOpensNothing(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.category = domain.GPUPool
	key := models.ScopedItemKey{Name: "pool-a"}

	assert.Nil(t, m.requestConfirm(m.confirmDelete(key, "")))
	assert.Nil(t, m.requestConfirm(m.confirmRestart(key, nil)))
	assert.Equal(t, common.ListView, m.viewMode, "no modal opened")
}

// Each entry names its category's own action, under the row it acts on.
func TestItemActions_NameTheirAction(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	key := models.ScopedItemKey{Scope: "s", Name: "n"}
	for cat, want := range map[domain.Category]string{
		domain.DedicatedAICluster: "delete dac",
		domain.GPUNode:            "terminate node",
		domain.InferenceService:   "delete inference_service",
	} {
		m.category = cat
		c := m.confirmDelete(key, "")
		require.NotNilf(t, c.run, "%s", cat)
		assert.Equalf(t, want, c.action.Verb+" "+c.action.Kind, "%s", cat)
		assert.Equalf(t, itemKeyString(key), c.target, "%s", cat)
	}
	for cat, want := range map[domain.Category]string{
		domain.GPUNode:          "restart node",
		domain.InferenceService: "restart inference_service",
	} {
		m.category = cat
		c := m.confirmRestart(key, nil)
		require.NotNilf(t, c.run, "%s", cat)
		assert.Equalf(t, want, c.action.Verb+" "+c.action.Kind, "%s", cat)
	}
}
