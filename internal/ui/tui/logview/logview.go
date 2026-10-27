// Package logview is the TUI's log overlay: the in-memory log ring shown
// in a scrolling panel under a breadcrumb, with a minimum-level filter
// (l cycles it) and a case-insensitive text filter (/ types it).
//
// While the user is at the bottom the panel follows the tail; scrolling
// up pauses it, and end (or scrolling back down) resumes. home jumps to
// the top. esc clears the filters first and closes once they are clear;
// ` closes too. While the filter input is up, keys and pastes type into
// it: enter keeps the filter, esc clears it.
//
// Model is a value type in the Bubble Tea nested-model style: Update and
// the mutators return the updated Model, which the parent stores back,
// and Update returns a Result that says whether the log closed or the
// program should quit. The Result is returned synchronously, so the
// parent acts on it within the same keypress.
//
// The body is laid out ahead of rendering: Refresh re-reads the ring at
// a size and in styles the parent gives (on open, on each Tick, on
// resize and restyle), and filter or level changes re-lay it out
// themselves. View only renders, so it may be called any number of times
// without changing the Model.
package logview

import (
	"fmt"
	"image/color"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	logging "github.com/jingle2008/toolkit/internal/logging"
	keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"
	view "github.com/jingle2008/toolkit/internal/ui/tui/view"
)

// RefreshInterval is how often Tick fires while the log is open.
const RefreshInterval = 400 * time.Millisecond

// TickMsg drives the periodic refresh of the open log, so the live tail
// updates even while the app is otherwise idle.
type TickMsg struct{}

// Tick schedules the next TickMsg.
func Tick() tea.Cmd {
	return tea.Tick(RefreshInterval, func(time.Time) tea.Msg { return TickMsg{} })
}

// Outcome is what a message decided.
type Outcome int

// The outcomes: None leaves the log open.
const (
	None Outcome = iota
	Closed
	Quit
)

// Result is what Update decided: on Closed the parent restores the view
// the log was opened over; on Quit it quits the program.
type Result struct {
	Outcome Outcome
}

// Styles are the colors the log is drawn in, taken from the parent's
// theme.
type Styles struct {
	Text    color.Color // the panel title
	Accent  color.Color // the panel border, filter chips
	Muted   color.Color // the entry count, key descriptions, no-match note
	Primary color.Color // key names in the footer

	Context   lipgloss.Style // the Log chip in the breadcrumb
	Following lipgloss.Style // the following state
	Paused    lipgloss.Style // the PAUSED state

	Warn  lipgloss.Style
	Error lipgloss.Style
	Debug lipgloss.Style
}

// Model is the log overlay's state. Its zero value is not ready to use;
// make one with New.
type Model struct {
	store    *logging.RingSink
	viewport viewport.Model
	ready    bool

	// minLevel hides entries below it; filter keeps entries containing it
	// (case-insensitive), typed into input while typing.
	minLevel logging.Level
	filter   string
	input    textinput.Model
	typing   bool

	// The layout the body was last refreshed at, and what it showed.
	width  int
	styles Styles
	shown  int
	total  int
}

// New makes a log over store (nil when there is none).
func New(store *logging.RingSink) Model {
	vp := viewport.New(viewport.WithWidth(20), viewport.WithHeight(20))
	// v2 viewports bind h/l to horizontal scroll; h is the app's Help, and
	// the entries wrap, so there is nothing to scroll sideways.
	vp.KeyMap.Left.SetEnabled(false)
	vp.KeyMap.Right.SetEnabled(false)
	return Model{store: store, viewport: vp, ready: true}
}

// Ready reports whether m was made by New.
func (m Model) Ready() bool { return m.ready }

// Store returns the ring the log reads.
func (m Model) Store() *logging.RingSink { return m.store }

// Filter returns the text filter.
func (m Model) Filter() string { return m.filter }

// MinLevel returns the minimum level shown.
func (m Model) MinLevel() logging.Level { return m.minLevel }

// Typing reports whether the filter input is up.
func (m Model) Typing() bool { return m.typing }

// Following reports whether the panel is at the bottom, following the
// tail.
func (m Model) Following() bool { return m.viewport.AtBottom() }

// Counts returns how many entries the body shows of how many the ring
// held at the last refresh.
func (m Model) Counts() (shown, total int) { return m.shown, m.total }

// Viewport returns a copy of the body's viewport.
func (m Model) Viewport() viewport.Model { return m.viewport }

