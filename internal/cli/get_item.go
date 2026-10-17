package cli

import (
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jingle2008/toolkit/internal/cli/output"
	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/pkg/models"
)

// itemArg returns `get <category> <name>`'s name, "" without one. A
// name switches the default output to yaml: one row reads best as an
// object.
func itemArg(cmd *cobra.Command, args []string, cat domain.Category, format *string) (string, error) {
	if len(args) < 2 {
		return "", nil
	}
	if cat == domain.Alias {
		return "", fmt.Errorf("`get alias` takes no name")
	}
	if !cmd.Flags().Changed("output") {
		*format = string(output.FormatYAML)
	}
	return args[1], nil
}

// itemName is the name `get <category> <name>` matches: the row's own
// name, the first column of the TUI's table and of `get`.
func itemName(it any) (string, bool) {
	n, ok := it.(models.NamedItem)
	if !ok {
		return "", false
	}
	return n.GetName(), true
}

// pickFlat finds the one row of a flat category named name, exactly.
func pickFlat[T any](cat domain.Category, items []T, name string) (T, error) {
	var found []T
	for _, it := range items {
		if n, ok := itemName(it); ok && n == name {
			found = append(found, it)
		}
	}
	var zero T
	switch len(found) {
	case 0:
		return zero, notFound(cat, name)
	case 1:
		return found[0], nil
	default:
		return zero, fmt.Errorf("%d %s rows are named %q", len(found), cat, name)
	}
}

// pickGrouped finds the one row of a grouped category named name, or
// <group>/<name> when the name alone is shared across groups (an
// override's name across tenants, an endpoint's across namespaces).
func pickGrouped[T any](cat domain.Category, grouped map[string][]T, name string) (string, T, error) {
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
		return "", zero, notFound(cat, name)
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
			len(found), cat, name, strings.Join(ids, ", "))
	}
}

func notFound(cat domain.Category, name string) error {
	return fmt.Errorf("no %s named %q (the name must match exactly; --filter matches loosely)", cat, name)
}

// writeOne writes the picked row: a single object for json, jsonl and
// yaml, else the one-row table table renders.
func writeOne(w writer, item any, opts output.Options, table func(output.Options) error) error {
	switch opts.Format {
	case output.FormatJSON, output.FormatJSONL, output.FormatYAML:
		return writeEncoded(w, opts, item)
	default:
		opts.Item = ""
		return table(opts)
	}
}
