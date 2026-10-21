package tui

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/table"
	"github.com/charmbracelet/x/ansi"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
)

// statusTone names the role a Status value is drawn in.
type statusTone int

const (
	tonePlain statusTone = iota
	toneDanger
	toneWarn
	toneSuccess
)

// statusTones maps every value a Status column produces, lowercased, to
// its tone. A value not listed stays plain, so a new state is never
// colored by accident. Sources:
//   - Tenant: account state (ACTIVE, DELETING, TERMINATED, ALWAYS_FREE_ONLY)
//   - DAC: the CR's status (Active, Failed), pending when unset
//   - an action running on a row (action tracker): Deleting, Terminating,
//     Restarting, Draining, Scaling
//   - BaseModel, ImportedModel: CR state (Ready, Creating, Unknown)
//   - GPU pool: the OCI instance-pool lifecycle state, plus UNKNOWN,
//     NONEXIST and "..." (not yet enriched)
//   - ServingRuntime: live, off, disabled
//
// GPU node statuses ("WARN: CORDONED", "ERROR: Not ready", ...) are
// matched by prefix in toneOf.
var statusTones = map[string]statusTone{
	"failed":           toneDanger,
	"terminated":       toneDanger,
	"deleting":         toneDanger,
	"nonexist":         toneDanger,
	"pending":          toneWarn,
	"creating":         toneWarn,
	"provisioning":     toneWarn,
	"scaling":          toneWarn,
	"starting":         toneWarn,
	"stopping":         toneWarn,
	"terminating":      toneWarn,
	"restarting":       toneWarn,
	"draining":         toneWarn,
	"unknown":          toneWarn,
	"...":              toneWarn,
	"active":           toneSuccess,
	"ready":            toneSuccess,
	"running":          toneSuccess,
	"ok":               toneSuccess,
	"live":             toneSuccess,
	"stopped":          tonePlain,
	"off":              tonePlain,
	"disabled":         tonePlain,
	"always_free_only": tonePlain,
}

// toneOf returns the tone of a Status value: GPU node "ERROR:"/"WARN:"
// prefixes first, then the exact value.
func toneOf(cell string) statusTone {
	s := strings.ToLower(strings.TrimSpace(cell))
	switch {
	case strings.HasPrefix(s, "error:"):
		return toneDanger
	case strings.HasPrefix(s, "warn:"):
		return toneWarn
	}
	return statusTones[s]
}

// color returns the palette role for t, or nil for plain.
func (t statusTone) color(p Palette) color.Color {
	switch t {
	case toneDanger:
		return p.Danger
	case toneWarn:
		return p.Warn
	case toneSuccess:
		return p.Success
	case tonePlain:
	}
	return nil
}

// toneStatusCells colors the Status column of rows by meaning. It sets
// and resets only the foreground, so the selected row's fill carries on
// across the cell. rows are display rows: m.rawRows keeps the plain text
// for filtering, copying and identity.
func (m *Model) toneStatusCells(rows []table.Row) {
	col := -1
	for i, h := range m.headers {
		if strings.EqualFold(h.text, common.StatusCol) {
			col = i
			break
		}
	}
	if col < 0 {
		return
	}
	for _, r := range rows {
		if col >= len(r) {
			continue
		}
		if c := toneOf(r[col]).color(m.theme.Palette); c != nil {
			r[col] = fgSeq(c) + r[col] + "\x1b[39m"
		}
	}
}

// toneOverSizePools draws a GPU pool's Actual Size in Warn when it is
// past the Terraform Size: a deliberate over-scale, which isn't faulty
// but is drift the next terraform apply undoes (models.GPUPool.IsOverSize).
// Like toneStatusCells it sets and resets only the foreground.
func (m *Model) toneOverSizePools(rows []table.Row) {
	if m.category != domain.GPUPool {
		return
	}
	size, actual := -1, -1
	for i, h := range m.headers {
		switch h.text {
		case common.SizeCol:
			size = i
		case common.ActualSizeCol:
			actual = i
		}
	}
	if size < 0 || actual < 0 {
		return
	}
	for _, r := range rows {
		if size >= len(r) || actual >= len(r) {
			continue
		}
		declared, err1 := strconv.Atoi(strings.TrimSpace(r[size]))
		live, err2 := strconv.Atoi(strings.TrimSpace(r[actual]))
		if err1 == nil && err2 == nil && live > declared {
			r[actual] = fgSeq(m.theme.Palette.Warn) + r[actual] + "\x1b[39m"
		}
	}
}

// markEmptyCells draws empty cells as a muted "—", so a blank reads as
// "no value" rather than a rendering gap. Like toneStatusCells it sets
// and resets only the foreground.
func (m *Model) markEmptyCells(rows []table.Row) {
	dash := "—"
	if c := m.theme.Palette.Muted; c != nil {
		dash = fgSeq(c) + dash + "\x1b[39m"
	}
	for _, r := range rows {
		for i, cell := range r {
			if cell == "" {
				r[i] = dash
			}
		}
	}
}

// fgSeq is the SGR sequence that sets c as a 24-bit foreground.
func fgSeq(c color.Color) string {
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", r>>8, g>>8, b>>8)
}

// numericCols are the columns whose values are counts, sizes, ages or
// percentages: they are right-aligned, header included, so digits line
// up. Matched case-insensitively against the header text.
var numericCols = map[string]bool{
	"total": true, "free": true, "size": true, "actual size": true, "gpus": true,
	"age": true, "usage": true, "min": true, "max": true, "context": true,
	"restarts": true, "replicas": true,
}

// idCols are the columns that only identify a row (OCIDs): drawn Muted,
// so the data around them leads.
var idCols = map[string]bool{"ocid": true}

// isNumericCol reports whether the column titled h is right-aligned.
func isNumericCol(h string) bool { return numericCols[strings.ToLower(h)] }

// muteIDCells draws idCols cells in Muted, setting and resetting only the
// foreground like toneStatusCells.
func (m *Model) muteIDCells(rows []table.Row) {
	c := m.theme.Palette.Muted
	if c == nil {
		return
	}
	for i, h := range m.headers {
		if !idCols[strings.ToLower(h.text)] {
			continue
		}
		for _, r := range rows {
			if i < len(r) && r[i] != "" {
				r[i] = fgSeq(c) + r[i] + "\x1b[39m"
			}
		}
	}
}

// alignNumericCells right-aligns numericCols cells within their column
// width. It runs last, on cells already cut and colored.
func (m *Model) alignNumericCells(rows []table.Row) {
	cols := m.table.Columns()
	for i, h := range m.headers {
		if !isNumericCol(h.text) || i >= len(cols) {
			continue
		}
		w := cols[i].Width
		for _, r := range rows {
			if i < len(r) {
				r[i] = padLeft(r[i], w)
			}
		}
	}
}

// padLeft right-aligns s, which may carry ANSI styling, in w cells.
func padLeft(s string, w int) string {
	if n := ansi.StringWidth(s); n < w {
		return strings.Repeat(" ", w-n) + s
	}
	return s
}
