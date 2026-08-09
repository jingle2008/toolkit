package production

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/infra/loader"
	"github.com/jingle2008/toolkit/pkg/models"
)

func TestClient_ImplementsWatcher(t *testing.T) {
	t.Parallel()
	assert.Implements(t, (*loader.Watcher)(nil), &Client{})
}

// TestWatch_BadKubeConfigErrors pins the client-construction guard on
// every Watch* method: a kubeconfig that can't be loaded must surface as
// an error rather than a nil channel the TUI would silently select on
// forever.
//
// Both the nonexistent explicit path and the empty Environment (whose
// KubeContext() is a context no real kubeconfig defines) push toward the
// failure, so this stays deterministic on developer machines that do
// have a working ~/.kube/config.
func TestWatch_BadKubeConfigErrors(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		watch func(context.Context, string, models.Environment) (<-chan struct{}, error)
	}{
		{"WatchBaseModels", Client{}.WatchBaseModels},
		{"WatchImportedModels", Client{}.WatchImportedModels},
		{"WatchGPUNodes", Client{}.WatchGPUNodes},
		{"WatchGPUWorkloads", Client{}.WatchGPUWorkloads},
		{"WatchDedicatedAIClusters", Client{}.WatchDedicatedAIClusters},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ch, err := tc.watch(context.Background(), "/nonexistent/kubeconfig", models.Environment{})
			require.Error(t, err, "%s should reject an unloadable kubeconfig", tc.name)
			assert.Nil(t, ch, "%s must not hand back a channel on failure", tc.name)
		})
	}
}
