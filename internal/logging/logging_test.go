package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestParseLevel(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in    string
		debug bool
		want  slog.Level
	}{
		{"debug", false, slog.LevelDebug},
		{"", true, slog.LevelDebug},
		{"", false, slog.LevelInfo},
		{"info", true, slog.LevelInfo},
		{" WARN ", false, slog.LevelWarn},
		{"warning", false, slog.LevelWarn},
		{"error", true, slog.LevelError},
		{"bogus", true, slog.LevelInfo},
	}
	for _, c := range cases {
		if got := ParseLevel(c.in, c.debug); got != c.want {
			t.Errorf("ParseLevel(%q, %v) = %v, want %v", c.in, c.debug, got, c.want)
		}
	}
}

func TestNewHandler_FormatsAndSource(t *testing.T) {
	t.Parallel()
	var text, js bytes.Buffer
	slog.New(NewHandler(&text, FormatConsole, slog.LevelInfo)).Info("hello", "k", "v")
	slog.New(NewHandler(&js, FormatJSON, slog.LevelInfo)).Info("hello", "k", "v")

	// The source is the short dir/file.go:line form, not an absolute path.
	if got := text.String(); !strings.Contains(got, "msg=hello k=v") ||
		!strings.Contains(got, "source=logging/logging_test.go:") {
		t.Errorf("text line = %q, want msg, kv and the test's short source location", got)
	}
	var rec map[string]any
	if err := json.Unmarshal(js.Bytes(), &rec); err != nil {
		t.Fatalf("json line %q: %v", js.String(), err)
	}
	src, _ := rec["source"].(string)
	if rec["msg"] != "hello" || rec["k"] != "v" || !strings.HasPrefix(src, "logging/logging_test.go:") {
		t.Errorf("json record = %v", rec)
	}
}

func TestNewHandler_Level(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	l := slog.New(NewHandler(&buf, FormatJSON, slog.LevelWarn))
	l.Info("dropped")
	if buf.Len() != 0 {
		t.Errorf("info written at warn level: %q", buf.String())
	}
	if l.Enabled(context.Background(), slog.LevelDebug) {
		t.Error("debug enabled at warn level")
	}
}

func TestNewFile_WritesAndCloses(t *testing.T) {
	t.Parallel()
	for _, format := range []string{FormatConsole, FormatJSON, FormatSlog} {
		path := filepath.Join(t.TempDir(), "toolkit.log")
		l, closer := NewFile(path, format, "", true)
		l.Debug("probe", "n", 1)
		if err := closer.Close(); err != nil {
			t.Fatalf("%s: close: %v", format, err)
		}
		data, err := os.ReadFile(path) //nolint:gosec // G304: test temp path
		if err != nil {
			t.Fatalf("%s: read: %v", format, err)
		}
		got := string(data)
		if !strings.Contains(got, "probe") {
			t.Errorf("%s: debug line missing: %q", format, got)
		}
		if deprecated := strings.Contains(got, "deprecated"); deprecated != (format == FormatSlog) {
			t.Errorf("%s: deprecation warning present = %v: %q", format, deprecated, got)
		}
		if format != FormatConsole && !strings.HasPrefix(got, "{") {
			t.Errorf("%s: want JSON lines, got %q", format, got)
		}
	}
}

func TestContext(t *testing.T) {
	t.Parallel()
	l := slog.New(NewRingSink(1))
	if got := FromContext(WithContext(context.Background(), l)); got != l {
		t.Error("FromContext did not return the attached logger")
	}
	//nolint:staticcheck // SA1012: nil context is the case under test
	for _, ctx := range []context.Context{nil, context.Background(), WithContext(context.Background(), nil)} {
		got := FromContext(ctx)
		if got == nil || got.Enabled(context.Background(), slog.LevelError) {
			t.Errorf("FromContext(%v) = %v, want a discard logger", ctx, got)
		}
	}
}

