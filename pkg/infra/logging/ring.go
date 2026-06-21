package logging

import (
	"sync"
	"time"
)

// Level identifies the severity of a captured log entry.
type Level int8

// Severity levels for captured entries, ascending.
const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

// Entry is a single captured log record.
type Entry struct {
	Time    time.Time
	Level   Level
	Message string
	Fields  []any // alternating key, value — as passed to the ...w methods
}

// RingSink is a Logger that retains the most recent entries in a
// fixed-capacity, mutex-guarded ring buffer. It is safe for concurrent
// writes from loader / watch goroutines. Snapshot returns a copy for
// rendering. DebugEnabled is always true so it captures every level
// regardless of the file logger's configured level.
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

func (r *RingSink) record(level Level, msg string, fields []any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.buf[(r.start+r.size)%r.cap] = Entry{
		Time: time.Now(), Level: level, Message: msg, Fields: fields,
	}
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
	for i := 0; i < r.size; i++ {
		out[i] = r.buf[(r.start+i)%r.cap]
	}
	return out
}

// Debugw logs a debug message with key-value fields.
func (r *RingSink) Debugw(msg string, kv ...any) { r.record(LevelDebug, msg, kv) }

// Infow logs an info message with key-value fields.
func (r *RingSink) Infow(msg string, kv ...any) { r.record(LevelInfo, msg, kv) }

// Warnw logs a warning message with key-value fields.
func (r *RingSink) Warnw(msg string, kv ...any) { r.record(LevelWarn, msg, kv) }

// Errorw logs an error message with key-value fields.
func (r *RingSink) Errorw(msg string, kv ...any) { r.record(LevelError, msg, kv) }

// WithFields returns a Logger that records into the same ring with kv
// prepended to every entry's fields.
func (r *RingSink) WithFields(kv ...any) Logger {
	return &ringView{sink: r, fields: append([]any(nil), kv...)}
}

// DebugEnabled reports true: the ring always captures debug entries.
func (r *RingSink) DebugEnabled() bool { return true }

// Sync is a no-op (in-memory).
func (r *RingSink) Sync() error { return nil }

// ringView is a RingSink wrapper that prepends accumulated fields.
type ringView struct {
	sink   *RingSink
	fields []any
}

func (v *ringView) join(kv []any) []any {
	if len(v.fields) == 0 {
		return kv
	}
	out := make([]any, 0, len(v.fields)+len(kv))
	out = append(out, v.fields...)
	out = append(out, kv...)
	return out
}

func (v *ringView) Debugw(msg string, kv ...any) { v.sink.record(LevelDebug, msg, v.join(kv)) }
func (v *ringView) Infow(msg string, kv ...any)  { v.sink.record(LevelInfo, msg, v.join(kv)) }
func (v *ringView) Warnw(msg string, kv ...any)  { v.sink.record(LevelWarn, msg, v.join(kv)) }
func (v *ringView) Errorw(msg string, kv ...any) { v.sink.record(LevelError, msg, v.join(kv)) }
func (v *ringView) WithFields(kv ...any) Logger {
	return &ringView{sink: v.sink, fields: v.join(kv)}
}
func (v *ringView) DebugEnabled() bool { return true }
func (v *ringView) Sync() error        { return nil }
