package tui

import (
	"strings"
	"testing"

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
	}})
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
	}})
	require.Equal(t, "new", m.dataset.Tenants[0].Name, "merge still applied while on a k8s category")
	require.Len(t, m.dataset.BaseModels, 1, "k8s field preserved")
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
	require.True(t, strings.Contains(m.View(), "LIVE"),
		"a repo-backed category with repoWatching must show the live indicator")

	m.watch.repoActive = false
	require.False(t, strings.Contains(m.View(), "LIVE"),
		"no indicator when the repo watch is not established")
}
