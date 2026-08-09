package fswatch

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/stretchr/testify/assert"
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

func TestWatch_NonexistentRootErrors(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// addRecursive's WalkDir hands the stat error straight back, so Watch
	// fails instead of returning a channel that never fires.
	trig, err := Watch(ctx, filepath.Join(t.TempDir(), "no-such-dir"), 50*time.Millisecond)
	require.Error(t, err)
	require.Nil(t, trig)
}

func TestIsHidden(t *testing.T) {
	t.Parallel()
	assert.True(t, isHidden("/repo/.git"))
	assert.True(t, isHidden(".idea"))
	assert.False(t, isHidden("/repo/config"))
	// "." and ".." are relative-path entries, not hidden dirs — treating
	// them as hidden would make a relative root skip itself.
	assert.False(t, isHidden("."))
	assert.False(t, isHidden(".."))
}

func TestRun_ClosedWatcherClosesOutput(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	w, err := fsnotify.NewWatcher()
	require.NoError(t, err)
	require.NoError(t, w.Add(dir))

	out := make(chan struct{})
	go run(context.Background(), w, dir, 50*time.Millisecond, out)

	// Closing the backend closes both Events and Errors; run treats
	// either as "watcher died" and shuts down, which the caller sees as a
	// closed trigger channel. (run's own deferred Close is a no-op here —
	// fsnotify's Close is idempotent.)
	require.NoError(t, w.Close())

	select {
	case _, ok := <-out:
		require.False(t, ok, "trigger channel should close when the backend dies")
	case <-time.After(2 * time.Second):
		t.Fatal("trigger channel was not closed after the watcher closed")
	}
}

func TestRun_CancelWhileTriggerPending(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	w, err := fsnotify.NewWatcher()
	require.NoError(t, err)
	require.NoError(t, w.Add(dir))

	ctx, cancel := context.WithCancel(context.Background())
	out := make(chan struct{})
	go run(ctx, w, dir, 20*time.Millisecond, out)

	// Write a file but never read `out`, so once the debounce window
	// elapses run is parked on the send.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.yaml"), []byte("x"), 0o600)) //nolint:gosec // test helper; 0o600 is fine for temp files
	time.Sleep(200 * time.Millisecond)

	// Cancelling must unblock the parked send rather than leak the
	// goroutine. Both select arms are ready, so drain until closed
	// instead of asserting which one wins.
	cancel()
	deadline := time.After(2 * time.Second)
	for {
		select {
		case _, ok := <-out:
			if !ok {
				return
			}
		case <-deadline:
			t.Fatal("trigger channel was not closed after ctx cancel")
		}
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
