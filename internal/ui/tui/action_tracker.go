package tui

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	"github.com/jingle2008/toolkit/pkg/models"
)

// runningAction is a mutation in flight: what it is, where it runs, and
// what it last reported. Actions outlive navigation and environment
// switches (only quitting cancels them), so each keeps its own env.
type runningAction struct {
	id     int
	env    models.Environment
	cat    domain.Category
	key    models.ItemKey
	label  string // "drain 10.0.0.11", as the indicator names it
	status string // the row's Status while it runs ("Draining"), or "" to leave it
	step   string // the latest progress line, or ""
	box    *progressBox
}

// actionDoneMsg carries a tracked action's result: the tracker drops the
// action, then msg is handled as it would be on its own.
type actionDoneMsg struct {
	id  int
	msg tea.Msg
}

// actionProgressMsg is a running action's latest progress line.
type actionProgressMsg struct {
	id   int
	line string
}

// trackAction runs an action on the current category's row key off the
// Update loop, showing it as running — in the env bar, and as status in
// the row's Status cell — until its result arrives. run gets a progress
// func for the action's steps and returns the result message. It refuses
// an action already running on the same target.
func (m *Model) trackAction(label, status string, key models.ItemKey, run func(progress func(string)) tea.Msg) tea.Cmd {
	if slices.ContainsFunc(m.actions, func(a runningAction) bool {
		return a.label == label && a.env.Equals(m.environment)
	}) {
		return m.showToast(label+" is already running", toastWarn)
	}
	var tick tea.Cmd
	if len(m.actions) == 0 && m.pendingTasks == 0 {
		tick = m.loadingSpinner.Tick // no load is animating the spinner
	}
	m.actionSeq++
	a := runningAction{
		id: m.actionSeq, env: m.environment, cat: m.category, key: key,
		label: label, status: status, box: newProgressBox(),
	}
	m.actions = append(m.actions, a)
	m.refreshActionRows(a)
	done := func() tea.Msg {
		msg := run(a.box.report)
		a.box.close()
		return actionDoneMsg{id: a.id, msg: msg}
	}
	return tea.Batch(done, a.box.next(a.id), tick)
}

// handleActionDone drops the finished action and hands back its result
// to handle.
func (m *Model) handleActionDone(msg actionDoneMsg) tea.Msg {
	i := slices.IndexFunc(m.actions, func(a runningAction) bool { return a.id == msg.id })
	if i >= 0 {
		a := m.actions[i]
		m.actions = slices.Delete(m.actions, i, i+1)
		m.refreshActionRows(a)
	}
	// The quit confirmation exists only because actions were running;
	// with the last one done it would ask about none, so close it (q
	// then quits at once).
	if len(m.actions) == 0 && m.viewMode == common.ConfirmView && m.confirm.quit {
		m.dismissConfirm()
	}
	return msg.msg
}

// handleActionProgress records a running action's latest step and waits
// for the next.
func (m *Model) handleActionProgress(msg actionProgressMsg) tea.Cmd {
	i := slices.IndexFunc(m.actions, func(a runningAction) bool { return a.id == msg.id })
	if i < 0 {
		return nil
	}
	m.actions[i].step = msg.line
	return m.actions[i].box.next(msg.id)
}

// refreshActionRows redraws the rows when a's status shows in them.
func (m *Model) refreshActionRows(a runningAction) {
	if a.status != "" && a.cat == m.category && a.env.Equals(m.environment) && m.dataset != nil {
		m.updateRows(false)
	}
}

// overlayActionStatus writes each running action's status into its row's
// Status cell. rows line up with m.rawRows, whose keys identify them. The
// status lives on the tracker, not the item, so a reload while the action
// runs can't erase it.
func (m *Model) overlayActionStatus(rows []table.Row) {
	col := slices.IndexFunc(m.headers, func(h header) bool { return strings.EqualFold(h.text, common.StatusCol) })
	if col < 0 {
		return
	}
	status := map[models.ItemKey]string{}
	for _, a := range m.actions {
		if a.status != "" && a.cat == m.category && a.env.Equals(m.environment) {
			status[a.key] = a.status
		}
	}
	if len(status) == 0 {
		return
	}
	for i, r := range rows {
		if i >= len(m.rawRows) || col >= len(r) {
			continue
		}
		if s, ok := status[itemKeyFrom(m.category, m.rawRows[i])]; ok {
			r[col] = s
		}
	}
}

