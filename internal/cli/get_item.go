package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/jingle2008/toolkit/internal/collections"
	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/output"
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

// pickFlat finds the one row of a flat category named name, exactly.
func pickFlat[T any](cat domain.Category, items []T, name string) (T, error) {
	it, err := collections.PickByName(items, name, cat.String())
	return it, withNameHint(err)
}

// pickGrouped finds the one row of a grouped category named name, or
// <group>/<name> when the name alone is shared across groups.
func pickGrouped[T any](cat domain.Category, grouped map[string][]T, name string) (string, T, error) {
	group, it, err := collections.PickByNameGrouped(grouped, name, cat.String())
	return group, it, withNameHint(err)
}

// withNameHint adds, to a no-such-name error, how to search loosely.
func withNameHint(err error) error {
	if _, ok := errors.AsType[*collections.NoSuchNameError](err); ok {
		return fmt.Errorf("%w (the name must match exactly; --filter matches loosely)", err)
	}
	return err
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
