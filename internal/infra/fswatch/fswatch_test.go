package fswatch

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestWatch_TriggersOnFileChange(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	trig, err := Watch(ctx, dir, 50*time.Millisecond)
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.yaml"), []byte("x"), 0o600)) //nolint:gosec // test helper; 0o600 is fine for temp files

	select {
	case <-trig:
	case <-time.After(2 * time.Second):
		t.Fatal("expected a trigger after writing a file")
	}
}

func TestWatch_RecursiveNewSubdir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	trig, err := Watch(ctx, dir, 50*time.Millisecond)
	require.NoError(t, err)

	sub := filepath.Join(dir, "sub")
	require.NoError(t, os.Mkdir(sub, 0o750)) //nolint:gosec // test helper; 0o750 is fine for temp dirs
	<-trig                                   // drain the trigger caused by creating the directory

	require.NoError(t, os.WriteFile(filepath.Join(sub, "b.yaml"), []byte("y"), 0o600)) //nolint:gosec // test helper; 0o600 is fine for temp files
	select {
	case <-trig:
	case <-time.After(2 * time.Second):
		t.Fatal("expected trigger from a file in a newly created subdir")
	}
}

func TestWatch_IgnoresDotGit(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o750)) //nolint:gosec // test helper; 0o750 is fine for temp dirs
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	trig, err := Watch(ctx, dir, 50*time.Millisecond)
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(filepath.Join(dir, ".git", "HEAD"), []byte("ref"), 0o600)) //nolint:gosec // test helper; 0o600 is fine for temp files
	select {
	case <-trig:
		t.Fatal("changes under .git must not trigger a reload")
	case <-time.After(400 * time.Millisecond):
		// good: no trigger
	}
}

func TestWatch_CancelClosesChannel(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())

	trig, err := Watch(ctx, dir, 50*time.Millisecond)
	require.NoError(t, err)

	cancel()
	select {
	case _, ok := <-trig:
		require.False(t, ok, "channel should be closed after ctx cancel")
	case <-time.After(2 * time.Second):
		t.Fatal("channel was not closed after ctx cancel")
	}
}
