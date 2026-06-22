package tui

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/pkg/models"
)

// watchableLoader is a fakeLoader that also implements loader.Watcher.
type watchableLoader struct {
	fakeLoader // existing minimal Composite stub used in TUI tests
	trigger    chan struct{}
	err        error
}

func (w *watchableLoader) WatchBaseModels(_ context.Context, _ string, _ models.Environment) (<-chan struct{}, error) {
	if w.err != nil {
		return nil, w.err
	}
	return w.trigger, nil
}

func (w *watchableLoader) WatchImportedModels(_ context.Context, _ string, _ models.Environment) (<-chan struct{}, error) {
	if w.err != nil {
		return nil, w.err
	}
	return w.trigger, nil
}

func (w *watchableLoader) WatchGPUNodes(_ context.Context, _ string, _ models.Environment) (<-chan struct{}, error) {
	if w.err != nil {
		return nil, w.err
	}
	return w.trigger, nil
}

func (w *watchableLoader) WatchGPUWorkloads(_ context.Context, _ string, _ models.Environment) (<-chan struct{}, error) {
	if w.err != nil {
		return nil, w.err
	}
	return w.trigger, nil
}

func (w *watchableLoader) WatchDedicatedAIClusters(_ context.Context, _ string, _ models.Environment) (<-chan struct{}, error) {
	if w.err != nil {
		return nil, w.err
	}
	return w.trigger, nil
}

func TestStartWatchCmd_SuccessEmitsStarted(t *testing.T) {
	t.Parallel()
	trig := make(chan struct{}, 1)
	ld := &watchableLoader{trigger: trig}
	cmd := startK8sWatchCmd(context.Background(), ld, domain.GPUNode, "kc", models.Environment{}, 7)
	require.NotNil(t, cmd)

	msg := cmd()
	started, ok := msg.(k8sWatchStartedMsg)
	require.True(t, ok, "expected k8sWatchStartedMsg, got %T", msg)
	assert.Equal(t, domain.GPUNode, started.Cat)
	assert.Equal(t, 7, started.Gen)
	assert.NotNil(t, started.Trigger)
}

func TestStartWatchCmd_UnsupportedEmitsUnavailable(t *testing.T) {
	t.Parallel()
	// fakeLoader implements Composite but NOT loader.Watcher.
	ld := fakeLoader{}
	cmd := startK8sWatchCmd(context.Background(), ld, domain.GPUNode, "kc", models.Environment{}, 3)
	msg := cmd()
	unavail, ok := msg.(k8sWatchUnavailableMsg)
	require.True(t, ok, "expected k8sWatchUnavailableMsg, got %T", msg)
	assert.Equal(t, 3, unavail.Gen)
	assert.Equal(t, domain.GPUNode, unavail.Cat)
}

func TestWaitForTriggerCmd_TickEmitsTriggered(t *testing.T) {
	t.Parallel()
	trig := make(chan struct{}, 1)
	trig <- struct{}{}
	cmd := waitForK8sTriggerCmd(domain.GPUNode, trig, 5)
	msg := cmd()
	triggered, ok := msg.(k8sWatchTriggeredMsg)
	require.True(t, ok, "expected k8sWatchTriggeredMsg, got %T", msg)
	assert.Equal(t, 5, triggered.Gen)
}

func TestWaitForTriggerCmd_ClosedEmitsClosed(t *testing.T) {
	t.Parallel()
	trig := make(chan struct{})
	close(trig)
	cmd := waitForK8sTriggerCmd(domain.GPUNode, trig, 9)
	msg := cmd()
	closed, ok := msg.(k8sWatchClosedMsg)
	require.True(t, ok, "expected k8sWatchClosedMsg, got %T", msg)
	assert.Equal(t, domain.GPUNode, closed.Cat)
	assert.Equal(t, 9, closed.Gen)
}
