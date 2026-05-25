package columns

import (
	"strings"

	"github.com/jingle2008/toolkit/pkg/models"
)

// LimitTenancyOverrideColumns is the canonical column set for
// domain.LimitTenancyOverride. CLI defaults widen vs today (was
// TENANT|NAME only) to match the TUI — Name, Tenant, Regions,
// Min, Max all Default==true. Per spec Decision #9.
var LimitTenancyOverrideColumns = GroupedSet[models.LimitTenancyOverride]{Columns: []GroupedColumn[models.LimitTenancyOverride]{
	{Title: "Name", Key: "name", Ratio: 0.40,
		Render: func(_ string, v models.LimitTenancyOverride) string { return v.Name }},
	{Title: "Tenant", Key: "tenant", Ratio: 0.24, TruncateMiddle: true,
		Render: func(k string, _ models.LimitTenancyOverride) string { return k }},
	{Title: "Regions", Key: "regions", Ratio: 0.20,
		Render: func(_ string, v models.LimitTenancyOverride) string { return strings.Join(v.Regions, ", ") }},
	{Title: "Min", Key: "min", Ratio: 0.08,
		Render: func(_ string, v models.LimitTenancyOverride) string { return limitOverrideMin(v.Values) }},
	{Title: "Max", Key: "max", Ratio: 0.08,
		Render: func(_ string, v models.LimitTenancyOverride) string { return limitOverrideMax(v.Values) }},
}}
