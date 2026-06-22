package tui

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/infra/loader"
	"github.com/jingle2008/toolkit/pkg/infra/logging"
	"github.com/jingle2008/toolkit/pkg/models"
)

// repoWatchLoader is a fake Composite that also implements RepoWatcher.
type repoWatchLoader struct {
	loader.Composite
	trigger <-chan struct{}
	err     error
}

func (f repoWatchLoader) WatchRepo(context.Context, string) (<-chan struct{}, error) {
	return f.trigger, f.err
}

func TestStartRepoWatchCmd_Started(t *testing.T) {
	t.Parallel()
	ch := make(chan struct{})
	cmd := startRepoWatchCmd(context.Background(), repoWatchLoader{trigger: ch}, "/repo")
	msg := cmd()
	started, ok := msg.(repoWatchStartedMsg)
	require.True(t, ok, "expected repoWatchStartedMsg, got %T", msg)
	require.NotNil(t, started.Trigger)
}

func TestStartRepoWatchCmd_NotAWatcher(t *testing.T) {
	t.Parallel()
	// fakeLoader (defined in the package's existing tests) does not implement
	// RepoWatcher.
	cmd := startRepoWatchCmd(context.Background(), fakeLoader{}, "/repo")
	_, ok := cmd().(repoWatchClosedMsg)
	require.True(t, ok, "a loader without RepoWatcher must yield repoWatchClosedMsg")
}

func TestStartRepoWatchCmd_SetupError(t *testing.T) {
	t.Parallel()
	cmd := startRepoWatchCmd(context.Background(), repoWatchLoader{err: errors.New("nope")}, "/repo")
	_, ok := cmd().(repoWatchClosedMsg)
	require.True(t, ok, "a WatchRepo error must yield repoWatchClosedMsg")
}

func TestWaitForRepoTriggerCmd_TickAndClose(t *testing.T) {
	t.Parallel()
	ch := make(chan struct{}, 1)
	ch <- struct{}{}
	_, ok := waitForRepoTriggerCmd(ch)().(repoWatchTriggeredMsg)
	require.True(t, ok, "a value on the channel must yield repoWatchTriggeredMsg")

	closed := make(chan struct{})
	close(closed)
	_, ok = waitForRepoTriggerCmd(closed)().(repoWatchClosedMsg)
	require.True(t, ok, "a closed channel must yield repoWatchClosedMsg")
}

func TestReloadDatasetCmd_SuccessAndError(t *testing.T) {
	t.Parallel()
	ds := &models.Dataset{Tenants: []models.Tenant{{Name: "t"}}}
	okLoader := fakeLoader{dataset: ds}
	msg := reloadDatasetCmd(context.Background(), okLoader, "/repo", models.Environment{}, logging.NewNoOpLogger())()
	reloaded, ok := msg.(datasetReloadedMsg)
	require.True(t, ok, "expected datasetReloadedMsg, got %T", msg)
	require.Same(t, ds, reloaded.Dataset)

	errLoader := fakeLoader{err: errors.New("boom")}
	require.Nil(t, reloadDatasetCmd(context.Background(), errLoader, "/repo", models.Environment{}, logging.NewNoOpLogger())(),
		"a reload error must produce a nil (no-op) message, not a toast")
}
