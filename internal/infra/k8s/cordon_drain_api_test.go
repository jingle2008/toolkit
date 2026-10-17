package k8s

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/jingle2008/toolkit/pkg/infra/logging"
)

// SetCordon's inner, through kubectl's real drain helper against a fake
// API: cordon, then uncordon, each landing on the node.
func TestSetCordon_API(t *testing.T) { //nolint:paralleltest // paralleltest is not supported in this package
	ctx := context.Background()
	client := fake.NewSimpleClientset()
	node := makeNode("n1", map[string]string{}, 0, false, nil)
	_ = client.Tracker().Add(node)
	for _, want := range []bool{true, false} {
		changed, err := setCordon(ctx, client, "n1", want)
		require.NoError(t, err)
		assert.True(t, changed)
		got, err := client.CoreV1().Nodes().Get(ctx, "n1", metav1.GetOptions{})
		require.NoError(t, err)
		assert.Equal(t, want, got.Spec.Unschedulable)
	}
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

func (m *mockLogger) Warnw(_ string, _ ...any) {}

func (m *mockLogger) Errorw(_ string, _ ...any) {}

func (m *mockLogger) Sync() error { return nil }

func (m *mockLogger) WithFields(_ ...any) logging.Logger { return m }
