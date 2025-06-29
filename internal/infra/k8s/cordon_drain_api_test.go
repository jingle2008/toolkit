package k8s

import (
	"context"
	"testing"

	"github.com/jingle2008/toolkit/pkg/infra/logging"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes/fake"
	drainpkg "k8s.io/kubectl/pkg/drain"
)

func TestToggleCordon_API(t *testing.T) { //nolint:paralleltest // paralleltest is not supported in this package
	ctx := context.Background()
	client := fake.NewSimpleClientset()
	// Add a node to the fake client
	node := makeNode("n1", map[string]string{}, 0, false, nil)
	_ = client.Tracker().Add(node)
	// Should not error
	err := toggleCordon(ctx, client, "n1")
	assert.NoError(t, err)
}

func TestLogWriter_Write(t *testing.T) {
	t.Parallel()
	called := false
	logger := &mockLogger{onInfo: func(_ string, _ ...interface{}) {
		called = true
	}}
	w := logWriter{logger: logger}
	n, err := w.Write([]byte("test message"))
	assert.NoError(t, err)
	assert.Equal(t, len("test message"), n)
	assert.True(t, called)
}

type mockLogger struct {
	onInfo func(msg string, kv ...interface{})
}

func (m *mockLogger) Infow(msg string, kv ...interface{}) {
	if m.onInfo != nil {
		m.onInfo(msg, kv...)
	}
}

func (m *mockLogger) DebugEnabled() bool {
	return false
}

func (m *mockLogger) Debugw(_ string, _ ...interface{}) {}

func (m *mockLogger) Errorw(_ string, _ ...interface{}) {}

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
	assert.NoError(t, err)
}
