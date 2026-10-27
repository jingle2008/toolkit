package logging

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// deadPID is a pid no live process holds (above every OS's pid_max).
const deadPID = 2147483000

func touch(t *testing.T, path, data string, mod time.Time) {
	t.Helper()
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, mod, mod); err != nil {
		t.Fatal(err)
	}
}

func TestDefaultDir_XDGStateHome(t *testing.T) {
	state := t.TempDir()
	t.Setenv("XDG_STATE_HOME", state)
	if got, want := DefaultDir(), filepath.Join(state, "toolkit", "logs"); got != want {
		t.Errorf("DefaultDir() = %q, want %q", got, want)
	}
	// The spec ignores a relative value.
	t.Setenv("XDG_STATE_HOME", "rel")
	if got := DefaultDir(); !filepath.IsAbs(got) || strings.HasPrefix(got, "rel") {
		t.Errorf("relative XDG_STATE_HOME used: %q", got)
	}
}

func TestListSessions_GroupsFilesPerSession(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	now := time.Now()
	older := SessionID(now.Add(-time.Hour), "get", deadPID)
	newer := SessionID(now, "drain-status", os.Getpid())
	touch(t, filepath.Join(dir, older+".log"), "a", now)
	touch(t, filepath.Join(dir, newer+".log"), "bb", now)
	touch(t, filepath.Join(dir, newer+"-2026-10-26T10-00-00.000.log.gz"), "ccc", now)
	touch(t, filepath.Join(dir, newer+"-2026-10-26T09-00-00.000.log"), "dddd", now)
	touch(t, filepath.Join(dir, newer+".log.stderr"), "e", now)
	touch(t, filepath.Join(dir, "toolkit.log"), "not a session", now)
	touch(t, filepath.Join(dir, "notes.txt"), "x", now)

	got, err := ListSessions(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d sessions, want 2: %+v", len(got), got)
	}
	s := got[0]
	if s.ID != newer || s.Cmd != "drain-status" || s.PID != os.Getpid() || !s.Running {
		t.Errorf("newest session = %+v", s)
	}
	if s.Size != 2+3+4+1 {
		t.Errorf("size = %d, want 10", s.Size)
	}
	wantFiles := []string{
		filepath.Join(dir, newer+"-2026-10-26T09-00-00.000.log"),
		filepath.Join(dir, newer+"-2026-10-26T10-00-00.000.log.gz"),
		filepath.Join(dir, newer+".log"),
		filepath.Join(dir, newer+".log.stderr"),
	}
	if !slices.Equal(s.Files(), wantFiles) {
		t.Errorf("Files() = %v, want %v", s.Files(), wantFiles)
	}
	if o := got[1]; o.ID != older || o.Running || o.Stderr != "" || len(o.Backups) != 0 {
		t.Errorf("older session = %+v", o)
	}
}

func TestListSessions_MissingDir(t *testing.T) {
	t.Parallel()
	got, err := ListSessions(filepath.Join(t.TempDir(), "absent"))
	if err != nil || got != nil {
		t.Errorf("ListSessions(missing) = %v, %v; want nil, nil", got, err)
	}
}

func TestPruneSessions(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	now := time.Now()
	start := now.Add(-time.Hour)
	// MaxSessions+2 recent exited sessions: the two oldest go.
	ids := make([]string, 0, MaxSessions+2)
	for i := range MaxSessions + 2 {
		id := SessionID(start.Add(time.Duration(i)*time.Second), "get", deadPID)
		ids = append(ids, id)
		touch(t, filepath.Join(dir, id+".log"), "x", now)
	}
	// A stale exited session goes; a stale running one stays.
	stale := SessionID(now.Add(-30*24*time.Hour), "tui", deadPID)
	touch(t, filepath.Join(dir, stale+".log"), "x", now.Add(-MaxAge-time.Hour))
	touch(t, filepath.Join(dir, stale+".log.stderr"), "x", now.Add(-MaxAge-time.Hour))
	running := SessionID(now.Add(-30*24*time.Hour), "mcp", os.Getpid())
	touch(t, filepath.Join(dir, running+".log"), "x", now.Add(-MaxAge-time.Hour))

	removed, err := PruneSessions(dir, now)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 3 {
		t.Errorf("removed %d, want 3", removed)
	}
	for _, gone := range []string{ids[0] + ".log", ids[1] + ".log", stale + ".log", stale + ".log.stderr"} {
		if _, err := os.Stat(filepath.Join(dir, gone)); !os.IsNotExist(err) {
			t.Errorf("%s survived the prune", gone)
		}
	}
	for _, kept := range []string{ids[2] + ".log", ids[len(ids)-1] + ".log", running + ".log"} {
		if _, err := os.Stat(filepath.Join(dir, kept)); err != nil {
			t.Errorf("%s pruned: %v", kept, err)
		}
	}
}

func TestNewSession_WritesPerProcessFile(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "logs")
	l, closer, path, err := NewSession(dir, "get", FormatConsole, "", false)
	if err != nil {
		t.Fatal(err)
	}
	l.Info("probe")
	if err := closer.Close(); err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(path) != dir {
		t.Errorf("path %q not in %q", path, dir)
	}
	sessions, err := ListSessions(dir)
	if err != nil || len(sessions) != 1 {
		t.Fatalf("ListSessions = %v, %v", sessions, err)
	}
	if s := sessions[0]; s.Path != path || s.Cmd != "get" || s.PID != os.Getpid() || !s.Running {
		t.Errorf("session = %+v, path %q", s, path)
	}
	if fi, err := os.Stat(dir); err != nil || fi.Mode().Perm() != 0o700 {
		t.Errorf("log dir mode: %v, %v", fi, err)
	}
}
