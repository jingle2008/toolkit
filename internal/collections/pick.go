package collections

import (
	"fmt"
	"slices"
	"strings"

	"github.com/jingle2008/toolkit/pkg/models"
)

// NoSuchNameError is PickByName's and PickByNameGrouped's error when no
// row has the name, so callers can add how to search loosely instead.
type NoSuchNameError struct {
	What string // the rows' category, e.g. "GPUNode"
	Name string
}

func (e *NoSuchNameError) Error() string { return fmt.Sprintf("no %s named %q", e.What, e.Name) }

// itemName is the name PickByName matches: the row's own name, the first
// column of its table.
func itemName(it any) (string, bool) {
	n, ok := it.(models.NamedItem)
	if !ok {
		return "", false
	}
	return n.GetName(), true
}

// PickByName returns the one item of a flat category named name, exactly
// (case-sensitive). what names the rows in errors. It is how `toolkit get
// <category> <name>` and MCP's get_item find a row, so both agree. Rows
// that can share a name (models.QualifiedItem) also answer to their
// qualified name, and an ambiguous name lists each of those.
func PickByName[T any](items []T, name, what string) (T, error) {
	var found []T
	for _, it := range items {
		if n, ok := itemName(it); ok && (n == name || qualifiedName(it) == name) {
			found = append(found, it)
		}
	}
	var zero T
	switch len(found) {
	case 0:
		return zero, &NoSuchNameError{What: what, Name: name}
	case 1:
		return found[0], nil
	}
	ids := make([]string, 0, len(found))
	for _, it := range found {
		if q := qualifiedName(it); q != "" {
			ids = append(ids, q)
		}
	}
	if len(ids) < len(found) {
		return zero, fmt.Errorf("%d %s rows are named %q", len(found), what, name)
	}
	slices.Sort(ids)
	return zero, fmt.Errorf("%d %s rows are named %q: %s; name one as listed",
		len(found), what, name, strings.Join(ids, ", "))
}

// qualifiedName is a models.QualifiedItem's qualified name, or "".
func qualifiedName(it any) string {
	if q, ok := it.(models.QualifiedItem); ok {
		return q.QualifiedName()
	}
	return ""
}

// PickByNameGrouped is PickByName for a grouped category, returning the
// item's group too. A name rows in different groups share (an override's
// across tenants, an endpoint's across namespaces) is ambiguous: the
// error lists each as <group>/<name>, which picks one.
func PickByNameGrouped[T any](grouped map[string][]T, name, what string) (string, T, error) {
	type hit struct {
		group string
		item  T
	}
	var found []hit
	for group, items := range grouped {
		for _, it := range items {
			if n, ok := itemName(it); ok && (n == name || group+"/"+n == name) {
				found = append(found, hit{group, it})
			}
		}
	}
	var zero T
	switch len(found) {
	case 0:
		return "", zero, &NoSuchNameError{What: what, Name: name}
	case 1:
		return found[0].group, found[0].item, nil
	default:
		ids := make([]string, len(found))
		for i, h := range found {
			n, _ := itemName(h.item)
			ids[i] = h.group + "/" + n
		}
		slices.Sort(ids)
		return "", zero, fmt.Errorf("%d %s rows are named %q: %s; name one as <group>/<name>",
			len(found), what, name, strings.Join(ids, ", "))
	}
}
