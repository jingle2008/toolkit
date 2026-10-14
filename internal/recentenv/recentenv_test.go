package recentenv

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/pkg/models"
)

func env(region string) models.Environment {
	return models.Environment{Type: "dev", Region: region, Realm: "oc1"}
}

var regions = []string{
	"us-phoenix-1", "us-ashburn-1", "us-chicago-1",
	"uk-london-1", "eu-frankfurt-1", "ap-tokyo-1", "ap-osaka-1",
}

func slotOf(t *testing.T, l *List, e models.Environment) int {
	t.Helper()
	for _, en := range l.BySlot() {
		if en.Env.Equals(e) {
			return en.Slot
		}
	}
	return -1
}

func TestTouch_FillsLowestFreeSlot(t *testing.T) {
	t.Parallel()
	l := &List{}
	for i := range Slots {
		require.True(t, l.Touch(env(regions[i])))
		assert.Equal(t, i, slotOf(t, l, env(regions[i])))
	}
	assert.Equal(t, Slots, l.Len())
}

func TestTouch_ExistingKeepsSlot(t *testing.T) {
	t.Parallel()
	l := &List{}
	l.Touch(env(regions[0]))
	l.Touch(env(regions[1]))
	assert.True(t, l.Touch(env(regions[0])), "moving to front is a change")
	assert.False(t, l.Touch(env(regions[0])), "already most recent")
	assert.Equal(t, 0, slotOf(t, l, env(regions[0])))
	assert.Equal(t, 1, slotOf(t, l, env(regions[1])))
}

func TestTouch_EvictsLeastRecentlyUsedSlot(t *testing.T) {
	t.Parallel()
	l := &List{}
	for i := range Slots {
		l.Touch(env(regions[i]))
	}
	// Refresh slot 0 so slot 1 becomes the LRU victim.
	l.Touch(env(regions[0]))
	l.Touch(env(regions[6]))

	assert.Equal(t, Slots, l.Len())
	assert.Equal(t, 1, slotOf(t, l, env(regions[6])), "new env reuses the evicted slot")
	assert.Equal(t, -1, slotOf(t, l, env(regions[1])))
	assert.Equal(t, 0, slotOf(t, l, env(regions[0])))
}

func TestRetain_FreesSlotForReuse(t *testing.T) {
	t.Parallel()
	l := &List{}
	for i := range 3 {
		l.Touch(env(regions[i]))
	}
	changed := l.Retain(func(e models.Environment) bool { return !e.Equals(env(regions[1])) })
	assert.True(t, changed)
	assert.False(t, l.Retain(func(models.Environment) bool { return true }))

	l.Touch(env(regions[5]))
	assert.Equal(t, 1, slotOf(t, l, env(regions[5])))
}

func TestGet(t *testing.T) {
	t.Parallel()
	l := &List{}
	l.Touch(env(regions[0]))
	got, ok := l.Get(0)
	assert.True(t, ok)
	assert.Equal(t, env(regions[0]), got)
	_, ok = l.Get(1)
	assert.False(t, ok)
}

func TestSaveLoad_RoundTripPreservesSlotsAndRecency(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "nested", "state.yaml")
	l := &List{}
	for i := range Slots {
		l.Touch(env(regions[i]))
	}
	l.Touch(env(regions[0]))
	require.NoError(t, l.Save(path))

	got, err := Load(path)
	require.NoError(t, err)
	assert.Equal(t, l.BySlot(), got.BySlot())

	// Recency survived: slot 1 is still the eviction victim.
	got.Touch(env(regions[6]))
	assert.Equal(t, 1, slotOf(t, got, env(regions[6])))
}

func TestLoad_MissingFileIsEmpty(t *testing.T) {
	t.Parallel()
	l, err := Load(filepath.Join(t.TempDir(), "nope.yaml"))
	require.NoError(t, err)
	assert.Equal(t, 0, l.Len())
}

func TestLoad_DropsMalformedEntries(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "state.yaml")
	data := `recentContexts:
- {slot: 0, realm: oc1, type: dev, region: us-phoenix-1}
- {slot: 0, realm: oc1, type: dev, region: us-ashburn-1}
- {slot: 1, realm: oc1, type: dev, region: us-phoenix-1}
- {slot: 6, realm: oc1, type: dev, region: us-chicago-1}
- {slot: 2, realm: oc1, type: dev}
- {slot: 3, realm: oc1, type: dev, region: uk-london-1}
`
	require.NoError(t, os.WriteFile(path, []byte(data), 0o600))
	l, err := Load(path)
	require.NoError(t, err)
	assert.Equal(t, []Entry{
		{Slot: 0, Env: env("us-phoenix-1")},
		{Slot: 3, Env: env("uk-london-1")},
	}, l.BySlot())
}

func TestLoad_InvalidYAML(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "state.yaml")
	require.NoError(t, os.WriteFile(path, []byte("recentContexts: ["), 0o600))
	l, err := Load(path)
	require.Error(t, err)
	assert.Equal(t, 0, l.Len())
}
