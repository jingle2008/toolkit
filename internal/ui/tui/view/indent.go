package view

import "strings"

// IndentLines prefixes every line of s with pad.
func IndentLines(s, pad string) string {
	return pad + strings.ReplaceAll(s, "\n", "\n"+pad)
}
