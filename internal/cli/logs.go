package cli

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/jingle2008/toolkit/internal/config"
	"github.com/jingle2008/toolkit/internal/logging"
	"github.com/jingle2008/toolkit/internal/output"
)

// followInterval is how often `toolkit logs --follow` polls for new output.
var followInterval = 250 * time.Millisecond

type logsOptions struct {
	follow bool
	cmd    string
	stderr bool
	path   bool
}

// addLogsCommand wires `toolkit logs`, which reads the per-session log
// files every toolkit process writes under --log-dir.
func addLogsCommand(rootCmd *cobra.Command, cfgFile *string) {
	var opts logsOptions
	cmd := &cobra.Command{
		Use:   "logs [SESSION]",
		Short: "Show toolkit's log files (one per session)",
		Long: `Show the logs toolkit writes. Every toolkit process (tui, get, mcp,
mutations) logs to its own session file in --log-dir, named
<start>-<cmd>-<pid>.log; exited sessions beyond the newest 50, or idle
for 14 days, are pruned when the next one starts.

With no SESSION, prints the most recent session (narrowed by --cmd).
SESSION is a PID or a session ID (prefix); ` + "`toolkit logs ls`" + ` lists them.
Rotated and gzipped parts of a session print in order before the
current file.

When --log-file is set, logs go to that single file instead, and this
command reads it.

Examples:
  toolkit logs
  toolkit logs --follow
  toolkit logs --cmd mcp --follow
  toolkit logs 4242
  toolkit logs --stderr
  tail -f "$(toolkit logs --path)"
  toolkit logs ls`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			cfg, err := logsConfig(cfgFile)
			if err != nil {
				return err
			}
			return runLogs(c.Context(), c.OutOrStdout(), cfg, args, opts)
		},
	}
	cmd.Flags().BoolVar(&opts.follow, "follow", false, "keep printing new output until the session exits (Ctrl+C stops)")
	cmd.Flags().StringVar(&opts.cmd, "cmd", "", "only consider sessions of this command (tui, get, mcp, ...)")
	cmd.Flags().BoolVar(&opts.stderr, "stderr", false, "show the session's captured stderr (TUI only) instead of its log")
	cmd.Flags().BoolVar(&opts.path, "path", false, "print the file path instead of its contents")
	cmd.AddCommand(newLogsListCommand(cfgFile))
	rootCmd.AddCommand(cmd)
}

func newLogsListCommand(cfgFile *string) *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List log sessions, newest first",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			cfg, err := logsConfig(cfgFile)
			if err != nil {
				return err
			}
			if cfg.LogFile != "" {
				return fmt.Errorf("log-file is set (%s): logs go to that one file, not to sessions", cfg.LogFile)
			}
			sessions, err := logging.ListSessions(logDir(cfg))
			if err != nil {
				return err
			}
			return writeSessions(c.OutOrStdout(), sessions, format)
		},
	}
	cmd.Flags().StringVarP(&format, "output", "o", "table", "table|json")
	_ = cmd.RegisterFlagCompletionFunc("output", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return []string{"table", "json"}, cobra.ShellCompDirectiveNoFileComp
	})
	return cmd
}

// logsConfig is the merged config, for its log-dir and log-file only:
// reading logs mustn't need the env settings other commands validate.
func logsConfig(cfgFile *string) (config.Config, error) {
	var cfg config.Config
	if err := readConfigFile(cfgFile); err != nil {
		return cfg, err
	}
	if err := viper.Unmarshal(&cfg); err != nil {
		return cfg, fmt.Errorf("unmarshal config: %w", err)
	}
	return cfg, nil
}

// logDir is cfg.LogDir, or the default when a config file blanked it.
func logDir(cfg config.Config) string {
	if cfg.LogDir != "" {
		return cfg.LogDir
	}
	return logging.DefaultDir()
}

func runLogs(ctx context.Context, w io.Writer, cfg config.Config, args []string, opts logsOptions) error {
	if cfg.LogFile != "" {
		if len(args) > 0 || opts.cmd != "" {
			return fmt.Errorf("log-file is set (%s): there are no sessions to select", cfg.LogFile)
		}
		path := cfg.LogFile
		if opts.stderr {
			path += ".stderr"
		}
		return showLog(ctx, w, []string{path}, nil, opts)
	}

	dir := logDir(cfg)
	sessions, err := logging.ListSessions(dir)
	if err != nil {
		return err
	}
	s, err := pickSession(sessions, args, opts.cmd)
	if err != nil {
		return fmt.Errorf("%w in %s", err, dir)
	}
	files := s.Files()
	if opts.stderr {
		if s.Stderr == "" {
			return fmt.Errorf("session %s captured no stderr", s.ID)
		}
		files = []string{s.Stderr}
	} else if s.Stderr != "" {
		files = files[:len(files)-1]
	}
	alive := func() bool { return s.Running && processAliveFn(s.PID) }
	return showLog(ctx, w, files, alive, opts)
}

// processAliveFn is the liveness probe behind --follow's exit; a seam for tests.
var processAliveFn = logging.ProcessAlive

