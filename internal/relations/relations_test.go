package relations

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/pkg/models"
)

// keySets indexes every exported key set by the category it scopes.
var keySets = map[domain.Category]interface{ Parents() []domain.Category }{
	domain.LimitTenancyOverride:            LimitTenancyOverride,
	domain.ConsolePropertyTenancyOverride:  ConsolePropertyTenancyOverride,
	domain.PropertyTenancyOverride:         PropertyTenancyOverride,
	domain.LimitRegionalOverride:           LimitRegionalOverride,
	domain.ConsolePropertyRegionalOverride: ConsolePropertyRegionalOverride,
	domain.PropertyRegionalOverride:        PropertyRegionalOverride,
	domain.ImportedModel:                   ImportedModel,
	domain.GPUNode:                         GPUNode,
	domain.GPUWorkload:                     GPUWorkload,
	domain.DedicatedAICluster:              DedicatedAICluster,
	domain.InferenceService:                InferenceService,
}

// TestKeysMatchRelations requires a key set for every scoped category,
// whose parents equal the domain relations into it: a missing key would
// make a relation unfilterable, an extra one dead code.
func TestKeysMatchRelations(t *testing.T) {
	t.Parallel()
	for _, cat := range domain.Categories() {
		want := cat.ScopedBy()
		slices.Sort(want)
		ks, ok := keySets[cat]
		if !ok {
			assert.Emptyf(t, want, "%s is scoped by %v but has no key set", cat, want)
			continue
		}
		assert.Equalf(t, want, ks.Parents(), "%s: key-set parents differ from relations", cat)
	}
}

func TestGrouped_Filter(t *testing.T) {
	t.Parallel()
	m := map[string][]models.GPUWorkload{
		"node-a": {{Name: "pod-1", Namespace: "amaaaaaa-dac1"}, {Name: "pod-2", Namespace: "amaaaaaa-dac2"}},
		"node-b": {{Name: "pod-3", Namespace: "amaaaaaa-dac1"}},
	}
	names := func(g map[string][]models.GPUWorkload) []string {
		var out []string
		for _, ws := range g {
			for _, w := range ws {
				out = append(out, w.Name)
			}
		}
		slices.Sort(out)
		return out
	}

	// ByField: across groups.
	assert.Equal(t, []string{"pod-1", "pod-3"},
		names(GPUWorkload.Filter(m, &domain.Scope{Category: domain.DedicatedAICluster, Name: "amaaaaaa-dac1"}, "", nil)))
	// ByGroup: one group.
	assert.Equal(t, []string{"pod-1", "pod-2"},
		names(GPUWorkload.Filter(m, &domain.Scope{Category: domain.GPUNode, Name: "node-a"}, "", nil)))
	// Nil and unrelated scopes don't narrow.
	assert.Len(t, names(GPUWorkload.Filter(m, nil, "", nil)), 3)
	assert.Len(t, names(GPUWorkload.Filter(m, &domain.Scope{Category: domain.GPUPool, Name: "x"}, "", nil)), 3)
}

func TestFlat_Keep(t *testing.T) {
	t.Parallel()
	keep := LimitRegionalOverride.Keep(&domain.Scope{Category: domain.LimitDefinition, Name: "l1"})
	assert.True(t, keep(models.LimitRegionalOverride{Name: "l1"}))
	assert.False(t, keep(models.LimitRegionalOverride{Name: "l2"}))
	assert.Nil(t, LimitRegionalOverride.Keep(nil))
	assert.Nil(t, LimitRegionalOverride.Keep(&domain.Scope{Category: domain.Tenant, Name: "t"}))
}

// ParentName is the drill-in run in reverse: filtering by the parent name
// it returns keeps the item.
func TestParentName_InvertsFilter(t *testing.T) {
	t.Parallel()
	w := models.GPUWorkload{Name: "pod-1", Node: "node-a", Namespace: "amaaaaaa-dac1", Runtime: "vllm", Owner: &models.Tenant{Name: "Acme"}}
	m := map[string][]models.GPUWorkload{"node-a": {w}}
	for _, parent := range GPUWorkload.Parents() {
		name, ok := GPUWorkload.ParentName(parent, "node-a", w)
		require.Truef(t, ok, "%s", parent)
		got := GPUWorkload.Filter(m, &domain.Scope{Category: parent, Name: name}, "", nil)
		assert.Lenf(t, got["node-a"], 1, "%s %q", parent, name)
	}

	// No parent row: an unresolved owner, a legacy endpoint's runtime.
	_, ok := GPUWorkload.ParentName(domain.Tenant, "node-a", models.GPUWorkload{})
	assert.False(t, ok)
	_, ok = InferenceService.ParentName(domain.ServingRuntime, "ns", models.InferenceService{Runtime: "r", Legacy: true})
	assert.False(t, ok)
	_, ok = InferenceService.ParentName(domain.GPUPool, "ns", models.InferenceService{})
	assert.False(t, ok)

	name, ok := LimitRegionalOverride.ParentName(domain.LimitDefinition, models.LimitRegionalOverride{Name: "l1"})
	assert.True(t, ok)
	assert.Equal(t, "l1", name)
}

// The two model categories split endpoints by model name, so an endpoint
// relates to exactly one of them.
func TestModelKeys_Disjoint(t *testing.T) {
	t.Parallel()
	for _, s := range []models.InferenceService{
		{Model: "llama"},
		{Model: "amaaaaaa-x", ImportedModel: true},
		{Model: "amaaaaaa-y"}, // tenant-labelled ClusterBaseModel ref
	} {
		_, bm := InferenceService.ParentName(domain.BaseModel, "ns", s)
		_, im := InferenceService.ParentName(domain.ImportedModel, "ns", s)
		assert.NotEqualf(t, bm, im, "%q must relate to exactly one model category", s.Model)
	}
}