// actionsIndicator is the env bar's running-actions cell: the newest
// action, its environment when it isn't the current one, and its latest
// step, led by how many are running when there are several. "" when none.
func (m *Model) actionsIndicator(maxWidth int) string {
	n := len(m.actions)
	if n == 0 {
		return ""
	}
	a := m.actions[n-1]
	text := a.label + m.envNote(a.env)
	if a.step != "" {
		text += ": " + a.step
	}
	if n > 1 {
		text = fmt.Sprintf("%d running · %s", n, text)
	}
	text = truncateString(text, max(maxWidth-2, 1))
	return m.loadingSpinner.View() + " " + lipgloss.NewStyle().Foreground(m.theme.Palette.Warn).Render(text)
}

// envNote names env, as " (dp-prod-ord)", when it isn't the current
// environment: an action's result can land after a switch.
func (m *Model) envNote(env models.Environment) string {
	if env.Equals(m.environment) {
		return ""
	}
	return " (" + env.KubeContext() + ")"
}

// runningSummary lists the running actions for the quit confirmation,
// each with its environment when it isn't the current one.
func (m *Model) runningSummary() []string {
	out := make([]string, 0, len(m.actions))
	for _, a := range m.actions {
		out = append(out, a.label+m.envNote(a.env))
	}
	return out
}

// progressBox carries an action's latest progress line to the UI. A
// report replaces any line not yet read, so a fast action never blocks
// on a slow redraw; reports after close are dropped.
type progressBox struct {
	mu     sync.Mutex
	closed bool
	ch     chan string
}

func newProgressBox() *progressBox { return &progressBox{ch: make(chan string, 1)} }

func (b *progressBox) report(line string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	select {
	case <-b.ch:
	default:
	}
	b.ch <- line
}

func (b *progressBox) close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.closed {
		b.closed = true
		close(b.ch)
	}
}

// next waits for the action's next progress line; nil once it is done.
func (b *progressBox) next(id int) tea.Cmd {
	return func() tea.Msg {
		line, ok := <-b.ch
		if !ok {
			return nil
		}
		return actionProgressMsg{id: id, line: line}
	}
}

// newProgressLogger returns a logger that writes through l and also
// reports each info, warning and error record as progress, for
// operations that log their steps rather than take a progress hook (DAC
// delete).
func newProgressLogger(l *slog.Logger, report func(string)) *slog.Logger {
	return slog.New(progressHandler{Handler: l.Handler(), report: report})
}

// progressHandler wraps the app's handler, reporting info and above as
// progress lines whatever level the wrapped handler writes at.
type progressHandler struct {
	slog.Handler
	report func(string)
}

func (h progressHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= slog.LevelInfo || h.Handler.Enabled(ctx, level)
}

func (h progressHandler) Handle(ctx context.Context, r slog.Record) error {
	if r.Level >= slog.LevelInfo {
		h.report(progressLine(r))
	}
	if !h.Handler.Enabled(ctx, r.Level) {
		return nil
	}
	return h.Handler.Handle(ctx, r)
}

func (h progressHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return progressHandler{Handler: h.Handler.WithAttrs(attrs), report: h.report}
}

func (h progressHandler) WithGroup(name string) slog.Handler {
	return progressHandler{Handler: h.Handler.WithGroup(name), report: h.report}
}

// progressLine is a log record as one progress summary: its message and
// the fields of that call (not those bound with With).
func progressLine(r slog.Record) string {
	var b strings.Builder
	b.WriteString(r.Message)
	r.Attrs(func(a slog.Attr) bool {
		fmt.Fprintf(&b, " %s=%v", a.Key, a.Value.Resolve().Any())
		return true
	})
	return b.String()
}
