package tui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"unicode"

	"charm.land/lipgloss/v2"
	"github.com/mattn/go-runewidth"
)

// node is one value of an item's JSON, kept in document order: an object
// (fields), an array (items) or a scalar (text).
type node struct {
	fields []field
	items  []node
	text   string
	kind   nodeKind
}

type field struct {
	key string
	val node
}

type nodeKind int

const (
	scalarNode nodeKind = iota
	objectNode
	arrayNode
)

// parseFields decodes an item's JSON (as jsonutil.Pretty writes it) into
// a node tree, keeping the field order the JSON has.
func parseFields(data []byte) (node, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	return decodeNode(dec)
}

func decodeNode(dec *json.Decoder) (node, error) {
	tok, err := dec.Token()
	if err != nil {
		return node{}, err
	}
	switch t := tok.(type) {
	case json.Delim:
		return decodeContainer(dec, t)
	case nil:
		return node{text: "—"}, nil
	case string:
		if t == "" {
			return node{text: "—"}, nil
		}
		return node{text: t}, nil
	default:
		return node{text: fmt.Sprint(t)}, nil
	}
}

// decodeContainer reads an object or array whose opening delimiter open
// was just read, through its closing one.
func decodeContainer(dec *json.Decoder, open json.Delim) (node, error) {
	n := node{kind: objectNode}
	if open == '[' {
		n.kind = arrayNode
	}
	for dec.More() {
		var k string
		if n.kind == objectNode {
			tok, err := dec.Token()
			if err != nil {
				return node{}, err
			}
			k = fmt.Sprint(tok)
		}
		v, err := decodeNode(dec)
		if err != nil {
			return node{}, err
		}
		if n.kind == objectNode {
			n.fields = append(n.fields, field{key: k, val: v})
		} else {
			n.items = append(n.items, v)
		}
	}
	_, err := dec.Token() // the closing delimiter
	return n, err
}

// scalarItems is an array's elements' text, or false when any element is
// an object or array.
func (n node) scalarItems() ([]string, bool) {
	parts := make([]string, 0, len(n.items))
	for _, it := range n.items {
		if it.kind != scalarNode {
			return nil, false
		}
		parts = append(parts, it.text)
	}
	return parts, true
}

// scalarList is an array of scalars joined as one value ("a, b, c"), or
// false when any element is an object or array.
func (n node) scalarList() (string, bool) {
	parts, ok := n.scalarItems()
	if !ok {
		return "", false
	}
	if len(parts) == 0 {
		return "—", true
	}
	return strings.Join(parts, ", "), true
}

// fieldStyles colors a fields layout: labels muted, section titles in
// the primary color.
type fieldStyles struct {
	label, section lipgloss.Style
}

/*
renderFields lays out an object as label/value rows, width columns wide:
its plain values first, then its nested objects and arrays of objects as
titled, indented sections, to depth levels (0 is unlimited); deeper ones
collapse to {…} and [n]. Arrays of scalars join into one value, or list
one bulleted item per row when joined they would be ambiguous (an item
holds ", ") or overflow the value column. Long values are cut at width,
or wrapped when wrap is set. sep puts a blank
line before each top-level section.
*/
func renderFields(n node, width, depth int, wrap, sep bool, st fieldStyles) string {
	r := fieldRenderer{width: width, depth: depth, wrap: wrap, sep: sep, st: st}
	r.object(n, 0, 0)
	return strings.Join(r.out, "\n")
}

type fieldRenderer struct {
	width, depth int
	wrap, sep    bool
	st           fieldStyles
	out          []string
}

// opens reports whether v is drawn as a section at level: a non-empty
// object or array of objects within the depth limit.
func (r *fieldRenderer) opens(v node, level int) bool {
	if _, isList := v.scalarList(); v.kind == scalarNode || (v.kind == arrayNode && isList) {
		return false
	}
	return (r.depth == 0 || level < r.depth) && (len(v.fields) > 0 || len(v.items) > 0)
}

func (r *fieldRenderer) object(n node, indent, level int) {
	keyW := 0
	for _, f := range n.fields {
		if !r.opens(f.val, level) {
			keyW = max(keyW, runewidth.StringWidth(humanize(f.key)))
		}
	}
	keyW = min(keyW, max((r.width-indent)/2, 1))
	pad := strings.Repeat(" ", indent)
	for _, f := range n.fields {
		if r.opens(f.val, level) {
			continue
		}
		if items, ok := stacked(f.val, max(r.width-indent-keyW-2, 1)); ok {
			r.out = append(r.out, fieldListRows(pad, humanize(f.key), items, keyW, r.width-indent, r.wrap, r.st)...)
			continue
		}
		r.out = append(r.out, fieldRows(pad, humanize(f.key), collapsed(f.val), keyW, r.width-indent, r.wrap, r.st)...)
	}
	for _, f := range n.fields {
		if r.opens(f.val, level) {
			r.section(humanize(f.key), f.val, indent, level)
		}
	}
}

// section draws a nested object, or each object of an array, under its
// title.
func (r *fieldRenderer) section(label string, v node, indent, level int) {
	if r.sep && level == 0 {
		r.out = append(r.out, "")
	}
	pad := strings.Repeat(" ", indent)
	if v.kind == arrayNode {
		label = fmt.Sprintf("%s (%d)", label, len(v.items))
	}
	r.out = append(r.out, pad+r.st.section.Render(truncateString(label, max(r.width-indent, 1))))
	if v.kind == objectNode {
		r.object(v, indent+2, level+1)
		return
	}
	for i, it := range v.items {
		r.out = append(r.out, pad+"  "+r.st.label.Render(fmt.Sprintf("#%d", i+1)))
		r.object(it, indent+4, level+1)
	}
}

