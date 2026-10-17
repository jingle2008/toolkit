package tui

import tea "github.com/charmbracelet/bubbletea"

// namedKeys maps the key names tests press to their key types; anything
// else is typed as runes.
var namedKeys = map[string]tea.KeyType{
	"esc":       tea.KeyEsc,
	"enter":     tea.KeyEnter,
	"tab":       tea.KeyTab,
	"shift+tab": tea.KeyShiftTab,
	"up":        tea.KeyUp,
	"down":      tea.KeyDown,
	"left":      tea.KeyLeft,
	"right":     tea.KeyRight,
	"home":      tea.KeyHome,
	"end":       tea.KeyEnd,
	"pgup":      tea.KeyPgUp,
	"pgdown":    tea.KeyPgDown,
	"backspace": tea.KeyBackspace,
	"space":     tea.KeySpace,
	" ":         tea.KeySpace,
	"ctrl+a":    tea.KeyCtrlA,
	"ctrl+c":    tea.KeyCtrlC,
	"ctrl+o":    tea.KeyCtrlO,
	"ctrl+x":    tea.KeyCtrlX,
	"ctrl+z":    tea.KeyCtrlZ,
}

// keyPress builds the key event for s as the terminal would send it: a
// named key ("esc", "ctrl+c", "shift+tab", ...) or typed text ("y",
// "acme"). It is the only place tests build key events, so the bubbletea
// v2 key API change lands here alone.
func keyPress(s string) tea.KeyMsg {
	if t, ok := namedKeys[s]; ok {
		if t == tea.KeySpace {
			return tea.KeyMsg{Type: t, Runes: []rune{' '}}
		}
		return tea.KeyMsg{Type: t}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}
