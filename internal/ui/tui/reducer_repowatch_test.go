package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/pkg/models"
)

func TestHandleRepoWatchStarted_SetsWatchingAndArms(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	ch := make(chan struct{})
	cmd := m.handleRepoWatchStarted(repoWatchStartedMsg{Trigger: ch})
	require.True(t, m.watch.repoActive)
	require.NotNil(t, cmd, "must return a re-arm command")
}

func TestHandleRepoWatchClosed_ClearsWatching(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.watch.repoActive = true
	m.handleRepoWatchClosed()
	require.False(t, m.watch.repoActive)
}

// maybeStartRepoWatchCmd re-arms only when the repo watch is down, so manual
// refresh can recover a dropped watch without spawning a duplicate when one is
// already live.
func TestMaybeStartRepoWatchCmd_OnlyWhenDown(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)

	m.watch.repoActive = true
	require.Nil(t, m.maybeStartRepoWatchCmd(), "must not re-arm while the watch is live")

	m.watch.repoActive = false
	cmd := m.maybeStartRepoWatchCmd()
	require.NotNil(t, cmd, "must return a start command when the watch is down")
	// newTestModel's loader doesn't implement RepoWatcher, so the start command
	// resolves to repoWatchClosedMsg — confirming it is the repo-watch starter.
	_, ok := cmd().(repoWatchClosedMsg)
	require.True(t, ok, "re-arm command must be the repo-watch start")
}

func TestHandleRepoWatchTriggered_ReturnsBatch(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.dataset = &models.Dataset{GPUPools: []models.GPUPool{{Name: "p1"}}}
	m.watch.repoTrigger = make(chan struct{})
	cmd := m.handleRepoWatchTriggered()
	require.NotNil(t, cmd, "trigger must produce reload + re-arm commands")
}

func TestHandleDatasetReloaded_MergesPreservingK8s(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.dataset = &models.Dataset{
		Tenants:    []models.Tenant{{Name: "old"}},
		BaseModels: []models.BaseModel{{Name: "bm1"}},
	}
	m.handleDatasetReloaded(datasetReloadedMsg{Dataset: &models.Dataset{
		Tenants: []models.Tenant{{Name: "new"}},
	}, env: m.environment})
	require.Equal(t, "new", m.dataset.Tenants[0].Name, "repo field updated")
	require.Len(t, m.dataset.BaseModels, 1, "k8s field preserved")
}

// On a k8s-backed category the visible table is not recomputed (the merge
// cannot have changed its rows), but the reloaded repo data must still be
// merged into the cached dataset so it is fresh when the user navigates to a
// repo category.
func TestHandleDatasetReloaded_CachesMergeOnK8sCategory(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.category = domain.BaseModel // k8s-backed: NeedsKubeConfig() == true
	m.dataset = &models.Dataset{
		Tenants:    []models.Tenant{{Name: "old"}},
		BaseModels: []models.BaseModel{{Name: "bm1"}},
	}
	m.handleDatasetReloaded(datasetReloadedMsg{Dataset: &models.Dataset{
		Tenants: []models.Tenant{{Name: "new"}},
	}, env: m.environment})
	require.Equal(t, "new", m.dataset.Tenants[0].Name, "merge still applied while on a k8s category")
	require.Len(t, m.dataset.BaseModels, 1, "k8s field preserved")
}

// A tenant renamed in the metadata renames it on cached cluster rows too:
// their owners are re-resolved against the reloaded tenants.
func TestHandleDatasetReloaded_ReResolvesOwners(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.category = domain.DedicatedAICluster
	old := &models.Tenant{Name: "old", ID: "ocid1.tenancy.oc1..acme"}
	m.dataset = &models.Dataset{
		Tenants: []models.Tenant{*old},
		DedicatedAIClusterMap: map[string][]models.DedicatedAICluster{
			"old": {{Name: "dac1", TenantID: "acme", Owner: old}},
		},
	}
	m.handleDatasetReloaded(datasetReloadedMsg{Dataset: &models.Dataset{
		Tenants: []models.Tenant{{Name: "renamed", ID: "ocid1.tenancy.oc1..acme"}},
	}, env: m.environment})
	dacs := m.dataset.DedicatedAIClusterMap["renamed"]
	require.Len(t, dacs, 1, "re-keyed by the new name")
	require.NotNil(t, dacs[0].Owner)
	assert.Equal(t, "renamed", dacs[0].Owner.Name)
}

// A repo edit that breaks the load (a malformed file) keeps the data on
// screen but says so, rather than leaving it stale under LIVE.
func TestHandleDatasetReloaded_FailureWarns(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	before := m.dataset
	cmd := m.handleDatasetReloaded(datasetReloadedMsg{err: errors.New("x.json: failed to decode JSON"), env: m.environment})
	require.NotNil(t, cmd, "a warning toast")
	assert.Same(t, before, m.dataset, "the current data stays")
	require.NotNil(t, m.toasts.active)
	assert.Contains(t, m.toasts.active.msg, "x.json")
}

func TestHandleDatasetReloaded_NilDatasetIgnored(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	before := m.dataset
	m.handleDatasetReloaded(datasetReloadedMsg{Dataset: nil})
	require.Same(t, before, m.dataset, "a nil reload must be ignored")
}

func TestLiveIndicator_ShowsOnRepoCategory(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.viewWidth, m.viewHeight = 100, 20
	m.category = domain.Tenant // repo-backed: NeedsKubeConfig() == false
	m.watch.repoActive = true
	require.True(t, strings.Contains(m.render(), "LIVE"),
		"a repo-backed category with repoWatching must show the live indicator")

	m.watch.repoActive = false
	require.False(t, strings.Contains(m.render(), "LIVE"),
		"no indicator when the repo watch is not established")
}
