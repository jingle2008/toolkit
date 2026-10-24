package logging

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Level identifies the severity of a captured log entry. It is the TUI
// log overlay's own scale (consecutive, so the overlay can cycle it);
// slog levels map onto it by range.
type Level int8

// Severity levels for captured entries, ascending.
const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

// levelOf maps a slog level onto the ring's scale.
func levelOf(l slog.Level) Level {
	switch {
	case l < slog.LevelInfo:
		return LevelDebug
	case l < slog.LevelWarn:
		return LevelInfo
	case l < slog.LevelError:
		return LevelWarn
	default:
		return LevelError
	}
}

// Entry is a single captured log record.
type Entry struct {
	Time    time.Time
	Level   Level
	Message string
	Fields  []any // alternating key, value; group members keyed "group.key"
}

// RingSink is a slog.Handler that retains the most recent records in a
// fixed-capacity, mutex-guarded ring buffer. It is safe for concurrent
// writes from loader / watch goroutines. Snapshot returns a copy for
// rendering. It is enabled at every level, so it captures debug records
// regardless of the file handler's configured level.
type RingSink struct {
	mu    sync.Mutex
	buf   []Entry
	cap   int
	start int
	size  int
}

// NewRingSink returns a RingSink retaining the last capacity entries.
func NewRingSink(capacity int) *RingSink {
	if capacity < 1 {
		capacity = 1
	}
	return &RingSink{buf: make([]Entry, capacity), cap: capacity}
}

func (r *RingSink) record(e Entry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.buf[(r.start+r.size)%r.cap] = e
	if r.size < r.cap {
		r.size++
	} else {
		r.start = (r.start + 1) % r.cap
	}
}

// Snapshot returns the retained entries oldest-first as a fresh slice.
func (r *RingSink) Snapshot() []Entry {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Entry, r.size)
	for i := range r.size {
		out[i] = r.buf[(r.start+i)%r.cap]
	}
	return out
}

// Enabled reports true: the ring captures every level.
func (r *RingSink) Enabled(context.Context, slog.Level) bool { return true }

// Handle records rec in the ring.
func (r *RingSink) Handle(ctx context.Context, rec slog.Record) error {
	return ringView{sink: r}.Handle(ctx, rec)
}

// WithAttrs returns a handler recording into the same ring with attrs
// prepended to every entry's fields.
func (r *RingSink) WithAttrs(attrs []slog.Attr) slog.Handler {
	return ringView{sink: r}.WithAttrs(attrs)
}

// WithGroup returns a handler recording into the same ring with later
// keys qualified by name.
func (r *RingSink) WithGroup(name string) slog.Handler {
	return ringView{sink: r}.WithGroup(name)
}

// ringView is a RingSink handler carrying accumulated attrs and the
// open group prefix.
type ringView struct {
	sink   *RingSink
	prefix string // "a.b." for WithGroup("a").WithGroup("b")
	fields []any
}

func (v ringView) Enabled(context.Context, slog.Level) bool { return true }

func (v ringView) Handle(_ context.Context, rec slog.Record) error {
	fields := make([]any, 0, len(v.fields)+2*rec.NumAttrs())
	fields = append(fields, v.fields...)
	rec.Attrs(func(a slog.Attr) bool {
		fields = appendAttr(fields, v.prefix, a)
		return true
	})
	t := rec.Time
	if t.IsZero() {
		t = time.Now()
	}
	v.sink.record(Entry{Time: t, Level: levelOf(rec.Level), Message: rec.Message, Fields: fields})
	return nil
}

func (v ringView) WithAttrs(attrs []slog.Attr) slog.Handler {
	fields := append([]any(nil), v.fields...)
	for _, a := range attrs {
		fields = appendAttr(fields, v.prefix, a)
	}
	return ringView{sink: v.sink, prefix: v.prefix, fields: fields}
}

func (v ringView) WithGroup(name string) slog.Handler {
	if name == "" {
		return v
	}
	return ringView{sink: v.sink, prefix: v.prefix + name + ".", fields: v.fields}
}

// appendAttr flattens a onto fields as key, value pairs, following the
// slog.Handler rules: values are resolved, empty attrs are dropped, and
// groups are inlined with their key as a prefix (or without one when
// the key is empty).
func appendAttr(fields []any, prefix string, a slog.Attr) []any {
	a.Value = a.Value.Resolve()
	if a.Equal(slog.Attr{}) {
		return fields
	}
	if a.Value.Kind() == slog.KindGroup {
		if a.Key != "" {
			prefix += a.Key + "."
		}
		for _, g := range a.Value.Group() {
			fields = appendAttr(fields, prefix, g)
		}
		return fields
	}
	return append(fields, prefix+a.Key, a.Value.Any())
}
