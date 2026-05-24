package columns

import (
	"fmt"
	"sort"

	"github.com/jingle2008/toolkit/internal/domain"
)

// IsRegistered reports whether cat has a canonical column set.
// Implementation lives alongside the per-category files; this
// switch is the single edit-site when a new category is added.
func IsRegistered(cat domain.Category) bool {
	switch cat { //nolint:exhaustive
	// Per-category files (added in later tasks) flip these on by
	// adding their case. Until then everything is unregistered.
	}
	return false
}

// KeysFor returns the declared keys for cat in order.
// Returns nil for unregistered categories.
func KeysFor(cat domain.Category) []string {
	switch cat { //nolint:exhaustive
	}
	return nil
}

// DefaultsFor returns the Default flag for each column of cat in
// declared order. The two slices KeysFor / DefaultsFor share the
// same indices; together they're enough to drive shell completion
// and the `--columns help` table.
func DefaultsFor(cat domain.Category) []bool {
	switch cat { //nolint:exhaustive
	}
	return nil
}

// RatioSum returns the sum of Ratio across all columns of cat
// (for the ratios-sum-to-1 registry test).
func RatioSum(cat domain.Category) float64 {
	switch cat { //nolint:exhaustive
	}
	return 0
}

// RenderTable is the single entrypoint the CLI calls. It type-switches
// on cat, applies --columns selection, and produces headers+rows for
// the chosen encoding (table/csv/tsv). headers are uppercased to
// preserve today's CLI table headers (NAME, STATUS, ...); the TUI
// adapter (in internal/ui/tui) uses Titles as-is.
//
// `items` must be the concrete payload for cat. `selected` is the
// parsed --columns list (empty means "use Default columns").
//
//nolint:cyclop // a per-category switch is the contract here
func RenderTable(cat domain.Category, items any, selected []string) ([]string, [][]string, error) {
	switch cat { //nolint:exhaustive
	}
	return nil, nil, fmt.Errorf("category %s is not registered with the columns package", cat)
}

// HelpTable returns a (Key, Title, Default) row per column of cat,
// for the `--columns help` output. Empty if cat is unregistered.
func HelpTable(cat domain.Category) (headers []string, rows [][]string) {
	keys := KeysFor(cat)
	if keys == nil {
		return nil, nil
	}
	titles := TitlesFor(cat)
	defaults := DefaultsFor(cat)
	headers = []string{"KEY", "TITLE", "DEFAULT"}
	rows = make([][]string, len(keys))
	for i, k := range keys {
		def := "no"
		if defaults[i] {
			def = "yes"
		}
		rows[i] = []string{k, titles[i], def}
	}
	return headers, rows
}

// TitlesFor returns the Title for each column of cat in declared order.
func TitlesFor(cat domain.Category) []string {
	switch cat { //nolint:exhaustive
	}
	return nil
}

// sortedKeys returns the keys of a grouped map in sorted order so
// table output is deterministic.
func sortedKeys[T any](m map[string][]T) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
