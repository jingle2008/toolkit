/*
Package recentenv tracks the recently used environments (and therefore
kube contexts) behind the TUI's 0–5 quick-switch keys.

Each environment occupies a fixed slot until it is evicted, so a slot
number stays bound to the same context for muscle memory. A new
environment takes the lowest free slot; once all slots are full it
replaces the least recently used one. Recency only decides eviction —
it never renumbers slots.
*/
package recentenv

import (
	"errors"
	"fmt"
	"os"
	"slices"

	"sigs.k8s.io/yaml"

	"github.com/jingle2008/toolkit/internal/fileutil"
	"github.com/jingle2008/toolkit/pkg/models"
)

// Slots is the number of quick-switch slots (keys 0 through Slots-1).
const Slots = 6

// Entry is an environment bound to a slot.
type Entry struct {
	Slot int
	Env  models.Environment
}

// List holds the slot assignments. entries is ordered most recently used
// first, so the eviction victim is always the last element.
type List struct {
	entries []Entry
}

// fileFormat is the on-disk shape. Entries are written most recently used
// first, which is how recency survives a restart.
type fileFormat struct {
	RecentContexts []fileEntry `json:"recentContexts"`
}

type fileEntry struct {
	Slot   int    `json:"slot"`
	Realm  string `json:"realm"`
	Type   string `json:"type"`
	Region string `json:"region"`
}

// Touch marks env as most recently used, assigning it a slot if it has none.
// It reports whether the list changed.
func (l *List) Touch(env models.Environment) bool {
	if i := l.index(env); i >= 0 {
		if i == 0 {
			return false
		}
		e := l.entries[i]
		l.entries = slices.Insert(slices.Delete(l.entries, i, i+1), 0, e)
		return true
	}

	var slot int
	if len(l.entries) < Slots {
		slot = l.lowestFreeSlot()
	} else {
		slot = l.entries[len(l.entries)-1].Slot
		l.entries = l.entries[:len(l.entries)-1]
	}
	l.entries = slices.Insert(l.entries, 0, Entry{Slot: slot, Env: env})
	return true
}

// Get returns the environment in slot, if any.
func (l *List) Get(slot int) (models.Environment, bool) {
	for _, e := range l.entries {
		if e.Slot == slot {
			return e.Env, true
		}
	}
	return models.Environment{}, false
}

// BySlot returns the occupied entries ordered by slot number.
func (l *List) BySlot() []Entry {
	out := slices.Clone(l.entries)
	slices.SortFunc(out, func(a, b Entry) int { return a.Slot - b.Slot })
	return out
}

// Len returns the number of occupied slots.
func (l *List) Len() int {
	return len(l.entries)
}

// Retain drops entries whose environment fails keep, freeing their slots.
// It reports whether the list changed.
func (l *List) Retain(keep func(models.Environment) bool) bool {
	before := len(l.entries)
	l.entries = slices.DeleteFunc(l.entries, func(e Entry) bool { return !keep(e.Env) })
	return len(l.entries) != before
}

// Clone returns an independent copy, safe to hand to an async writer.
func (l *List) Clone() *List {
	return &List{entries: slices.Clone(l.entries)}
}

func (l *List) index(env models.Environment) int {
	return slices.IndexFunc(l.entries, func(e Entry) bool { return e.Env.Equals(env) })
}

func (l *List) lowestFreeSlot() int {
	for s := range Slots {
		if !slices.ContainsFunc(l.entries, func(e Entry) bool { return e.Slot == s }) {
			return s
		}
	}
	return -1 // unreachable: callers check len(entries) < Slots
}

// Load reads the list from path. A missing file yields an empty list.
// Malformed entries (out-of-range or duplicate slots, duplicate or
// incomplete environments) are dropped rather than failing the load.
func Load(path string) (*List, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path is the app-owned state file
	if errors.Is(err, os.ErrNotExist) {
		return &List{}, nil
	}
	if err != nil {
		return &List{}, fmt.Errorf("read recent contexts: %w", err)
	}
	var f fileFormat
	if err := yaml.Unmarshal(data, &f); err != nil {
		return &List{}, fmt.Errorf("parse recent contexts: %w", err)
	}

	l := &List{}
	for _, fe := range f.RecentContexts {
		e := Entry{Slot: fe.Slot, Env: models.Environment{Type: fe.Type, Region: fe.Region, Realm: fe.Realm}}
		if e.Slot < 0 || e.Slot >= Slots || len(l.entries) == Slots {
			continue
		}
		if e.Env.Type == "" || e.Env.Region == "" || e.Env.Realm == "" {
			continue
		}
		if _, taken := l.Get(e.Slot); taken || l.index(e.Env) >= 0 {
			continue
		}
		l.entries = append(l.entries, e)
	}
	return l, nil
}

// Save writes the list to path atomically, creating parent directories.
func (l *List) Save(path string) error {
	f := fileFormat{RecentContexts: make([]fileEntry, 0, len(l.entries))}
	for _, e := range l.entries {
		f.RecentContexts = append(f.RecentContexts, fileEntry{
			Slot: e.Slot, Realm: e.Env.Realm, Type: e.Env.Type, Region: e.Env.Region,
		})
	}
	data, err := yaml.Marshal(f)
	if err != nil {
		return fmt.Errorf("marshal recent contexts: %w", err)
	}
	if err := fileutil.WriteFileAtomic(path, data, 0o600); err != nil {
		return fmt.Errorf("write recent contexts: %w", err)
	}
	return nil
}
