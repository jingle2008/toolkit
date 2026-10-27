package tui

import (
	"context"
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/infra/k8s"
	"github.com/jingle2008/toolkit/internal/logging"
	"github.com/jingle2008/toolkit/internal/mutation"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	"github.com/jingle2008/toolkit/pkg/models"
)

// Bubble Tea runs each tea.Cmd on its own goroutine, alongside Update.
// These tests run a Cmd while the model changes under it: under -race
// they catch a Cmd reading the model, and the assertions catch one that
// sees state from after it was built.

// runConcurrently runs cmd on its own goroutine while mutate runs on
// this one, and returns cmd's message.
func runConcurrently(cmd tea.Cmd, mutate func()) tea.Msg {
	var (
		wg  sync.WaitGroup
		msg tea.Msg
	)
	wg.Go(func() { msg = cmd() })
	mutate()
	wg.Wait()
	return msg
}

// The export Cmd writes the table as it was when export was pressed,
// whatever Update does to the model meanwhile.
func TestExportTableCSVCmd_WritesSnapshotTakenInUpdate(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.category = domain.Tenant
	m.updateColumns()
	want := make([]string, len(m.headers))
	for i, h := range m.headers {
		want[i] = h.text
	}

	path := filepath.Join(t.TempDir(), "out.csv")
	cmd := m.exportTableCSVCmd(path)
	msg := runConcurrently(cmd, func() {
		m.headers = []header{{text: "changed"}}
		m.filter = "no-such-tenant"
		m.showFaulty = !m.showFaulty
		m.dataset = &models.Dataset{}
	})
	require.IsType(t, exportDoneMsg{}, msg)

	data, err := os.ReadFile(filepath.Clean(path))
	require.NoError(t, err)
	records, err := csv.NewReader(strings.NewReader(string(data))).ReadAll()
	require.NoError(t, err)
	require.Len(t, records, 2, "header plus the one tenant")
	assert.Len(t, records[0], len(want))
	assert.Equal(t, "tenant1", records[1][0])
}

func newGPUPoolDetailModel(t *testing.T) *Model {
	t.Helper()
	m := newTestModel(t)
	m.category = domain.GPUPool
	m.dataset.GPUPools = []models.GPUPool{{Name: "pool1", Status: "BEFORE"}}
	m.selectedKey = "pool1"
	m.viewMode = common.DetailsView
	return m
}

func enrichPools(m *Model) {
	m.handleUpdateDoneMsg(updateDoneMsg{
		category: domain.GPUPool,
		env:      m.environment,
		pools:    []models.GPUPool{{Name: "pool1", ID: "ocid1.pool", Status: "AFTER"}},
	})
}

// The details Cmd renders the pool as Update saw it; an enrichment
// landing meanwhile replaces the pool rather than writing into it.
func TestUpdateContentAsync_RendersCopyWhileEnrichmentLands(t *testing.T) {
	t.Parallel()
	m := newGPUPoolDetailModel(t)

	cmd := m.updateContentAsync()
	require.NotNil(t, cmd)
	msg := runConcurrently(cmd, func() { enrichPools(m) })
	rendered, ok := msg.(detailContentRenderedMsg)
	require.True(t, ok)
	require.NoError(t, rendered.Err)
	assert.Contains(t, rendered.Content, "BEFORE")
	assert.NotContains(t, rendered.Content, "AFTER")
	assert.Equal(t, "AFTER", m.dataset.GPUPools[0].Status, "the enrichment still applies")
}

// The copy Cmd copies the item as it was when the key was pressed.
//
//nolint:paralleltest // swaps the package-level clipboard seams
func TestCopyItemJSON_CopiesSnapshotWhileEnrichmentLands(t *testing.T) {
	written := stubClipboard(t, "", nil)
	m := newGPUPoolDetailModel(t)

	cmd := m.copyItemJSONByChoice()
	runConcurrently(cmd, func() { enrichPools(m) })
	assert.Contains(t, *written, "BEFORE")
	assert.NotContains(t, *written, "AFTER")
}

// Enrichment, a cordon result and a tenant save replace what they change,
// so a pointer a Cmd took into the dataset keeps the old values.
func TestDatasetUpdates_AreCopyOnWrite(t *testing.T) {
	t.Parallel()
	m := newGPUPoolDetailModel(t)
	pool := findItem(m.dataset, domain.GPUPool, "pool1").(*models.GPUPool)
	enrichPools(m)
	assert.Equal(t, "BEFORE", pool.Status)
	assert.Equal(t, "AFTER", m.dataset.GPUPools[0].Status)

	m, key, _ := newGPUNodeModel(t, models.GPUNode{Name: "gpu-1"})
	node := findItem(m.dataset, domain.GPUNode, key).(*models.GPUNode)
	m.handleCordonNodeResultMsg(cordonNodeResultMsg{key: key, state: true, env: m.environment})
	assert.False(t, node.IsSchedulingDisabled)
	assert.True(t, m.dataset.GPUNodeMap["pool1"][0].IsSchedulingDisabled)

	tenant := findItem(m.dataset, domain.Tenant, "tenant1").(*models.Tenant)
	name := "renamed"
	m.applyTenantSave(models.TenantMetadata{ID: "id1", Name: &name})
	assert.Equal(t, "tenant1", tenant.Name)
	assert.Equal(t, "renamed", m.dataset.Tenants[0].Name)

	m.handleDeleteDoneMsg(deleteDoneMsg{category: domain.GPUNode, key: key, env: m.environment})
	assert.Equal(t, "gpu-1", node.Name)
	assert.Empty(t, m.dataset.GPUNodeMap["pool1"])
}

