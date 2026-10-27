// Package fswatch provides a recursive, debounced filesystem watcher that
// emits a coalesced "something changed" trigger — the same channel shape the
// k8s watch feeds into the TUI reducer. It is used to make repo-backed
// categories live by reloading the dataset when the working tree changes.
package fswatch

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/jingle2008/toolkit/internal/logging"
)

// Watch establishes a recursive filesystem watch rooted at root and returns a
// coalesced trigger channel: one value per debounce window in which any
// non-hidden file under root changed. Dot-directories (.git, .idea, …) are
// excluded. The caller owns ctx; cancelling it stops the watcher and closes
// the channel. The channel also closes if the watcher backend dies, which the
// caller treats as a fallback signal.
func Watch(ctx context.Context, root string, window time.Duration) (<-chan struct{}, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	if err := addRecursive(w, root); err != nil {
		_ = w.Close()
		return nil, err
	}

	out := make(chan struct{})
	go run(ctx, w, root, window, out)
	return out, nil
}

// isHidden reports whether path's base name marks a dot-entry (.git, .idea),
// excluding the relative "." and ".." entries.
func isHidden(path string) bool {
	base := filepath.Base(path)
	return base != "." && base != ".." && strings.HasPrefix(base, ".")
}

// addRecursive adds root and every non-hidden subdirectory to w.
func addRecursive(w *fsnotify.Watcher, root string) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if path != root && isHidden(path) {
			return fs.SkipDir
		}
		return w.Add(path)
	})
}

// run consumes fsnotify events, adds newly created directories on the fly,
// ignores hidden paths, debounces into out, and tears everything down on ctx
// cancel or backend error.
func run(ctx context.Context, w *fsnotify.Watcher, root string, window time.Duration, out chan<- struct{}) { //nolint:cyclop // event-select loop; its arms are the debounce/watch lifecycle
	defer close(out)
	defer func() { _ = w.Close() }()

	logging.FromContext(ctx).Info("fs watch established", "root", root)

	var timerC <-chan time.Time
	for {
		select {
		case <-ctx.Done():
			logging.FromContext(ctx).Debug("fs watch stopped: context canceled")
			return
		case ev, ok := <-w.Events:
			if !ok {
				return
			}
			if !watchEvent(ctx, w, ev) {
				continue
			}
			if timerC == nil {
				timerC = time.After(window)
			}
		case err, ok := <-w.Errors:
			if !ok || !survivable(ctx, err) {
				return
			}
			if timerC == nil {
				timerC = time.After(window)
			}
		case <-timerC:
			timerC = nil
			select {
			case out <- struct{}{}:
			case <-ctx.Done():
				return
			}
		}
	}
}

// watchEvent reports whether ev should trigger a reload: not under a
// dot-dir (e.g. .git churn). A newly created directory is watched too, so
// its contents trigger.
func watchEvent(ctx context.Context, w *fsnotify.Watcher, ev fsnotify.Event) bool {
	if isHidden(ev.Name) {
		return false
	}
	if ev.Op.Has(fsnotify.Create) {
		if info, statErr := os.Stat(ev.Name); statErr == nil && info.IsDir() {
			if err := addRecursive(w, ev.Name); err != nil {
				// Its changes won't trigger until the watch restarts.
				logging.FromContext(ctx).Warn("fs watch: can't watch new directory", "dir", ev.Name, "error", err)
			}
		}
	}
	return true
}

// survivable reports whether the watch carries on after err, reloading
// in its place: an overflow only lost events, so the reload they would
// have caused still comes. Any other error ends the watch.
func survivable(ctx context.Context, err error) bool {
	if errors.Is(err, fsnotify.ErrEventOverflow) {
		logging.FromContext(ctx).Warn("fs watch: events overflowed; reloading", "error", err)
		return true
	}
	logging.FromContext(ctx).Warn("fs watch error; live repo watch will drop", "error", err)
	return false
}
