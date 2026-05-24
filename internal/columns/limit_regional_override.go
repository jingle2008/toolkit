package columns

import (
	"strings"

	"github.com/jingle2008/toolkit/pkg/models"
)

// LimitRegionalOverrideColumns is the canonical column set for
// domain.LimitRegionalOverride. Default columns match today's CLI
// (NAME, REGIONS only). Min/Max have Default==false so they are
// opt-in via --columns, matching the TUI-only extra columns.
var LimitRegionalOverrideColumns = Set[models.LimitRegionalOverride]{Columns: []Column[models.LimitRegionalOverride]{
	{Title: "Name", Key: "name", Ratio: 0.40,
		Render: func(o models.LimitRegionalOverride) string { return o.Name }},
	{Title: "Regions", Key: "regions", Ratio: 0.30,
		Render: func(o models.LimitRegionalOverride) string { return strings.Join(o.Regions, ", ") }},
	{Title: "Min", Key: "min", Ratio: 0.15,
		Render: func(o models.LimitRegionalOverride) string { return limitOverrideMin(o.Values) }},
	{Title: "Max", Key: "max", Ratio: 0.15,
		Render: func(o models.LimitRegionalOverride) string { return limitOverrideMax(o.Values) }},
}}
