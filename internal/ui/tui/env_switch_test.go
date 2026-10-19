package tui

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/infra/k8s"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	"github.com/jingle2008/toolkit/pkg/models"
)

var otherEnv = models.Environment{Type: "preprod", Region: "us-ashburn-1", Realm: "oc1"}

// A scope names a row of the old environment, so it doesn't survive a
// switch — including one from the Environment list, which lands on Tenant
// rather than switching in place.
func TestSwitchEnvironment_ClearsScope(t *testing.T) {
	t.Parallel()
	m := newInferenceServiceModel(t)
	m.goTo(domain.Scope{Category: domain.DedicatedAICluster, Name: "dac-1"})
	require.NotNil(t, m.scope)

	m.category = domain.Environment
	m.switchEnvironment(otherEnv, domain.Tenant)
	assert.Nil(t, m.scope)
	m.updateCategory(domain.InferenceService)
	assert.Nil(t, m.scope, "the old DAC no longer filters :is")
}

// A repo reload started before a switch would merge the old environment's
// tenants, overrides and pools into the new one.
func TestReloads_FromPreviousEnvDropped(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.dataset = &models.Dataset{Tenants: []models.Tenant{{Name: "current"}}, GPUPools: []models.GPUPool{{Name: "pool-current"}}}

	m.handleDatasetReloaded(datasetReloadedMsg{Dataset: &models.Dataset{Tenants: []models.Tenant{{Name: "stale"}}}, env: otherEnv})
	assert.Equal(t, "current", m.dataset.Tenants[0].Name)
	assert.Nil(t, m.handleGPUPoolsReloaded(gpuPoolsReloadedMsg{Items: []models.GPUPool{{Name: "pool-stale"}}, env: otherEnv}))
	assert.Equal(t, "pool-current", m.dataset.GPUPools[0].Name)

	m.handleDatasetReloaded(datasetReloadedMsg{Dataset: &models.Dataset{Tenants: []models.Tenant{{Name: "fresh"}}}, env: m.environment})
	assert.Equal(t, "fresh", m.dataset.Tenants[0].Name, "a reload for the current environment still applies")
}

// Pool names and node IPs repeat across environments: a result for the old
// one must not mark the new one's same-named row.
func TestActionResults_FromPreviousEnvLeaveRowsAlone(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	poolKey := "gais_inference_h100_01"
	nodeKey := models.ScopedItemKey{Scope: "pool1", Name: "10.0.0.1"}
	m.dataset.GPUPools = []models.GPUPool{{Name: poolKey, Status: "RUNNING"}}
	m.dataset.GPUNodeMap = map[string][]models.GPUNode{"pool1": {{Name: "10.0.0.1", NodePool: "pool1"}}}

	m.handleGPUPoolScaleResultMsg(gpuPoolScaleResultMsg{key: poolKey, err: errors.New("boom"), env: otherEnv})
	assert.Equal(t, "RUNNING", m.dataset.GPUPools[0].Status)

	m.handleCordonNodeResultMsg(cordonNodeResultMsg{key: nodeKey, state: true, env: otherEnv})
	m.handleRestartNodeResultMsg(restartNodeResultMsg{key: nodeKey, err: errors.New("boom"), env: otherEnv})
	m.handleDeleteDoneMsg(deleteDoneMsg{category: domain.GPUNode, key: nodeKey, env: otherEnv})
	require.Len(t, m.dataset.GPUNodeMap["pool1"], 1, "not removed")
	assert.False(t, m.dataset.GPUNodeMap["pool1"][0].IsSchedulingDisabled)

	m.handleCordonNodeResultMsg(cordonNodeResultMsg{key: nodeKey, state: true, env: m.environment})
	assert.True(t, m.dataset.GPUNodeMap["pool1"][0].IsSchedulingDisabled, "a result for the current environment still applies")
}

// A result that lands after an environment switch is still shown, naming
// the environment it ran in.
func TestActionResults_FromPreviousEnvNameIt(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.handleRestartNodeResultMsg(restartNodeResultMsg{key: "10.0.0.1", err: errors.New("boom"), env: otherEnv})
	require.NotNil(t, m.toasts.active)
	assert.Equal(t, toastError, m.toasts.active.sev)
	assert.Contains(t, m.toasts.active.msg, "("+otherEnv.KubeContext()+")")
}

// A delete plan from the old environment would open a modal over the new
// one's same-named endpoint.
func TestISVCDeletePlan_FromPreviousEnvDropped(t *testing.T) {
	t.Parallel()
	m := newInferenceServiceModel(t)
	m.handleISVCDeletePlannedMsg(isvcDeletePlannedMsg{key: models.ScopedItemKey{Scope: "ns1", Name: "svc1"}, outcome: k8s.OrphanDeleted, env: otherEnv})
	assert.Equal(t, common.ListView, m.viewMode)
	m.handleISVCDeletePlannedMsg(isvcDeletePlannedMsg{key: models.ScopedItemKey{Scope: "ns1", Name: "svc1"}, outcome: k8s.OrphanDeleted, env: m.environment})
	assert.Equal(t, common.ConfirmView, m.viewMode)
}

