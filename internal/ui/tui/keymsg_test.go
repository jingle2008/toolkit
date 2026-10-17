package tui

import tea "charm.land/bubbletea/v2"

// namedKeys maps the key names tests press to their key events; anything
// else is typed as text.
var namedKeys = map[string]tea.Key{
	"esc":       {Code: tea.KeyEscape},
	"enter":     {Code: tea.KeyEnter},
	"tab":       {Code: tea.KeyTab},
	"shift+tab": {Code: tea.KeyTab, Mod: tea.ModShift},
	"up":        {Code: tea.KeyUp},
	"down":      {Code: tea.KeyDown},
	"left":      {Code: tea.KeyLeft},
	"right":     {Code: tea.KeyRight},
	"home":      {Code: tea.KeyHome},
	"end":       {Code: tea.KeyEnd},
	"pgup":      {Code: tea.KeyPgUp},
	"pgdown":    {Code: tea.KeyPgDown},
	"backspace": {Code: tea.KeyBackspace},
	"space":     {Code: tea.KeySpace, Text: " "},
	" ":         {Code: tea.KeySpace, Text: " "},
	"ctrl+a":    {Code: 'a', Mod: tea.ModCtrl},
	"ctrl+c":    {Code: 'c', Mod: tea.ModCtrl},
	"ctrl+o":    {Code: 'o', Mod: tea.ModCtrl},
	"ctrl+x":    {Code: 'x', Mod: tea.ModCtrl},
	"ctrl+z":    {Code: 'z', Mod: tea.ModCtrl},
}

// keyPress builds the key event for s as the terminal would send it: a
// named key ("esc", "ctrl+c", "shift+tab", ...) or typed text ("y",
// "acme"). It is the only place tests build key events.
func keyPress(s string) tea.KeyMsg {
	if k, ok := namedKeys[s]; ok {
		return tea.KeyPressMsg(k)
	}
	return tea.KeyPressMsg{Code: []rune(s)[0], Text: s}
}