// collapsed is the one-line value of a field not drawn as a section.
func collapsed(v node) string {
	if list, ok := v.scalarList(); v.kind == arrayNode && ok {
		return list
	}
	switch v.kind {
	case objectNode:
		if len(v.fields) == 0 {
			return "—"
		}
		return "{…}"
	case arrayNode:
		return fmt.Sprintf("[%d]", len(v.items))
	case scalarNode:
	}
	return v.text
}

// stacked is v's items when v is an array of two or more scalars that
// reads badly joined into one value: an item holds ", " (so the join
// can't be split back apart), or the join is wider than valW and would
// wrap.
func stacked(v node, valW int) ([]string, bool) {
	if v.kind != arrayNode {
		return nil, false
	}
	items, ok := v.scalarItems()
	if !ok || len(items) < 2 {
		return nil, false
	}
	for _, it := range items {
		if strings.Contains(it, ", ") {
			return items, true
		}
	}
	return items, runewidth.StringWidth(strings.Join(items, ", ")) > valW
}

// fieldRows renders one label/value row: the value cut to fit, or
// wrapped under itself when wrap is set.
func fieldRows(pad, label, value string, keyW, width int, wrap bool, st fieldStyles) []string {
	labelCell := st.label.Width(keyW + 2).Render(truncateString(label, keyW))
	lines := valueLines(value, max(width-keyW-2, 1), wrap)
	cont := pad + strings.Repeat(" ", keyW+2)
	rows := make([]string, len(lines))
	for i, l := range lines {
		rows[i] = cont + l
	}
	rows[0] = pad + labelCell + lines[0]
	return rows
}

// fieldListRows renders a label with items under it, one per row, each
// bulleted and, when wrap is set, wrapped under its own text.
func fieldListRows(pad, label string, items []string, keyW, width int, wrap bool, st fieldStyles) []string {
	labelCell := st.label.Width(keyW + 2).Render(truncateString(label, keyW))
	cont := pad + strings.Repeat(" ", keyW+2)
	itemW := max(width-keyW-4, 1)
	var rows []string
	for _, it := range items {
		for j, l := range valueLines(it, itemW, wrap) {
			lead := cont
			if len(rows) == 0 {
				lead = pad + labelCell
			}
			mark := "  "
			if j == 0 {
				mark = "• "
			}
			rows = append(rows, lead+mark+l)
		}
	}
	return rows
}

// valueLines is value cut to valW in the middle (so its end, often what
// tells values apart, stays), or wrapped to it when wrap is set.
func valueLines(value string, valW int, wrap bool) []string {
	if !wrap {
		return []string{truncateMiddle(value, valW)}
	}
	lines := strings.Split(lipgloss.NewStyle().Width(valW).Render(value), "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " ")
	}
	return lines
}

// labelAcronyms are the words humanize writes in capitals.
var labelAcronyms = map[string]string{
	"id": "ID", "ids": "IDs", "ocid": "OCID", "url": "URL", "api": "API",
	"gpu": "GPU", "gpus": "GPUs", "cpu": "CPU", "dac": "DAC", "ip": "IP",
	"oci": "OCI", "oke": "OKE", "csv": "CSV", "k8s": "k8s",
}

// humanize turns a JSON key into a label: "totalReplicas" and
// "is_internal" become "Total replicas" and "Is internal", with known
// acronyms in capitals ("tenantId" → "Tenant ID") and capitalized runs
// kept ("HTTPServerPort" → "HTTP server port").
func humanize(key string) string {
	words := splitWords(key)
	for i, w := range words {
		words[i] = labelWord(w, i == 0)
	}
	return strings.Join(words, " ")
}

// splitWords splits a key at separators (_ - . space) and case changes:
// "tenantId" → tenant, Id; "HTTPServer" → HTTP, Server.
func splitWords(key string) []string {
	var words []string
	var cur []rune
	flush := func() {
		if len(cur) > 0 {
			words = append(words, string(cur))
			cur = cur[:0]
		}
	}
	runes := []rune(key)
	for i, r := range runes {
		if strings.ContainsRune("_-. ", r) {
			flush()
			continue
		}
		if unicode.IsUpper(r) && len(cur) > 0 {
			prev := cur[len(cur)-1]
			nextLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
			if unicode.IsLower(prev) || unicode.IsDigit(prev) || nextLower {
				flush()
			}
		}
		cur = append(cur, r)
	}
	flush()
	return words
}

// labelWord writes one word of a label: a known acronym in capitals, a
// capitalized run as it is, the first word capitalized, the rest lower.
func labelWord(w string, first bool) string {
	lw := strings.ToLower(w)
	switch {
	case labelAcronyms[lw] != "":
		return labelAcronyms[lw]
	case len(w) > 1 && w == strings.ToUpper(w):
		return w // an acronym run the key already capitalizes ("HTTPServer")
	case first:
		r := []rune(lw)
		return string(unicode.ToUpper(r[0])) + string(r[1:])
	}
	return lw
}