func TestRingSink_CapturesAllLevels(t *testing.T) {
	t.Parallel()
	r := NewRingSink(10)
	l := slog.New(r)
	l.Debug("d")
	l.Info("i", "k", "v")
	l.Warn("w")
	l.Error("e")
	l.Log(context.Background(), slog.LevelError+4, "beyond")
	snap := r.Snapshot()
	want := []Level{LevelDebug, LevelInfo, LevelWarn, LevelError, LevelError}
	if len(snap) != len(want) {
		t.Fatalf("got %d entries, want %d", len(snap), len(want))
	}
	for i, e := range snap {
		if e.Level != want[i] {
			t.Errorf("entry %d level = %v, want %v", i, e.Level, want[i])
		}
		if e.Time.IsZero() {
			t.Errorf("entry %d has no time", i)
		}
	}
	if snap[1].Message != "i" || len(snap[1].Fields) != 2 || snap[1].Fields[0] != "k" || snap[1].Fields[1] != "v" {
		t.Errorf("entry 1 = %+v", snap[1])
	}
}

func TestRingSink_ZeroTimeRecord(t *testing.T) {
	t.Parallel()
	r := NewRingSink(1)
	_ = r.Handle(context.Background(), slog.Record{Message: "m"})
	if r.Snapshot()[0].Time.IsZero() {
		t.Error("zero record time not replaced")
	}
}

func TestRingSink_WrapsAndCopies(t *testing.T) {
	t.Parallel()
	r := NewRingSink(0) // clamped to 1
	l := slog.New(r)
	l.Info("a")
	l.Info("b")
	snap := r.Snapshot()
	if len(snap) != 1 || snap[0].Message != "b" {
		t.Fatalf("snapshot = %+v, want only b", snap)
	}
	snap[0].Message = "mutated"
	if r.Snapshot()[0].Message != "b" {
		t.Error("snapshot mutation leaked into ring")
	}
}

func TestRingSink_AttrsAndGroups(t *testing.T) {
	t.Parallel()
	r := NewRingSink(4)
	l := slog.New(r).With("req", "1").WithGroup("g").With("a", 1).WithGroup("")
	l.Info("hi", "k", "v", slog.Group("sub", "x", 2), slog.Group("", "inl", 3), slog.Attr{})
	got := r.Snapshot()[0].Fields
	want := []any{"req", "1", "g.a", int64(1), "g.k", "v", "g.sub.x", int64(2), "g.inl", int64(3)}
	if len(got) != len(want) {
		t.Fatalf("fields = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("field %d = %v, want %v", i, got[i], want[i])
		}
	}
	// Siblings derived from the same parent do not share field storage.
	base := slog.New(r).With("p", 0)
	base.With("x", 1).Info("one")
	base.With("y", 2).Info("two")
	if f := r.Snapshot()[2].Fields; f[2] != "y" {
		t.Errorf("sibling fields = %v", f)
	}
}

func TestRingSink_ConcurrentWrites(t *testing.T) {
	t.Parallel()
	r := NewRingSink(50)
	l := slog.New(r)
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 100 {
				l.Info("x")
			}
		})
	}
	wg.Wait()
	if n := len(r.Snapshot()); n != 50 {
		t.Errorf("got %d entries, want 50", n)
	}
}

type errHandler struct {
	slog.Handler
	err error
}

func (h errHandler) Handle(context.Context, slog.Record) error { return h.err }

func TestFanout(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	file := NewHandler(&buf, FormatJSON, slog.LevelInfo)
	r := NewRingSink(8)
	l := slog.New(NewFanout(file, r)).With("cmd", "tui").WithGroup("")

	if !l.Enabled(context.Background(), slog.LevelDebug) {
		t.Error("fanout should be enabled when any leg is")
	}
	l.Debug("ring only")
	l.WithGroup("g").Info("both", "k", "v")

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 1 || !strings.Contains(lines[0], `"cmd":"tui"`) || !strings.Contains(lines[0], `"g":{"k":"v"}`) {
		t.Errorf("file leg = %q", buf.String())
	}
	snap := r.Snapshot()
	if len(snap) != 2 || snap[0].Message != "ring only" || snap[1].Fields[2] != "g.k" {
		t.Errorf("ring leg = %+v", snap)
	}

	quiet := slog.New(NewFanout(NewHandler(&buf, FormatJSON, slog.LevelError)))
	if quiet.Enabled(context.Background(), slog.LevelInfo) {
		t.Error("fanout enabled though no leg is")
	}

	boom := errors.New("boom")
	h := NewFanout(errHandler{Handler: NewRingSink(1), err: boom}, NewRingSink(1))
	if err := h.Handle(context.Background(), slog.Record{}); !errors.Is(err, boom) {
		t.Errorf("Handle error = %v, want boom", err)
	}
}

func TestDiscard(t *testing.T) {
	t.Parallel()
	Discard().Error("nothing") // must not panic
}
