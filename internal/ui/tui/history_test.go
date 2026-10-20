package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	"github.com/jingle2008/toolkit/pkg/models"
)

// histOf is a history of unscoped entries, one per category.
func histOf(cats ...domain.Category) []historyEntry {
	out := make([]historyEntry, len(cats))
	for i, c := range cats {
		out[i] = historyEntry{category: c}
	}
	return out
}

// histCategories is m's history, categories only.
func histCategories(m *Model) []domain.Category {
	out := make([]domain.Category, len(m.history))
	for i, e := range m.history {
		out[i] = e.category
	}
	return out
}

const histDAC = "amaaaaaa-dac1"

// newHistoryModel loads a tenant with limit overrides, a DAC, two pools of
// nodes, and pods on them: one DAC pod on each of n1 and n2, plus a
// non-DAC pod on n1.
func newHistoryModel(t *testing.T) *Model {
	t.Helper()
	m := newTestModel(t)
	m.viewMode = common.ListView
	ov := func(name string) models.LimitTenancyOverride {
		return models.LimitTenancyOverride{LimitRegionalOverride: models.LimitRegionalOverride{Name: name}}
	}
	m.dataset.Tenants = []models.Tenant{{Name: "Acme", ID: "ocid1.tenancy.oc1..acme"}, {Name: "Other", ID: "ocid1.tenancy.oc1..other"}}
	m.dataset.LimitDefinitionGroup = models.LimitDefinitionGroup{Values: []models.LimitDefinition{{Name: "l1"}, {Name: "l2"}}}
	m.dataset.LimitTenancyOverrideMap = map[string][]models.LimitTenancyOverride{
		"Acme":  {ov("l1"), ov("l2")},
		"Other": {ov("l1")},
	}
	m.dataset.DedicatedAIClusterMap = map[string][]models.DedicatedAICluster{"Acme": {{Name: histDAC}}}
	m.dataset.GPUNodeMap = map[string][]models.GPUNode{
		"p1": {{Name: "n1", NodePool: "p1"}},
		"p2": {{Name: "n2", NodePool: "p2"}, {Name: "n3", NodePool: "p2"}},
	}
	m.dataset.GPUWorkloadMap = map[string][]models.GPUWorkload{
		"n1": {{Name: "dac-pod-1", Node: "n1", Namespace: histDAC}, {Name: "other-pod", Node: "n1", Namespace: "other"}},
		"n2": {{Name: "dac-pod-2", Node: "n2", Namespace: histDAC}},
	}
	m.history, m.historyIdx = histOf(m.category), 0
	return m
}

// shownNames is the first column of the rows m shows now.
func shownNames(m *Model) []string {
	m.updateRows(true)
	out := make([]string, len(m.rawRows))
	for i, r := range m.rawRows {
		out[i] = r[0]
	}
	return out
}

// drill drills into name from the current (parent) category, as Enter does.
func drill(m *Model, child domain.Category, name string) {
	m.scope = &domain.Scope{Category: m.category, Name: name}
	m.updateCategory(child)
}

func back(t *testing.T, m *Model) {
	t.Helper()
	_, _ = m.Update(keyPress("["))
}

// Going to a related row changes the scope; [ must still return to the
// list as it was visited, not to it under the new scope.
func TestHistory_BackAfterGoToRestoresScope(t *testing.T) {
	t.Parallel()
	m := newHistoryModel(t)
	drill(m, domain.LimitTenancyOverride, "Acme")
	require.ElementsMatch(t, []string{"l1", "l2"}, shownNames(m))

	m.goTo(domain.Scope{Category: domain.LimitDefinition, Name: "l1"})
	require.Equal(t, domain.LimitDefinition, m.category)

	back(t, m)
	assert.Equal(t, domain.LimitTenancyOverride, m.category)
	assert.Equal(t, &domain.Scope{Category: domain.Tenant, Name: "Acme"}, m.scope)
	assert.ElementsMatch(t, []string{"l1", "l2"}, shownNames(m), "Acme's overrides, not every tenant's l1")
}

