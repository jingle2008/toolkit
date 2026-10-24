package k8s

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	logging "github.com/jingle2008/toolkit/internal/logging"
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
	ring := logging.NewRingSink(4)
	w := logWriter{logger: slog.New(ring)}
	n, err := w.Write([]byte("test message"))
	require.NoError(t, err)
	assert.Equal(t, len("test message"), n)
	snap := ring.Snapshot()
	require.Len(t, snap, 1)
	assert.Equal(t, logging.LevelInfo, snap[0].Level)
}
