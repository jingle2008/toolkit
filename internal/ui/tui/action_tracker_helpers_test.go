package tui

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

// actionResult runs a tracked action's command — a batch of the action,
// its progress listener and maybe a spinner tick, run concurrently since
// the listener waits for the action — and returns the action's result
// message.
func actionResult(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()
	msgs := make(chan tea.Msg, 16)
	var run func(tea.Cmd)
	run = func(c tea.Cmd) {
		if c == nil {
			return
		}
		go func() {
			msg := c()
			if batch, ok := msg.(tea.BatchMsg); ok {
				for _, sub := range batch {
					run(sub)
				}
				return
			}
			msgs <- msg
		}()
	}
	run(cmd)
	timeout := time.After(2 * time.Second)
	for {
		select {
		case msg := <-msgs:
			if done, ok := msg.(actionDoneMsg); ok {
				return done.msg
			}
		case <-timeout:
			t.Fatal("the action produced no result")
			return nil
		}
	}
}
