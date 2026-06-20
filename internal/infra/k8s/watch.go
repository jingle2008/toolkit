package k8s

import (
	"context"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/watch"
)

// DebounceWindow is the coalescing window for watch triggers: events
// observed within one window collapse to a single reload tick. It is a
// package-level var so tests can shorten it. ~5s matches the TUI's
// "eventual" freshness target without re-listing on every raw event.
var DebounceWindow = 5 * time.Second

// watchTrigger opens every watcher via the given openers and merges
// their events into a single coalesced trigger channel. Each received
// value on the returned channel means "something changed; reload now".
//
// The watch is a TRIGGER, not a data source — event bodies are
// discarded. The caller owns ctx; cancelling it stops all watchers and
// closes the returned channel. The channel also closes if any
// underlying stream dies (the API server closing the connection), which
// the caller treats as a fallback signal.
//
// If any opener returns an error, all already-opened watchers are
// stopped and the error is returned with no channel.
func watchTrigger(
	ctx context.Context,
	window time.Duration,
	openers ...func(context.Context) (watch.Interface, error),
) (<-chan struct{}, error) {
	watchers := make([]watch.Interface, 0, len(openers))
	for _, open := range openers {
		w, err := open(ctx)
		if err != nil {
			for _, prev := range watchers {
				prev.Stop()
			}
			return nil, err
		}
		watchers = append(watchers, w)
	}

	// done is closed when any stream dies; signals fallback to callers.
	done := make(chan struct{})
	var once sync.Once
	closeDone := func() { once.Do(func() { close(done) }) }

	// raw carries one signal per observed event (buffered so a fan-in
	// goroutine never blocks while the coalescer is mid-timer).
	raw := make(chan struct{}, 1)

	var wg sync.WaitGroup
	for _, w := range watchers {
		wg.Add(1)
		go func(w watch.Interface) {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case <-done:
					return
				case _, ok := <-w.ResultChan():
					if !ok {
						closeDone() // stream died
						return
					}
					select {
					case raw <- struct{}{}:
					default: // a signal is already pending; coalesce
					}
				}
			}
		}(w)
	}

	// stopped is closed after all watchers are stopped and fan-in goroutines exit.
	stopped := make(chan struct{})

	// Stop every watcher once ctx is cancelled or a stream dies.
	go func() {
		select {
		case <-ctx.Done():
		case <-done:
		}
		for _, w := range watchers {
			w.Stop()
		}
		wg.Wait()
		close(stopped)
	}()

	out := make(chan struct{})
	go func() {
		// Wait for watchers to be stopped before closing out, so callers that
		// check fw.IsStopped() immediately after receiving the closed channel
		// observe consistent state.
		defer func() {
			<-stopped
			close(out)
		}()
		var timerC <-chan time.Time
		for {
			select {
			case <-ctx.Done():
				return
			case <-done:
				return
			case <-raw:
				if timerC == nil {
					timerC = time.After(window)
				}
			case <-timerC:
				timerC = nil
				select {
				case out <- struct{}{}:
				case <-ctx.Done():
					return
				case <-done:
					return
				}
			}
		}
	}()

	return out, nil
}
