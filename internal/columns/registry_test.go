package columns

import (
	"math"
	"testing"

	"github.com/jingle2008/toolkit/internal/domain"
)

// Every concrete Category must have a registered column set.
//
// Skipped only during the bootstrap state (registered == 0).
// All 19 categories are now registered — this is a live invariant.
func TestRegistry_EveryCategoryRegistered(t *testing.T) {
	t.Parallel()
	var missing []domain.Category
	registered := 0
	for _, cat := range domain.Categories {
		if cat == domain.CategoryUnknown {
			continue
		}
		if IsRegistered(cat) {
			registered++
		} else {
			missing = append(missing, cat)
		}
	}
	if registered == 0 {
		t.Skip("bootstrap state: no categories registered yet")
	}
	if len(missing) > 0 {
		t.Errorf("missing %d of %d categories: %v",
			len(missing), registered+len(missing), missing)
	}
}

// Keys must be unique, non-empty; `help` is reserved.
func TestRegistry_KeysValid(t *testing.T) {
	t.Parallel()
	for _, cat := range domain.Categories {
		if cat == domain.CategoryUnknown || !IsRegistered(cat) {
			continue
		}
		keys := KeysFor(cat)
		seen := make(map[string]bool, len(keys))
		var hasDefault bool
		for _, k := range keys {
			if k == "" {
				t.Errorf("%s: empty key", cat)
			}
			if k == "help" {
				t.Errorf("%s: key %q is reserved", cat, k)
			}
			if seen[k] {
				t.Errorf("%s: duplicate key %q", cat, k)
			}
			seen[k] = true
		}
		for _, isDefault := range DefaultsFor(cat) {
			if isDefault {
				hasDefault = true
				break
			}
		}
		if !hasDefault {
			t.Errorf("%s: no column has Default=true", cat)
		}
	}
}

// Ratios per set must sum to ~1.0 (±0.02).
func TestRegistry_RatiosSumToOne(t *testing.T) {
	t.Parallel()
	for _, cat := range domain.Categories {
		if cat == domain.CategoryUnknown || !IsRegistered(cat) {
			continue
		}
		sum := RatioSum(cat)
		if math.Abs(sum-1.0) > 0.02 {
			t.Errorf("%s: ratio sum %.3f outside ±0.02 of 1.0", cat, sum)
		}
	}
}
