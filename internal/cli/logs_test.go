//nolint:paralleltest // cobra/viper global state; followFile tests swap package seams
package cli

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/spf13/viper"

	"github.com/jingle2008/toolkit/internal/config"
	"github.com/jingle2008/toolkit/internal/logging"
)

const deadPID = 2147483000

func writeFile(t *testing.T, path, data string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeGzip(t *testing.T, path, data string) {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	_, _ = gz.Write([]byte(data))
	_ = gz.Close()
	writeFile(t, path, buf.String())
}

// seedSessions writes an exited get session (backups + current) and a
// newer running tui session with a stderr capture, returning their IDs.
func seedSessions(t *testing.T, dir string) (getID, tuiID string) {
	t.Helper()
	now := time.Now()
	getID = logging.SessionID(now.Add(-time.Hour), "get", deadPID)
	tuiID = logging.SessionID(now, "tui", os.Getpid())
	writeFile(t, filepath.Join(dir, getID+"-2026-10-26T09-00-00.000.log"), "one\n")
	writeGzip(t, filepath.Join(dir, getID+"-2026-10-26T10-00-00.000.log.gz"), "two\n")
	writeFile(t, filepath.Join(dir, getID+".log"), "three\n")
	writeFile(t, filepath.Join(dir, tuiID+".log"), "tui log\n")
	writeFile(t, filepath.Join(dir, tuiID+".log.stderr"), "plugin noise\n")
	return getID, tuiID
}

func TestRunLogs(t *testing.T) {
	dir := t.TempDir()
	getID, tuiID := seedSessions(t, dir)
	cfg := config.Config{LogDir: dir}
	cases := []struct {
		name string
		args []string
		opts logsOptions
		want string
	}{
		{"latest session", nil, logsOptions{}, "tui log\n"},
		{"by cmd, parts in order", nil, logsOptions{cmd: "get"}, "one\ntwo\nthree\n"},
		{"by pid", []string{"2147483000"}, logsOptions{}, "one\ntwo\nthree\n"},
		{"by id prefix", []string{getID[:20]}, logsOptions{}, "one\ntwo\nthree\n"},
		{"stderr", nil, logsOptions{stderr: true}, "plugin noise\n"},
		{"path", nil, logsOptions{path: true}, filepath.Join(dir, tuiID+".log") + "\n"},
		{"stderr path", nil, logsOptions{stderr: true, path: true}, filepath.Join(dir, tuiID+".log.stderr") + "\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := runLogs(context.Background(), &out, cfg, tc.args, tc.opts); err != nil {
				t.Fatal(err)
			}
			if out.String() != tc.want {
				t.Errorf("got %q, want %q", out.String(), tc.want)
			}
		})
	}
}

// A PID that prefixes a newer session's ID (which starts with its
// date) picks the PID's session, not the newer one.
func TestPickSession_PIDBeatsIDPrefix(t *testing.T) {
	now := time.Now()
	newer := logging.Session{ID: logging.SessionID(now, "get", 999), Cmd: "get", PID: 999}
	year := newer.ID[:4]
	pid, err := strconv.Atoi(year)
	if err != nil {
		t.Fatal(err)
	}
	older := logging.Session{ID: logging.SessionID(now.Add(-time.Hour), "get", pid), Cmd: "get", PID: pid}
	got, err := pickSession([]logging.Session{newer, older}, []string{year}, "")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != older.ID {
		t.Errorf("picked %s, want PID %d's session %s", got.ID, pid, older.ID)
	}
}

func TestRunLogs_Errors(t *testing.T) {
	dir := t.TempDir()
	seedSessions(t, dir)
	cases := []struct {
		name string
		cfg  config.Config
		args []string
		opts logsOptions
		want string
	}{
		{"no sessions", config.Config{LogDir: t.TempDir()}, nil, logsOptions{}, "no log sessions in"},
		{"no match", config.Config{LogDir: dir}, []string{"999"}, logsOptions{}, `no log session matches "999"`},
		{"no cmd match", config.Config{LogDir: dir}, nil, logsOptions{cmd: "mcp"}, "no mcp log sessions"},
		{"no stderr", config.Config{LogDir: dir}, nil, logsOptions{cmd: "get", stderr: true}, "captured no stderr"},
		{"log-file has no sessions", config.Config{LogFile: "/tmp/x.log"}, []string{"1"}, logsOptions{}, "no sessions to select"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := runLogs(context.Background(), &bytes.Buffer{}, tc.cfg, tc.args, tc.opts)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}

func TestRunLogs_LogFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "single.log")
	writeFile(t, path, "single\n")
	var out bytes.Buffer
	if err := runLogs(context.Background(), &out, config.Config{LogFile: path}, nil, logsOptions{}); err != nil {
		t.Fatal(err)
	}
	if out.String() != "single\n" {
		t.Errorf("got %q", out.String())
	}
}