// The enrichment merges by name: only ID, actual size and status, so a
// pool's repo fields reloaded meanwhile are kept, and a pool added since
// the snapshot keeps its state.
func TestHandleUpdateDoneMsg_MergesEnrichment(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.dataset.GPUPools = []models.GPUPool{{Name: "a", Size: 4, Status: "..."}, {Name: "new", Status: "..."}}
	snapshot := []models.GPUPool{{Name: "a", Size: 2, ID: "ocid1.pool.a", ActualSize: 3, Status: "RUNNING"}}

	m.handleUpdateDoneMsg(updateDoneMsg{category: domain.GPUPool, env: otherEnv, pools: snapshot})
	assert.Equal(t, "...", m.dataset.GPUPools[0].Status, "previous environment dropped")

	m.handleUpdateDoneMsg(updateDoneMsg{category: domain.GPUPool, env: m.environment, pools: snapshot})
	a := m.dataset.GPUPools[0]
	assert.Equal(t, "ocid1.pool.a", a.ID)
	assert.Equal(t, 3, a.ActualSize)
	assert.Equal(t, "RUNNING", a.Status)
	assert.Equal(t, 4, a.Size, "repo-declared size is kept")
	assert.Equal(t, "...", m.dataset.GPUPools[1].Status)
}

// The enrichment job works on a snapshot: the live pool list is never
// written off the Update loop.
func TestUpdateGPUPoolState_SnapshotsPools(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.kubeConfig = "/nonexistent/kubeconfig" // the compartment lookup fails fast
	m.dataset.GPUPools = []models.GPUPool{{Name: "a", Status: "..."}}
	msg, ok := m.updateGPUPoolState()().(updateDoneMsg)
	require.True(t, ok)
	require.Error(t, msg.err)
	assert.Equal(t, m.environment, msg.env)
	require.Len(t, msg.pools, 1)
	msg.pools[0].Status = "changed"
	assert.Equal(t, "...", m.dataset.GPUPools[0].Status, "the message carries a copy")
}

// With the OCI lookup failed, every pool's actual size is unknown: Ctrl+Z
// shows none as faulty, rather than every pool as short of its size.
func TestFaultyPools_EnrichmentFailureFlagsNone(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.dataset.GPUPools = []models.GPUPool{{Name: "a", Size: 4, Status: models.GPUPoolStatusPending}}
	m.handleUpdateDoneMsg(updateDoneMsg{category: domain.GPUPool, env: m.environment, err: errors.New("no OCI session")})
	require.Equal(t, models.GPUPoolStatusUnknown, m.dataset.GPUPools[0].Status)

	rows, _ := computeTableRows(m.dataset, domain.GPUPool, nil, "", "", true, true)
	assert.Empty(t, rows)
}

// Tenants and the three tenancy-override maps load as one group. A switch
// in place on an override category clears its rows, so the group's
// arrival must redraw that category, not just Tenant.
func TestSwitchEnvironment_TenancyOverridesRedraw(t *testing.T) {
	t.Parallel()
	for _, cat := range []domain.Category{
		domain.Tenant, domain.LimitTenancyOverride,
		domain.ConsolePropertyTenancyOverride, domain.PropertyTenancyOverride,
	} {
		t.Run(cat.String(), func(t *testing.T) {
			t.Parallel()
			m := newTestModel(t)
			m.dataset = &models.Dataset{}
			m.updateCategory(cat)
			m.switchEnvironment(otherEnv, cat)
			require.Empty(t, m.table.Rows())

			const tenant = "acme"
			m.handleTenancyOverridesLoaded(models.TenancyOverrideGroup{
				Tenants: []models.Tenant{{ID: "ocid1.tenancy.oc1..acme", Name: tenant}},
				LimitTenancyOverrideMap: map[string][]models.LimitTenancyOverride{
					tenant: {{TenantName: tenant, LimitRegionalOverride: models.LimitRegionalOverride{Name: "l1"}}},
				},
				ConsolePropertyTenancyOverrideMap: map[string][]models.ConsolePropertyTenancyOverride{
					tenant: {{TenantName: tenant}},
				},
				PropertyTenancyOverrideMap: map[string][]models.PropertyTenancyOverride{
					tenant: {{TenantName: tenant}},
				},
			}, m.gens.msg)
			assert.NotEmpty(t, m.table.Rows())
		})
	}
}
