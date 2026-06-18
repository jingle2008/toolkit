package tui

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
	logging "github.com/jingle2008/toolkit/pkg/infra/logging"
	"github.com/jingle2008/toolkit/pkg/models"
)

// TestStartupLazyCategory_KeepsFullDataset reproduces the "dac -> ld blank"
// bug: starting the TUI on a lazy-loaded category (e.g. DedicatedAICluster)
// made Init issue both the foundational dataset load and an updateCategory
// load. updateCategory bumps the generation, so a generationed dataset load
// looked stale and was dropped — wiping every non-lazy category's data
// (definitions, etc.). The dataset load is now ungenerationed (Gen 0) and
// always applies.
func TestStartupLazyCategory_KeepsFullDataset(t *testing.T) {
	t.Parallel()

	ds := &models.Dataset{
		LimitDefinitionGroup: models.LimitDefinitionGroup{
			Values: []models.LimitDefinition{{Name: "lim1"}},
		},
	}
	m, err := NewModel(
		WithRepoPath("repo"),
		WithEnvironment(models.Environment{Realm: "realm", Type: "type", Region: "region"}),
		WithLoader(fakeLoader{dataset: ds}),
		WithLogger(logging.NewNoOpLogger()),
		WithCategory(domain.DedicatedAICluster),
	)
	require.NoError(t, err)
	m.viewWidth, m.viewHeight = 80, 24

	// The startup dataset message is ungenerationed so it survives a later
	// generation bump.
	datasetMsg, ok := m.loadData()[2]().(datasetLoadedMsg)
	require.True(t, ok)
	require.Equal(t, 0, datasetMsg.Gen, "startup dataset must be ungenerationed (always-apply)")

	// Simulate the lazy-category load that Init issues next: it bumps the
	// generation well past anything the dataset load could have carried.
	m.gen = 5

	// The dataset must still be applied (not dropped as stale), so the
	// definitions are present and `ld` would render.
	m.handleDataMsg(dataMsg{Data: datasetMsg.Dataset, Gen: datasetMsg.Gen})
	require.NotNil(t, m.dataset)
	require.Len(t, m.dataset.LimitDefinitionGroup.Values, 1,
		"non-lazy data must survive the lazy-load generation bump")
}