// syncBuffer is a bytes.Buffer safe to read while followFile writes.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func waitFor(t *testing.T, out *syncBuffer, want string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(out.String(), want) {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %q; got %q", want, out.String())
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestFollowFile_AppendsRotationAndExit(t *testing.T) {
	old := followInterval
	followInterval = 5 * time.Millisecond
	t.Cleanup(func() { followInterval = old })

	path := filepath.Join(t.TempDir(), "s.log")
	writeFile(t, path, "a\n")
	var alive sync.Mutex
	running := true
	isAlive := func() bool { alive.Lock(); defer alive.Unlock(); return running }

	out := &syncBuffer{}
	done := make(chan error, 1)
	go func() { done <- followFile(context.Background(), out, path, isAlive) }()
	waitFor(t, out, "a\n")

	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600) //nolint:gosec // G304: test temp path
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString("b\n")
	_ = f.Close()
	waitFor(t, out, "a\nb\n")

	// Rotation: lumberjack renames the file away and starts a new one.
	if err := os.Rename(path, path+".1"); err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, "c\n")
	waitFor(t, out, "a\nb\nc\n")

	alive.Lock()
	running = false
	alive.Unlock()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("followFile did not return after the session exited")
	}
	if got := out.String(); got != "a\nb\nc\n" {
		t.Errorf("got %q", got)
	}
}

// Lines appended after the last drain but before a rotation are still
// printed, ahead of the new file's.
func TestTailer_ReadsRotatedFileToEnd(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.log")
	writeFile(t, path, "a\n")
	var out bytes.Buffer
	tl := &tailer{path: path, w: &out}
	defer tl.close()
	if err := tl.drain(); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600) //nolint:gosec // G304: test temp path
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString("b\n")
	_ = f.Close()
	if err := os.Rename(path, path+".1"); err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, "c\n")
	if err := tl.drain(); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "a\nb\nc\n" {
		t.Errorf("got %q", got)
	}
}

func TestFollowFile_StopsOnContext(t *testing.T) {
	old := followInterval
	followInterval = 5 * time.Millisecond
	t.Cleanup(func() { followInterval = old })

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	// The file doesn't exist yet: follow waits for the first write.
	go func() { done <- followFile(ctx, &syncBuffer{}, filepath.Join(t.TempDir(), "s.log"), nil) }()
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("followFile ignored ctx cancellation")
	}
}

// runLogsCmd executes `toolkit --log-dir dir <args>` and returns its output.
func runLogsCmd(t *testing.T, dir string, args ...string) string {
	t.Helper()
	viper.Reset()
	t.Cleanup(viper.Reset)
	cmd := NewRootCmd("vtest")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(append([]string{"--config", "", "--log-dir", dir}, args...))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("%v: %v\n%s", args, err, out.String())
	}
	return out.String()
}

func TestLogsCommand_ListTable(t *testing.T) {
	dir := t.TempDir()
	seedSessions(t, dir)
	table := runLogsCmd(t, dir, "logs", "ls")
	lines := strings.Split(strings.TrimSpace(table), "\n")
	if len(lines) != 3 || !strings.HasPrefix(lines[0], "STARTED") ||
		!strings.Contains(lines[1], "tui") || !strings.Contains(lines[1], "running") ||
		!strings.Contains(lines[2], "get") || !strings.Contains(lines[2], "exited") {
		t.Errorf("logs ls table:\n%s", table)
	}
}

func TestLogsCommand_ListJSON(t *testing.T) {
	dir := t.TempDir()
	seedSessions(t, dir)
	var views []sessionView
	if err := json.Unmarshal([]byte(runLogsCmd(t, dir, "logs", "ls", "-o", "json")), &views); err != nil {
		t.Fatal(err)
	}
	if len(views) != 2 || views[0].Cmd != "tui" || views[0].Stderr == "" || len(views[1].Backups) != 2 {
		t.Errorf("logs ls json = %+v", views)
	}
}

// A positional SESSION isn't mistaken for an unknown subcommand.
func TestLogsCommand_SessionArg(t *testing.T) {
	dir := t.TempDir()
	seedSessions(t, dir)
	if got := runLogsCmd(t, dir, "logs", "2147483000"); got != "one\ntwo\nthree\n" {
		t.Errorf("logs <pid> = %q", got)
	}
}

func TestHumanBytes(t *testing.T) {
	for n, want := range map[int64]string{0: "0 B", 1023: "1023 B", 1024: "1.0 KB", 1536: "1.5 KB", 8 << 20: "8.0 MB", 3 << 30: "3.0 GB"} {
		if got := humanBytes(n); got != want {
			t.Errorf("humanBytes(%d) = %q, want %q", n, got, want)
		}
	}
}
