package tui

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/bubbles/v2/table"

	"github.com/jingle2008/toolkit/internal/ui/tui/common"
)

// statusTones maps words found in a Status cell to the role it is drawn
// in, checked in order so the worst state wins ("WARN: CORDONED" is a
// warning, "Failed" danger). Matching is case-insensitive on substrings.
var statusTones = []struct {
	words []string
	role  func(Palette) color.Color
}{
	{[]string{"fail", "error", "delet", "terminat", "notready", "not ready"}, func(p Palette) color.Color { return p.Danger }},
	{[]string{"warn", "pending", "drain", "cordon", "creat", "updat", "scal", "unknown"}, func(p Palette) color.Color { return p.Warn }},
	{[]string{"active", "ok", "ready", "running", "succeed", "healthy"}, func(p Palette) color.Color { return p.Success }},
}

// statusTone returns the color for a Status cell, or nil to leave it plain.
func statusTone(p Palette, cell string) color.Color {
	s := strings.ToLower(cell)
	for _, t := range statusTones {
		for _, w := range t.words {
			if strings.Contains(s, w) {
				return t.role(p)
			}
		}
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
		if c := statusTone(m.theme.Palette, r[col]); c != nil {
			r[col] = fgSeq(c) + r[col] + "\x1b[39m"
		}
	}
}

// fgSeq is the SGR sequence that sets c as a 24-bit foreground.
func fgSeq(c color.Color) string {
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", r>>8, g>>8, b>>8)
}