// pickSession returns the newest session matching cmd (when set) and
// the SESSION argument (when given): a PID, or a session ID prefix.
func pickSession(sessions []logging.Session, args []string, cmd string) (logging.Session, error) {
	for _, s := range sessions {
		if cmd != "" && s.Cmd != cmd {
			continue
		}
		if len(args) == 0 || strconv.Itoa(s.PID) == args[0] || strings.HasPrefix(s.ID, args[0]) {
			return s, nil
		}
	}
	switch {
	case len(args) > 0:
		return logging.Session{}, fmt.Errorf("no log session matches %q", args[0])
	case cmd != "":
		return logging.Session{}, fmt.Errorf("no %s log sessions", cmd)
	default:
		return logging.Session{}, errors.New("no log sessions")
	}
}

// showLog prints files in order (or, with opts.path, the last one's
// path), then with opts.follow keeps printing what's appended to the
// last until alive reports false or ctx ends. A nil alive follows until
// ctx ends.
func showLog(ctx context.Context, w io.Writer, files []string, alive func() bool, opts logsOptions) error {
	last := files[len(files)-1]
	if opts.path {
		_, err := fmt.Fprintln(w, last)
		return err
	}
	for _, f := range files[:len(files)-1] {
		if err := copyLogFile(w, f); err != nil {
			return err
		}
	}
	if !opts.follow {
		return copyLogFile(w, last)
	}
	return followFile(ctx, w, last, alive)
}

// copyLogFile writes path to w, gunzipping .gz files. A missing file is
// empty: the current log only appears on a session's first write.
func copyLogFile(w io.Writer, path string) error {
	f, err := os.Open(path) //nolint:gosec // G304: a file in the user's own log dir
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	var r io.Reader = f
	if strings.HasSuffix(path, ".gz") {
		gz, err := gzip.NewReader(f)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		defer func() { _ = gz.Close() }()
		r = gz
	}
	_, err = io.Copy(w, r)
	return err
}

// followFile prints path and then whatever is appended to it, polling
// every followInterval. When lumberjack rotates the file (a new inode
// at path) or it shrinks, it starts over on the new file. It returns
// once alive reports false, after a final read, or when ctx ends.
func followFile(ctx context.Context, w io.Writer, path string, alive func() bool) error {
	t := &tailer{path: path, w: w}
	defer t.close()
	ticker := time.NewTicker(followInterval)
	defer ticker.Stop()
	for {
		if err := t.drain(); err != nil {
			return err
		}
		if alive != nil && !alive() {
			return t.drain()
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

// tailer copies what's been appended to path since its last drain.
type tailer struct {
	path string
	w    io.Writer
	f    *os.File
	off  int64
}

// drain writes path's new bytes to w, reopening from the start when
// path is a different file than the one held, or shorter than what was
// already read. A path that doesn't exist yet drains nothing.
func (t *tailer) drain() error {
	st, err := os.Stat(t.path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if t.f != nil {
		if cur, err := t.f.Stat(); err != nil || !os.SameFile(cur, st) || st.Size() < t.off {
			t.close()
		}
	}
	if t.f == nil {
		if t.f, err = os.Open(t.path); err != nil {
			return err
		}
	}
	n, err := io.Copy(t.w, io.NewSectionReader(t.f, t.off, 1<<62))
	t.off += n
	return err
}

func (t *tailer) close() {
	if t.f != nil {
		_ = t.f.Close()
	}
	t.f, t.off = nil, 0
}

// sessionView is the -o json shape of `toolkit logs ls`.
type sessionView struct {
	ID        string    `json:"id"`
	Started   time.Time `json:"started"`
	Cmd       string    `json:"cmd"`
	PID       int       `json:"pid"`
	Running   bool      `json:"running"`
	SizeBytes int64     `json:"size_bytes"`
	Modified  time.Time `json:"modified"`
	Path      string    `json:"path"`
	Stderr    string    `json:"stderr,omitempty"`
	Backups   []string  `json:"backups,omitempty"`
}

func writeSessions(w io.Writer, sessions []logging.Session, format string) error {
	switch format {
	case "json":
		views := make([]sessionView, len(sessions))
		for i, s := range sessions {
			views[i] = sessionView{
				ID: s.ID, Started: s.Started, Cmd: s.Cmd, PID: s.PID, Running: s.Running,
				SizeBytes: s.Size, Modified: s.ModTime, Path: s.Path, Stderr: s.Stderr, Backups: s.Backups,
			}
		}
		return output.WriteJSON(w, views, output.Options{Pretty: true})
	case "table", "":
		rows := make([][]string, len(sessions))
		for i, s := range sessions {
			status := "exited"
			if s.Running {
				status = "running"
			}
			rows[i] = []string{
				s.Started.Format(time.DateTime), s.Cmd, strconv.Itoa(s.PID), humanBytes(s.Size), status,
			}
		}
		return output.WriteTable(w, []string{"STARTED", "CMD", "PID", "SIZE", "STATUS"}, rows, output.Options{})
	default:
		return fmt.Errorf("invalid output format %q (valid: table|json)", format)
	}
}

// humanBytes renders n in binary units, one decimal from KB up.
func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}
