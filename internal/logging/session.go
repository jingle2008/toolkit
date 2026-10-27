package logging

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"time"
)

// Session retention, applied by PruneSessions at every session start.
// Running sessions are never removed; exited ones go once they fall
// outside the newest MaxSessions or haven't been written for MaxAge.
const (
	MaxSessions = 50
	MaxAge      = 14 * 24 * time.Hour
)

// sessionTimeLayout is the start-time prefix of a session ID. Local
// time, so `ls` of the directory reads in the user's clock.
const sessionTimeLayout = "20060102T150405"

// sessionFile matches every file a session owns: the current log
// (<id>.log), lumberjack's rotated backups (<id>-<stamp>.log[.gz]) and
// the TUI's stderr capture (<id>.log.stderr). The cmd group is lazy so
// a backup's -<stamp> suffix isn't read as part of the pid.
var sessionFile = regexp.MustCompile(
	`^((\d{8}T\d{6})-([a-z][a-z0-9-]*?)-(\d+))(-\d{4}-\d{2}-\d{2}T\d{2}-\d{2}-\d{2}\.\d{3})?\.log(\.gz|\.stderr)?$`)

// Session is one process's log files in the session directory.
type Session struct {
	ID      string    // <start>-<cmd>-<pid>, the shared file-name prefix
	Started time.Time // from the ID
	Cmd     string    // the command that wrote it (tui, get, mcp, ...)
	PID     int
	Path    string   // current log file; may not exist yet if nothing was logged
	Stderr  string   // TUI stderr capture, empty when there is none
	Backups []string // rotated logs, oldest first
	Size    int64    // bytes across the log, backups and stderr capture
	ModTime time.Time
	Running bool
}

// DefaultDir is where session logs go unless --log-dir says otherwise:
// $XDG_STATE_HOME/toolkit/logs, falling back to
// ~/.local/state/toolkit/logs (also on macOS, matching the ~/.config
// layout the rest of toolkit uses) or %LocalAppData%\toolkit\logs on
// Windows. Logs are state, not config or cache, per the XDG spec.
func DefaultDir() string {
	if d := os.Getenv("XDG_STATE_HOME"); filepath.IsAbs(d) {
		return filepath.Join(d, "toolkit", "logs")
	}
	if runtime.GOOS == "windows" {
		if d := os.Getenv("LocalAppData"); d != "" {
			return filepath.Join(d, "toolkit", "logs")
		}
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "state", "toolkit", "logs")
}

// SessionID names a session started at t by command cmd in process pid.
func SessionID(t time.Time, cmd string, pid int) string {
	return fmt.Sprintf("%s-%s-%d", t.Format(sessionTimeLayout), cmd, pid)
}

// NewSession prunes dir (PruneSessions) and returns a logger writing
// to a fresh per-process file there, named by SessionID, along with
// that file's path. Each process owns its file, so lumberjack's
// single-writer rotation (see NewFile) holds even when several
// toolkit instances run at once. The file appears on the first write.
func NewSession(dir, cmd, format, level string, debug bool) (*slog.Logger, io.Closer, string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, nil, "", fmt.Errorf("create log dir: %w", err)
	}
	now := time.Now()
	// Retention is best-effort: a failure costs disk, never the launch.
	_, pruneErr := PruneSessions(dir, now)
	path := filepath.Join(dir, SessionID(now, cmd, os.Getpid())+".log")
	logger, closer := NewFile(path, format, level, debug)
	if pruneErr != nil {
		logger.Warn("prune log sessions", "dir", dir, "error", pruneErr)
	}
	return logger, closer, path, nil
}

// ListSessions returns the sessions in dir, newest first. A missing dir
// is no sessions; files that don't follow the session naming are
// ignored.
func ListSessions(dir string) ([]Session, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	byID := map[string]*Session{}
	for _, e := range entries {
		if e.Type().IsRegular() {
			addSessionFile(byID, dir, e)
		}
	}
	out := make([]Session, 0, len(byID))
	for _, s := range byID {
		slices.Sort(s.Backups) // the stamp sorts chronologically
		s.Running = ProcessAlive(s.PID)
		out = append(out, *s)
	}
	slices.SortFunc(out, func(a, b Session) int {
		if c := b.Started.Compare(a.Started); c != 0 {
			return c
		}
		return b.PID - a.PID
	})
	return out, nil
}

// addSessionFile files dir entry e under its session in byID, creating
// the session on its first file. Names that aren't session files are
// skipped.
func addSessionFile(byID map[string]*Session, dir string, e fs.DirEntry) {
	m := sessionFile.FindStringSubmatch(e.Name())
	if m == nil {
		return
	}
	info, err := e.Info()
	if err != nil {
		return // removed since ReadDir
	}
	s := byID[m[1]]
	if s == nil {
		started, err := time.ParseInLocation(sessionTimeLayout, m[2], time.Local)
		pid, perr := strconv.Atoi(m[4])
		if err != nil || perr != nil {
			return
		}
		s = &Session{ID: m[1], Started: started, Cmd: m[3], PID: pid, Path: filepath.Join(dir, m[1]+".log")}
		byID[m[1]] = s
	}
	full := filepath.Join(dir, e.Name())
	switch {
	case m[6] == ".stderr":
		s.Stderr = full
	case m[5] != "":
		s.Backups = append(s.Backups, full)
	}
	s.Size += info.Size()
	if info.ModTime().After(s.ModTime) {
		s.ModTime = info.ModTime()
	}
}

// Files returns every file the session owns, in reading order: backups
// oldest first, then the current log, then the stderr capture.
func (s Session) Files() []string {
	files := slices.Clone(s.Backups)
	files = append(files, s.Path)
	if s.Stderr != "" {
		files = append(files, s.Stderr)
	}
	return files
}

// PruneSessions removes exited sessions beyond the newest MaxSessions,
// or last written more than MaxAge before now, and reports how many it
// removed. Running sessions are always kept, and so is anything a
// concurrent prune already deleted.
func PruneSessions(dir string, now time.Time) (int, error) {
	sessions, err := ListSessions(dir)
	if err != nil {
		return 0, err
	}
	var errs []error
	removed, kept := 0, 0
	for _, s := range sessions {
		if s.Running {
			continue
		}
		if kept < MaxSessions && now.Sub(s.ModTime) <= MaxAge {
			kept++
			continue
		}
		for _, f := range s.Files() {
			if err := os.Remove(f); err != nil && !errors.Is(err, fs.ErrNotExist) {
				errs = append(errs, err)
			}
		}
		removed++
	}
	return removed, errors.Join(errs...)
}
