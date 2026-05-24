package columns

import (
	"strings"

	"github.com/jingle2008/toolkit/internal/domain"
)

// AliasColumns is the canonical column set for domain.Alias.
// Canonical follows the TUI's 1-row-per-category shape (intentional
// behaviour change from the CLI's 1-row-per-alias layout).
//
// TODO(Task 7): the current CLI writeAliases in internal/cli/get.go
// builds []aliasItem{Alias, Category} (one row per alias). Task 7
// must rewrite it to pass []domain.Category so RenderTable's type
// assertion (`items.([]domain.Category)`) succeeds. Wrong-typed
// payload at runtime would produce: "renderFlat: items has wrong
// type []cli.aliasItem".
var AliasColumns = Set[domain.Category]{Columns: []Column[domain.Category]{
	{Title: "Name", Key: "name", Default: true, Ratio: 0.40,
		Render: func(c domain.Category) string { return c.String() }},
	{Title: "Aliases", Key: "aliases", Default: true, Ratio: 0.60,
		Render: func(c domain.Category) string { return strings.Join(c.GetAliases(), ", ") }},
}}