// The rows Cmd works on a copy of the dataset, so Update replacing a
// field meanwhile isn't a race.
func TestUpdateRowsAsync_WhileEnrichmentLands(t *testing.T) {
	t.Parallel()
	m := newGPUPoolDetailModel(t)
	m.viewMode = common.ListView
	msg := runConcurrently(m.updateRowsAsync(), func() { enrichPools(m) })
	require.IsType(t, tableRowsComputedMsg{}, msg)
}

// An action reads the context and logger it was built with, not the
// model's as they are when it runs.
func TestCordonNode_ActionReadsNothingOfTheModel(t *testing.T) {
	t.Parallel()
	fakeSetCordon := func(ctx context.Context, _, _, _ string, _ bool) (bool, error) {
		return true, ctx.Err()
	}
	m, key, sink := newGPUNodeModel(t, models.GPUNode{Name: "gpu-1"})
	m.deps.SetCordon = fakeSetCordon
	cmd := m.cordonNode(findItem(m.dataset, domain.GPUNode, key), key, mutation.CordonNode)
	require.NotNil(t, cmd)

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	var msg tea.Msg
	runConcurrently(func() tea.Msg { msg = actionResult(t, cmd); return nil }, func() {
		m.parentCtx = cancelled
		m.logger = logging.Discard()
	})
	res, ok := msg.(cordonNodeResultMsg)
	require.True(t, ok)
	require.NoError(t, res.err)
	assert.Len(t, auditRecords(sink), 2, "audited to the logger it was built with")
}

func TestPlanDeleteInferenceService_ReadsNothingOfTheModel(t *testing.T) {
	t.Parallel()
	fakePlanOrphanISVC := func(ctx context.Context, _, _, _, _ string) (k8s.OrphanDeleteOutcome, error) {
		return k8s.OrphanDeleted, ctx.Err()
	}
	m := newInferenceServiceModel(t)
	m.deps.PlanOrphanISVC = fakePlanOrphanISVC
	cmd := m.handleItemActions(ctrlX())
	require.NotNil(t, cmd)

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	msg := runConcurrently(cmd, func() { m.parentCtx = cancelled })
	planned, ok := msg.(isvcDeletePlannedMsg)
	require.True(t, ok)
	require.NoError(t, planned.err)
}

// Loads, a repo reload and a pool reload install a new dataset rather
// than write into the one a Cmd took: the Cmd reads its tenants and the
// owners resolved on its workloads and services while they land, and
// sees them as they were.
func TestDatasetLoads_ReplaceTheDataset(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.dataset.GPUWorkloadMap = map[string][]models.GPUWorkload{
		"node1": {{Name: "w1", TenantID: "id1", Namespace: "ns1"}},
	}
	m.dataset.InferenceServiceMap = map[string][]models.InferenceService{
		"ns1": {{Name: "isvc1", Namespace: "ns1", TenantID: "id1"}},
	}
	resolveTenantOwners(m.dataset)
	prev := m.dataset
	workload := prev.GPUWorkloadMap["node1"][0]
	service := prev.InferenceServiceMap["ns1"][0]

	read := func() tea.Msg {
		var seen []any
		for _, ws := range prev.GPUWorkloadMap {
			for _, w := range ws {
				seen = append(seen, w.TenantID, w.Owner.Name)
			}
		}
		for _, ss := range prev.InferenceServiceMap {
			for _, s := range ss {
				seen = append(seen, s.TenantID, s.Owner.Name)
			}
		}
		for _, tn := range prev.Tenants {
			seen = append(seen, tn.Name)
		}
		return append(seen, len(prev.GPUPools), len(prev.DedicatedAIClusterMap))
	}
	runConcurrently(read, func() {
		gen := m.gens.msg
		m.handleDedicatedAIClustersLoaded(map[string][]models.DedicatedAICluster{
			"id1": {{Name: "dac1", TenantID: "id1"}},
		}, gen)
		m.handleTenancyOverridesLoaded(models.TenancyOverrideGroup{
			Tenants: []models.Tenant{{Name: "renamed", ID: "id1"}},
		}, gen)
		m.mergeRepoDataset(&models.Dataset{Tenants: []models.Tenant{{Name: "fresh", ID: "id1"}}})
		m.handleGPUPoolsReloaded(gpuPoolsReloadedMsg{Items: []models.GPUPool{{Name: "pool1"}}, env: m.environment})
	})

	assert.NotSame(t, prev, m.dataset)
	assert.Equal(t, "tenant1", prev.Tenants[0].Name)
	assert.Equal(t, workload, prev.GPUWorkloadMap["node1"][0])
	assert.Equal(t, service, prev.InferenceServiceMap["ns1"][0])
	assert.Empty(t, prev.GPUPools)
	assert.Nil(t, prev.DedicatedAIClusterMap)

	assert.Equal(t, "fresh", m.dataset.Tenants[0].Name)
	assert.Equal(t, "fresh", m.dataset.GPUWorkloadMap["node1"][0].Owner.Name)
	assert.Equal(t, "fresh", m.dataset.InferenceServiceMap["ns1"][0].Owner.Name)
	assert.Len(t, m.dataset.GPUPools, 1)
	assert.Len(t, m.dataset.DedicatedAIClusterMap["fresh"], 1)
}
