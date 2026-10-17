package relations

import (
	"slices"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/pkg/models"
)

// faultyCategories are the categories whose rows can be faulty: the
// TUI's Ctrl+Z, `toolkit get --faulty` and MCP's faulty input list them.
var faultyCategories = []domain.Category{
	domain.BaseModel, domain.Tenant, domain.GPUPool, domain.GPUNode, domain.GPUWorkload,
	domain.ServingRuntime, domain.DedicatedAICluster, domain.InferenceService,
}

// FaultyCategories returns the categories whose rows can be faulty.
func FaultyCategories() []domain.Category { return slices.Clone(faultyCategories) }

// IsFaultyCategory reports whether category's rows can be faulty.
func IsFaultyCategory(category domain.Category) bool {
	return slices.Contains(faultyCategories, category)
}

// FaultyRows returns the faulty test for category's rows, stored under
// group in ds: a row is faulty when its own state is (IsFaulty) or it
// names a parent row that doesn't exist — what `doctor --relations`
// reports as FAIL (DanglingRows). Tenancy gaps (doctor's WARN) don't
// count. A parent counts as missing only if its category is loaded in
// ds, so for a full answer load the parents too.
func FaultyRows(ds *models.Dataset, category domain.Category) func(group string, it models.Faulty) bool {
	dangling := DanglingRows(ds, category)
	return func(group string, it models.Faulty) bool {
		return it.IsFaulty() || (dangling != nil && dangling(group, it))
	}
}

// FaultyMap keeps grouped's faulty rows (FaultyRows), dropping groups
// left empty.
func FaultyMap[T models.Faulty](ds *models.Dataset, category domain.Category, grouped map[string][]T) map[string][]T {
	faulty := FaultyRows(ds, category)
	out := make(map[string][]T)
	for g, items := range grouped {
		var kept []T
		for _, it := range items {
			if faulty(g, it) {
				kept = append(kept, it)
			}
		}
		if len(kept) > 0 {
			out[g] = kept
		}
	}
	return out
}

// FaultySlice keeps items' faulty rows (FaultyRows) of a flat category.
func FaultySlice[T models.Faulty](ds *models.Dataset, category domain.Category, items []T) []T {
	faulty := FaultyRows(ds, category)
	out := make([]T, 0)
	for _, it := range items {
		if faulty("", it) {
			out = append(out, it)
		}
	}
	return out
}
