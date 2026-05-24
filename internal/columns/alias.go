package columns

import (
	"strings"

	"github.com/jingle2008/toolkit/internal/domain"
)

// AliasColumns is the canonical column set for domain.Alias.
// Canonical follows the TUI's 1-row-per-category shape (intentional
// behaviour change from the CLI's 1-row-per-alias layout).
var AliasColumns = Set[domain.Category]{Columns: []Column[domain.Category]{
	{Title: "Name", Key: "name", Ratio: 0.40,
		Render: func(c domain.Category) string { return c.String() }},
	{Title: "Aliases", Key: "aliases", Ratio: 0.60,
		Render: func(c domain.Category) string { return strings.Join(c.GetAliases(), ", ") }},
}}
