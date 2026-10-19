package relations

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/pkg/models"
)

func TestFaultyCategories(t *testing.T) {
	t.Parallel()
	assert.ElementsMatch(t, []domain.Category{
		domain.BaseModel, domain.Tenant, domain.GPUPool, domain.GPUNode, domain.GPUWorkload,
		domain.ServingRuntime, domain.DedicatedAICluster, domain.InferenceService,
	}, FaultyCategories())
	assert.True(t, IsFaultyCategory(domain.GPUNode))
	assert.False(t, IsFaultyCategory(domain.LimitDefinition))
}

// A row is faulty for its own state or for naming a missing parent.
func TestFaultyMap(t *testing.T) {
	t.Parallel()
	ds := integrityDataset()
	// isvc-1 is healthy and its DAC exists; isvc-2 and isvc-4 name a gone
	// DAC; make isvc-3 (on-demand, nothing to miss) unready.
	for i, s := range ds.InferenceServiceMap["llama"] {
		if s.Name == "isvc-3" {
			ds.InferenceServiceMap["llama"][i].Ready = "False"
		}
	}
	for ns := range ds.InferenceServiceMap {
		for i := range ds.InferenceServiceMap[ns] {
			if ds.InferenceServiceMap[ns][i].Name != "isvc-3" {
				ds.InferenceServiceMap[ns][i].Ready = "True"
			}
		}
	}

	got := FaultyMap(ds, domain.InferenceService, ds.InferenceServiceMap)
	var names []string
	for _, items := range got {
		for _, s := range items {
			names = append(names, s.Name)
		}
	}
	slices.Sort(names)
	assert.Equal(t, []string{"isvc-2", "isvc-3", "isvc-4"}, names)
	for g, items := range got {
		assert.NotEmpty(t, items, "group %q kept with no rows", g)
	}
}

func TestFaultySlice(t *testing.T) {
	t.Parallel()
	pools := []models.GPUPool{{Name: "ok", Size: 2, ActualSize: 2}, {Name: "short", Size: 2, ActualSize: 1}}
	got := FaultySlice(&models.Dataset{GPUPools: pools}, domain.GPUPool, pools)
	require.Len(t, got, 1)
	assert.Equal(t, "short", got[0].Name)
	assert.NotNil(t, FaultySlice(&models.Dataset{}, domain.GPUPool, []models.GPUPool{}), "empty, not nil")
}

func TestFaultyCategoryErr(t *testing.T) {
	t.Parallel()
	require.NoError(t, FaultyCategoryErr(domain.GPUNode))
	assert.EqualError(t, FaultyCategoryErr(domain.LimitDefinition),
		"LimitDefinition rows are never faulty; faulty applies to: basemodel, dedicatedaicluster, gpunode, gpupool, gpuworkload, inferenceservice, servingruntime, tenant")
}

func TestCheckHint(t *testing.T) {
	t.Parallel()
	dangling := []Dangling{{Row: "r", Missing: "m"}}
	assert.Empty(t, Check{Child: domain.InferenceService, Parent: domain.DedicatedAICluster}.Hint(), "nothing to fix")
	assert.Contains(t, Check{Child: domain.InferenceService, Parent: domain.DedicatedAICluster, Dangling: dangling}.Hint(),
		"these InferenceService rows name a DedicatedAICluster that doesn't exist")
	assert.Contains(t, Check{Parent: domain.Tenant, Dangling: dangling}.Hint(), "metadata file")

	// An orphan has no tenancy to add to the metadata: label its CR.
	orphan := Dangling{Row: "ImportedModel default/x", Missing: MissingTenancyLabel}
	assert.Equal(t, "set the tenancy-id label on the CRs missing it", Check{Parent: domain.Tenant, Dangling: []Dangling{orphan}}.Hint())
	both := Check{Parent: domain.Tenant, Dangling: []Dangling{orphan, dangling[0]}}.Hint()
	assert.Contains(t, both, "metadata file")
	assert.Contains(t, both, "tenancy-id label")
}

// A missing pool may just be one that failed to load: the GPUNode →
// GPUPool check says so when it reports any.
func TestNotePartialPools(t *testing.T) {
	t.Parallel()
	checks := []Check{
		{Child: domain.GPUNode, Parent: domain.GPUPool, Dangling: []Dangling{{Row: "p/n", Missing: "p"}}, Note: "n1"},
		{Child: domain.GPUWorkload, Parent: domain.GPUNode, Dangling: []Dangling{{Row: "n/w", Missing: "n"}}},
	}
	NotePartialPools(checks, "pools partly loaded")
	assert.Equal(t, "n1; pools partly loaded", checks[0].Note)
	assert.Empty(t, checks[1].Note)

	clean := []Check{{Child: domain.GPUNode, Parent: domain.GPUPool}}
	NotePartialPools(clean, "pools partly loaded")
	assert.Empty(t, clean[0].Note, "no missing pool to explain")
}