// GPUWorkload › DAC, then a node's pods: [ [ returns to the DAC's pods.
func TestHistory_BackAfterDrillRestoresScope(t *testing.T) {
	t.Parallel()
	m := newHistoryModel(t)
	m.goTo(domain.Scope{Category: domain.DedicatedAICluster, Name: histDAC})
	drill(m, domain.GPUWorkload, histDAC)
	require.ElementsMatch(t, []string{"dac-pod-1", "dac-pod-2"}, shownNames(m))

	m.updateCategory(domain.GPUNode)
	drill(m, domain.GPUWorkload, "n1")
	require.ElementsMatch(t, []string{"dac-pod-1", "other-pod"}, shownNames(m))

	back(t, m)
	require.Equal(t, domain.GPUNode, m.category)
	back(t, m)
	assert.Equal(t, domain.GPUWorkload, m.category)
	assert.Equal(t, &domain.Scope{Category: domain.DedicatedAICluster, Name: histDAC}, m.scope)
	assert.ElementsMatch(t, []string{"dac-pod-1", "dac-pod-2"}, shownNames(m))

	_, _ = m.Update(keyPress("]"))
	_, _ = m.Update(keyPress("]"))
	assert.Equal(t, &domain.Scope{Category: domain.GPUNode, Name: "n1"}, m.scope, "] restores the node's scope too")
	assert.ElementsMatch(t, []string{"dac-pod-1", "other-pod"}, shownNames(m))
}

// GPUWorkload › node, o to the DAC: [ returns to the node's pods.
func TestHistory_BackAfterGoToFromScopedChild(t *testing.T) {
	t.Parallel()
	m := newHistoryModel(t)
	m.updateCategory(domain.GPUNode)
	drill(m, domain.GPUWorkload, "n1")
	m.goTo(domain.Scope{Category: domain.DedicatedAICluster, Name: histDAC})

	back(t, m)
	assert.Equal(t, domain.GPUWorkload, m.category)
	assert.ElementsMatch(t, []string{"dac-pod-1", "other-pod"}, shownNames(m))
}

// The same list under two scopes is two entries, and moving between them
// recomputes the rows.
func TestHistory_SameCategoryOtherScopeIsNewEntry(t *testing.T) {
	t.Parallel()
	m := newHistoryModel(t)
	m.updateCategory(domain.GPUNode)
	drill(m, domain.GPUWorkload, "n1")
	m.scope = &domain.Scope{Category: domain.GPUNode, Name: "n2"}
	m.updateCategory(domain.GPUWorkload)
	require.Len(t, m.history, 4)
	require.Equal(t, []string{"dac-pod-2"}, shownNames(m))

	back(t, m)
	assert.Equal(t, domain.GPUWorkload, m.category)
	assert.ElementsMatch(t, []string{"dac-pod-1", "other-pod"}, shownNames(m))
}

// esc from a node's pods returns to the nodes as they were listed (under
// their pool), with the cursor on the node drilled from; ] goes back in.
func TestHistory_EscRestoresParentScopeAndCursor(t *testing.T) {
	t.Parallel()
	m := newHistoryModel(t)
	m.updateCategory(domain.GPUPool)
	drill(m, domain.GPUNode, "p2")
	require.Equal(t, []string{"n2", "n3"}, shownNames(m))
	drill(m, domain.GPUWorkload, "n3")

	_, _ = m.Update(keyPress("esc"))
	require.Equal(t, domain.GPUNode, m.category)
	assert.Equal(t, 2, m.historyIdx, "back in history, not pushed")
	assert.Equal(t, []string{"n2", "n3"}, shownNames(m), "p2's nodes, as visited")
	assert.Equal(t, "n3", m.selectedRawRow()[0], "the cursor lands on the node drilled in from")

	_, _ = m.Update(keyPress("]"))
	assert.Equal(t, domain.GPUWorkload, m.category)
	assert.Equal(t, &domain.Scope{Category: domain.GPUNode, Name: "n3"}, m.scope)
}

// esc clearing a scope in place is recorded, so [ ] don't bring it back.
func TestHistory_EscClearingScopeIsRecorded(t *testing.T) {
	t.Parallel()
	m := newHistoryModel(t)
	m.updateCategory(domain.GPUNode)
	drill(m, domain.GPUWorkload, "n1")
	m.backToLastState()
	assert.Nil(t, m.history[m.historyIdx].scope)
}

// An entry keeps its own copy of the scope.
func TestHistory_EntryDoesNotAliasScope(t *testing.T) {
	t.Parallel()
	m := newHistoryModel(t)
	drill(m, domain.LimitTenancyOverride, "Acme")
	m.scope.Name = "Other"
	assert.Equal(t, "Acme", m.history[m.historyIdx].scope.Name)
}

// An environment switch drops every entry's scope: they name rows of the
// old environment.
func TestHistory_EnvSwitchClearsScopes(t *testing.T) {
	t.Parallel()
	m := newHistoryModel(t)
	m.goTo(domain.Scope{Category: domain.DedicatedAICluster, Name: histDAC})
	drill(m, domain.GPUWorkload, histDAC)
	m.switchEnvironment(models.Environment{Type: "preprod", Region: "us-ashburn-1", Realm: "realm"}, domain.GPUWorkload)
	for i, e := range m.history {
		assert.Nilf(t, e.scope, "entry %d", i)
	}
	assert.Nil(t, m.landOn)
}
