package tui

import (
	"fmt"
	"reflect"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	view "github.com/jingle2008/toolkit/internal/ui/tui/view"
)

// previewMinWidth is the narrowest terminal that shows the preview pane;
// below it the table needs every column.
const previewMinWidth = 160

// previewShown reports whether the list view draws the preview pane now:
// toggled on, in list view, on a wide enough terminal.
func (m *Model) previewShown() bool {
	return m.preview && m.viewMode == common.ListView && m.viewWidth >= previewMinWidth
}

// previewOuterWidth is the preview panel's width, borders included: 40%
// of the terminal.
func (m *Model) previewOuterWidth() int {
	return m.viewWidth * 2 / 5
}

// togglePreview flips the preview pane for the session. On a terminal too
// narrow for it, it says so instead.
func (m *Model) togglePreview() tea.Cmd {
	if !m.preview && m.viewWidth < previewMinWidth {
		return m.showToast(fmt.Sprintf("preview needs ≥%d columns (have %d)", previewMinWidth, m.viewWidth), toastInfo)
	}
	m.preview = !m.preview
	m.updateLayout(m.viewWidth, m.viewHeight)
	return nil
}

// previewPanel renders the selected row's summary in a panel titled with
// its name, height rows tall to match the table's panel.
func (m *Model) previewPanel(height int) string {
	p := m.theme.Palette
	width := m.previewOuterWidth()
	row := m.selectedRawRow()
	title := ""
	if len(row) > 0 {
		title = row[0]
	}
	body := m.previewBody(width - 4) // borders and padding
	lines := strings.Split(body, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return view.Box(title, strings.Join(lines, "\n"), p.Border,
		lipgloss.NewStyle().Foreground(p.Primary).Bold(true), width)
}

// previewBody is the condensed key/value summary of the selected item,
// width columns wide.
func (m *Model) previewBody(width int) string {
	item := findItem(m.dataset, m.category, itemKeyFrom(m.category, m.selectedRawRow()))
	if item == nil {
		return lipgloss.NewStyle().Foreground(m.theme.Palette.Muted).Render("nothing selected")
	}
	pairs := summarize(item)
	keyW := 0
	for _, kv := range pairs {
		keyW = max(keyW, lipgloss.Width(kv[0]))
	}
	keyW = min(keyW, width/2)
	label := lipgloss.NewStyle().Foreground(m.theme.Palette.Muted).Width(keyW + 2)
	out := make([]string, 0, len(pairs))
	for _, kv := range pairs {
		out = append(out, label.Render(truncateString(kv[0], keyW))+truncateString(kv[1], max(width-keyW-2, 1)))
	}
	return strings.Join(out, "\n")
}

// summarize lists item's exported fields as (json name, value) pairs in
// declaration order: scalars as they print, nil as —, nested structs as
// {…} and slices or maps as [n].
func summarize(item any) [][2]string {
	v := reflect.ValueOf(item)
	for v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return [][2]string{{"value", fmt.Sprint(v.Interface())}}
	}
	t := v.Type()
	var out [][2]string
	for i := range t.NumField() {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		name := f.Name
		if tag, _, _ := strings.Cut(f.Tag.Get("json"), ","); tag == "-" {
			continue
		} else if tag != "" {
			name = tag
		}
		if f.Anonymous && f.Type.Kind() == reflect.Struct {
			out = append(out, summarize(v.Field(i).Interface())...)
			continue
		}
		out = append(out, [2]string{name, summaryValue(v.Field(i))})
	}
	return out
}

// summaryValue is one field's condensed value.
func summaryValue(v reflect.Value) string {
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		if v.IsNil() {
			return "—"
		}
		return summaryValue(v.Elem())
	case reflect.Struct:
		if s, ok := v.Interface().(fmt.Stringer); ok {
			return s.String()
		}
		return "{…}"
	case reflect.Slice, reflect.Array, reflect.Map:
		return fmt.Sprintf("[%d]", v.Len())
	case reflect.String:
		if v.String() == "" {
			return "—"
		}
		return v.String()
	default:
		return fmt.Sprint(v.Interface())
	}
}