// Refresh re-reads the ring and lays the body out width columns wide
// (the whole frame; the panel border is inside it) and bodyHeight rows
// tall, in st. It keeps following the tail when the panel was at the
// bottom and otherwise leaves the scroll offset alone.
func (m Model) Refresh(width, bodyHeight int, st Styles) Model {
	m.width, m.styles = width, st
	m.viewport.SetWidth(max(width-2, 1)) // panel borders
	m.viewport.SetHeight(max(bodyHeight, 1))
	m.refresh()
	return m
}

// SetFilter replaces the text filter and re-lays the body out.
func (m Model) SetFilter(s string) Model {
	m.filter = s
	m.refresh()
	return m
}

// refresh rebuilds the body from the ring at the current layout,
// following the tail if the panel was at the bottom.
func (m *Model) refresh() {
	follow := m.viewport.AtBottom()
	body, shown, total := m.render(max(m.viewport.Width()-2, 1))
	m.shown, m.total = shown, total
	m.viewport.SetContent(view.IndentLines(body, " "))
	if follow {
		m.viewport.GotoBottom()
	}
}

// LevelLabel returns the fixed-width display label for a log level.
func LevelLabel(l logging.Level) string {
	switch l {
	case logging.LevelDebug:
		return "DEBUG"
	case logging.LevelInfo:
		return "INFO"
	case logging.LevelWarn:
		return "WARN"
	case logging.LevelError:
		return "ERROR"
	default:
		return "?"
	}
}

// levelStyle returns the color style for a log level.
func (st Styles) levelStyle(l logging.Level) lipgloss.Style {
	switch l {
	case logging.LevelWarn:
		return st.Warn
	case logging.LevelError:
		return st.Error
	case logging.LevelDebug:
		return st.Debug
	default:
		return lipgloss.NewStyle()
	}
}

// FormatEntry renders one entry as a single uncolored line:
// "15:04:05 LEVEL message k=v k=v".
func FormatEntry(e logging.Entry) string {
	var b strings.Builder
	b.WriteString(e.Time.Format("15:04:05"))
	b.WriteByte(' ')
	fmt.Fprintf(&b, "%-5s ", LevelLabel(e.Level))
	b.WriteString(e.Message)
	for i := 0; i+1 < len(e.Fields); i += 2 {
		fmt.Fprintf(&b, " %v=%v", e.Fields[i], e.Fields[i+1])
	}
	return b.String()
}

// passes reports whether e passes the level and text filters.
func (m Model) passes(e logging.Entry) bool {
	if e.Level < m.minLevel {
		return false
	}
	if m.filter == "" {
		return true
	}
	return strings.Contains(strings.ToLower(FormatEntry(e)), strings.ToLower(m.filter))
}

// render builds the log body — one color-coded entry per record that
// passes the filters, oldest first — and counts what it shows of the
// total. Entries wider than the view wrap (word-wrap, with long unbroken
// tokens like OCIDs hard-broken) so the whole line is readable rather
// than clipped with an ellipsis.
func (m Model) render(width int) (body string, shown, total int) {
	if m.store == nil {
		return "(log store unavailable)", 0, 0
	}
	entries := m.store.Snapshot()
	if len(entries) == 0 {
		return "(no log entries yet)", 0, 0
	}
	lines := make([]string, 0, len(entries))
	for _, e := range entries {
		if !m.passes(e) {
			continue
		}
		style := m.styles.levelStyle(e.Level)
		if width > 0 {
			style = style.Width(width) // wrap to the view width
		}
		lines = append(lines, style.Render(FormatEntry(e)))
	}
	if len(lines) == 0 {
		return lipgloss.NewStyle().Foreground(m.styles.Muted).
			Render("no entries match · esc clears the filters"), 0, len(entries)
	}
	return strings.Join(lines, "\n"), len(lines), len(entries)
}

// View renders the log below the parent's env bar: a breadcrumb with the
// follow/pause state and the level and text filters (or the filter
// input); the entries in a panel whose bottom edge counts them; and the
// log's keys. It does not change m.
func (m Model) View() string {
	p := m.styles
	titleStyle := lipgloss.NewStyle().Foreground(p.Text).Bold(true)
	status := fmt.Sprintf("%d entries", m.total)
	if m.shown < m.total {
		status = fmt.Sprintf("%d of %d", m.shown, m.total)
	}
	// The log has focus, so its border is Accent, as the list's is.
	panel := view.StatusPanel("", status, m.viewport.View(), p.Accent, titleStyle,
		lipgloss.NewStyle().Foreground(p.Muted), m.width)
	return lipgloss.JoinVertical(lipgloss.Left, m.crumb(), panel, m.footer())
}

