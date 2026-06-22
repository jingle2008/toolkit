package production

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/infra/loader"
)

// Compile-time guarantee that Client satisfies the optional RepoWatcher
// capability the TUI type-asserts for.
var _ loader.RepoWatcher = Client{}

func TestClient_WatchRepo_TriggersOnChange(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	trig, err := Client{}.WatchRepo(ctx, dir)
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "x.yaml"), []byte("v"), 0o600)) //nolint:gosec // test helper; 0o600 is fine for temp files
	select {
	case <-trig:
	case <-time.After(7 * time.Second): // > DebounceWindow (5s)
		t.Fatal("expected a trigger from WatchRepo after a file change")
	}
}
