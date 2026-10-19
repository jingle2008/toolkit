package tui

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/bubbles/v2/table"

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