// crumb is the log's breadcrumb line: the Log chip, the state (following
// the tail, or PAUSED), and the level and text filter chips or the
// filter input.
func (m Model) crumb() string {
	w := lipgloss.Width
	st := m.styles
	ctx := st.Context.Render("Log")
	state := st.Following.Padding(0, 1).Render("following")
	if !m.viewport.AtBottom() {
		state = st.Paused.Padding(0, 1).Render("PAUSED")
	}
	left := ctx + " " + state
	var middle string
	if m.typing {
		ti := m.input
		ti.SetWidth(max(m.width-w(left)-w(ti.Prompt)-2, 0))
		middle = " " + ti.View()
	} else {
		chip := lipgloss.NewStyle().Foreground(st.Accent)
		if m.minLevel > logging.LevelDebug {
			middle += " " + chip.Render("⟨≥"+strings.ToLower(LevelLabel(m.minLevel))+"⟩")
		}
		if m.filter != "" {
			middle += " " + chip.Render("⟨/"+m.filter+"⟩")
		}
	}
	room := m.width - w(left)
	if w(middle) > room {
		middle = view.Truncate(ansi.Strip(middle), max(room, 1))
	}
	return left + middle + strings.Repeat(" ", max(room-w(middle), 0))
}

// footer lists the log's keys, trimmed from the right to fit.
func (m Model) footer() string {
	keyStyle := lipgloss.NewStyle().Foreground(m.styles.Primary).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(m.styles.Muted)
	pairs := [][2]string{{"/", "filter"}, {"l", "level"}, {"end", "follow"}, {"home", "top"}, {"`", "close"}}
	if m.typing {
		pairs = [][2]string{{"enter", "apply"}, {"esc", "clear"}}
	}
	items := make([]string, 0, len(pairs))
	for _, kv := range pairs {
		items = append(items, keyStyle.Render(kv[0])+" "+descStyle.Render(kv[1]))
	}
	for n := len(items); n > 0; n-- {
		if line := " " + strings.Join(items[:n], "  "); lipgloss.Width(line) <= m.width {
			return line
		}
	}
	return ""
}

// Update handles input while the log is open. While the filter input is
// up, keys and pastes type into it (ctrl+c still quits). Otherwise q
// quits, / opens the input, l cycles the minimum level, home/end jump
// (the viewport keymap lacks them), esc clears the filters and then
// closes, ` closes, and the rest scrolls.
func (m Model) Update(msg tea.Msg) (Model, Result, tea.Cmd) {
	if paste, ok := msg.(tea.PasteMsg); ok && m.typing {
		cmd := m.updateFilter(paste)
		return m, Result{}, cmd
	}
	km, ok := msg.(tea.KeyPressMsg)
	if ok && m.typing {
		res, cmd := m.updateInput(km)
		return m, res, cmd
	}
	if ok {
		if res, cmd, handled := m.key(km); handled {
			return m, res, cmd
		}
	}
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, Result{}, cmd
}

// key runs the log's own keys and reports whether km was one.
func (m *Model) key(km tea.KeyPressMsg) (Result, tea.Cmd, bool) {
	switch {
	case key.Matches(km, keys.Quit):
		return Result{Outcome: Quit}, nil, true
	case key.Matches(km, keys.Back) && (m.filter != "" || m.minLevel > logging.LevelDebug):
		m.filter, m.minLevel = "", logging.LevelDebug
		m.refresh()
		return Result{}, nil, true
	case key.Matches(km, keys.ToggleLog, keys.Back):
		return Result{Outcome: Closed}, nil, true
	}
	switch km.String() {
	case "/":
		ti := textinput.New()
		ti.Prompt = "/"
		ti.SetValue(m.filter)
		m.input, m.typing = ti, true
		return Result{}, m.input.Focus(), true
	case "l":
		m.minLevel = (m.minLevel + 1) % (logging.LevelError + 1)
		m.refresh()
		return Result{}, nil, true
	case "end":
		m.viewport.GotoBottom()
		return Result{}, nil, true
	case "home":
		m.viewport.SetYOffset(0)
		return Result{}, nil, true
	}
	return Result{}, nil, false
}

// updateInput feeds km to the filter input, filtering as it is typed.
func (m *Model) updateInput(km tea.KeyPressMsg) (Result, tea.Cmd) {
	switch km.String() {
	case "ctrl+c":
		return Result{Outcome: Quit}, nil
	case "enter":
		m.typing = false
		return Result{}, nil
	case "esc":
		m.typing, m.filter = false, ""
		m.refresh()
		return Result{}, nil
	}
	return Result{}, m.updateFilter(km)
}

// updateFilter feeds msg (a key or a paste) to the filter input and
// re-lays the body out when the filter changed.
func (m *Model) updateFilter(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	if f := strings.TrimSpace(m.input.Value()); f != m.filter {
		m.filter = f
		m.refresh()
	}
	return cmd
}
