package logging

import (
	"context"
	"errors"
	"log/slog"
)

// fanout is a slog.Handler that forwards each record to every wrapped
// handler that accepts its level.
type fanout struct {
	handlers []slog.Handler
}

// NewFanout returns a handler that sends every record to each of
// handlers whose Enabled accepts it. It is enabled at a level when any
// leg is, so a leg that captures everything (the ring) does not lower
// what the others write.
func NewFanout(handlers ...slog.Handler) slog.Handler {
	return &fanout{handlers: handlers}
}

func (f *fanout) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range f.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (f *fanout) Handle(ctx context.Context, r slog.Record) error {
	var errs []error
	for _, h := range f.handlers {
		if !h.Enabled(ctx, r.Level) {
			continue
		}
		// Clone: a handler may retain or extend the record's attrs.
		if err := h.Handle(ctx, r.Clone()); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (f *fanout) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := make([]slog.Handler, len(f.handlers))
	for i, h := range f.handlers {
		next[i] = h.WithAttrs(attrs)
	}
	return &fanout{handlers: next}
}

func (f *fanout) WithGroup(name string) slog.Handler {
	if name == "" {
		return f
	}
	next := make([]slog.Handler, len(f.handlers))
	for i, h := range f.handlers {
		next[i] = h.WithGroup(name)
	}
	return &fanout{handlers: next}
}
