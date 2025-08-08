package k8s

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes/fake"
	drainpkg "k8s.io/kubectl/pkg/drain"

	"github.com/jingle2008/toolkit/pkg/infra/logging"
)

func TestToggleCordon_API(t *testing.T) { //nolint:paralleltest // paralleltest is not supported in this package
	ctx := context.Background()
	client := fake.NewSimpleClientset()
	// Add a node to the fake client
	node := makeNode("n1", map[string]string{}, 0, false, nil)
	_ = client.Tracker().Add(node)
	// Should not error
	state, err := toggleCordon(ctx, client, "n1")
	require.NoError(t, err)
	assert.True(t, state)
	state, err = toggleCordon(ctx, client, "n1")
	require.NoError(t, err)
	assert.False(t, state)
}

func TestLogWriter_Write(t *testing.T) {
	t.Parallel()
	called := false
	logger := &mockLogger{onInfo: func(_ string, _ ...any) {
		called = true
	}}
	w := logWriter{logger: logger}
	n, err := w.Write([]byte("test message"))
	require.NoError(t, err)
	assert.Equal(t, len("test message"), n)
	assert.True(t, called)
}

type mockLogger struct {
	onInfo func(msg string, kv ...any)
}

func (m *mockLogger) Infow(msg string, kv ...any) {
	if m.onInfo != nil {
		m.onInfo(msg, kv...)
	}
}

func (m *mockLogger) DebugEnabled() bool {
	return false
}

func (m *mockLogger) Debugw(_ string, _ ...any) {}

func (m *mockLogger) Errorw(_ string, _ ...any) {}

func (m *mockLogger) Sync() error { return nil }

func (m *mockLogger) WithFields(_ ...any) logging.Logger { return m }

func TestDrainNode_API(t *testing.T) { //nolint:paralleltest // paralleltest is not supported in this package
	ctx := context.Background()
	client := fake.NewSimpleClientset()
	node := makeNode("n2", map[string]string{}, 0, false, nil)
	_ = client.Tracker().Add(node)
	// Patch runNodeDrain to simulate success
	orig := runNodeDrain
	defer func() { runNodeDrain = orig }()
	runNodeDrain = func(_ *drainpkg.Helper, _ string) error {
		return nil
	}
	err := drainNode(ctx, client, "n2")
	require.NoError(t, err)
}
