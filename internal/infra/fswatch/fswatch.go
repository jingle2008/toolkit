// Package fswatch provides a recursive, debounced filesystem watcher that
// emits a coalesced "something changed" trigger — the same channel shape the
// k8s watch feeds into the TUI reducer. It is used to make repo-backed
// categories live by reloading the dataset when the working tree changes.
package fswatch

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/jingle2008/toolkit/pkg/infra/logging"
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
func run(ctx context.Context, w *fsnotify.Watcher, root string, window time.Duration, out chan<- struct{}) {
	defer close(out)
	defer func() { _ = w.Close() }()

	logging.FromContext(ctx).Infow("fs watch established", "root", root)

	var timerC <-chan time.Time
	for {
		select {
		case <-ctx.Done():
			logging.FromContext(ctx).Debugw("fs watch stopped: context canceled")
			return
		case ev, ok := <-w.Events:
			if !ok {
				return
			}
			if isHidden(ev.Name) {
				continue // ignore events under dot-dirs (e.g. .git churn)
			}
			// A newly created directory must be watched so its contents
			// trigger too.
			if ev.Op.Has(fsnotify.Create) {
				if info, statErr := os.Stat(ev.Name); statErr == nil && info.IsDir() {
					_ = addRecursive(w, ev.Name)
				}
			}
			if timerC == nil {
				timerC = time.After(window)
			}
		case _, ok := <-w.Errors:
			if !ok {
				return
			}
			logging.FromContext(ctx).Warnw("fs watch error; live repo watch will drop")
			return
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
