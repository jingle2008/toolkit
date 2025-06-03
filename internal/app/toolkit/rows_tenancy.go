package toolkit

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/jingle2008/toolkit/pkg/models"
	"github.com/jingle2008/toolkit/pkg/utils"
)

// Adapter function for models.LimitTenancyOverride
func LimitTenancyOverrideToRow(scope string, l models.LimitTenancyOverride) table.Row {
	return table.Row{
		scope,
		l.Name,
		strings.Join(l.Regions, ", "),
		fmt.Sprint(l.Values[0].Min),
		fmt.Sprint(l.Values[0].Max),
	}
}

// Adapter function for models.ConsolePropertyTenancyOverride
func ConsolePropertyTenancyOverrideToRow(scope string, c models.ConsolePropertyTenancyOverride) table.Row {
	return table.Row{
		scope,
		c.Name,
		strings.Join(c.GetRegions(), ", "),
		c.GetValue(),
	}
}

// Adapter function for models.PropertyTenancyOverride
func PropertyTenancyOverrideToRow(scope string, p models.PropertyTenancyOverride) table.Row {
	return table.Row{
		scope,
		p.Name,
		strings.Join(p.GetRegions(), ", "),
		p.GetValue(),
	}
}

// getScopedItems is used for tenancy and other scoped overrides.
func getScopedItems[T models.NamedFilterable](g map[string][]T,
	scopeCategory Category, context *AppContext, filter string,
) []table.Row {
	var (
		key  *string
		name *string
	)

	if context != nil {
		if context.Category == scopeCategory {
			key = &context.Name
		} else {
			name = &context.Name
		}
	}

	return utils.FilterMap(g, key, name, filter,
		func(s string, v T) table.Row {
			return getTableRow(s, v)
		})
}
