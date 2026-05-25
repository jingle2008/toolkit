package columns

import (
	"strings"

	"github.com/jingle2008/toolkit/pkg/models"
)

// TenancyOverrideColumns is parameterized by the concrete
// DefinitionOverride type (ConsolePropertyTenancyOverride or
// PropertyTenancyOverride); both satisfy models.DefinitionOverride.
//
// CLI defaults widen vs today (was TENANT|NAME only) to match the
// TUI — Name, Tenant, Regions, Value all Default==true. Per spec
// Decision #9.
func TenancyOverrideColumns[T models.DefinitionOverride]() GroupedSet[T] {
	return GroupedSet[T]{Columns: []GroupedColumn[T]{
		{
			Title: "Name", Key: "name", Ratio: 0.40,
			Render: func(_ string, v T) string { return v.GetName() },
		},
		{
			Title: "Tenant", Key: "tenant", Ratio: 0.25, TruncateMiddle: true,
			Render: func(k string, _ T) string { return k },
		},
		{
			Title: "Regions", Key: "regions", Ratio: 0.25,
			Render: func(_ string, v T) string { return strings.Join(v.GetRegions(), ", ") },
		},
		{
			Title: "Value", Key: "value", Ratio: 0.10,
			Render: func(_ string, v T) string { return v.GetValue() },
		},
	}}
}

var (
	ConsolePropertyTenancyOverrideColumns = TenancyOverrideColumns[models.ConsolePropertyTenancyOverride]()
	PropertyTenancyOverrideColumns        = TenancyOverrideColumns[models.PropertyTenancyOverride]()
)
