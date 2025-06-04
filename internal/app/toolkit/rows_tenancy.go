package toolkit

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/jingle2008/toolkit/pkg/models"
	"github.com/jingle2008/toolkit/pkg/utils"
	"go.uber.org/zap"
)

/*
limitTenancyOverrideRow is a wrapper to implement RowMarshaler for models.LimitTenancyOverride.
*/
type limitTenancyOverrideRow models.LimitTenancyOverride

func (l limitTenancyOverrideRow) ToRow(scope string) table.Row {
	return table.Row{
		scope,
		l.Name,
		strings.Join(l.Regions, ", "),
		fmt.Sprint(l.Values[0].Min),
		fmt.Sprint(l.Values[0].Max),
	}
}

/*
consolePropertyTenancyOverrideRow is a wrapper to implement RowMarshaler for models.ConsolePropertyTenancyOverride.
*/
type consolePropertyTenancyOverrideRow models.ConsolePropertyTenancyOverride

func (c consolePropertyTenancyOverrideRow) ToRow(scope string) table.Row {
	return table.Row{
		scope,
		c.Name,
		strings.Join(c.GetRegions(), ", "),
		c.GetValue(),
	}
}

/*
propertyTenancyOverrideRow is a wrapper to implement RowMarshaler for models.PropertyTenancyOverride.
*/
type propertyTenancyOverrideRow models.PropertyTenancyOverride

func (p propertyTenancyOverrideRow) ToRow(scope string) table.Row {
	return table.Row{
		scope,
		p.Name,
		strings.Join(p.GetRegions(), ", "),
		p.GetValue(),
	}
}

// (bulk *ToRows helpers removed as unused)

// getScopedItems is used for tenancy and other scoped overrides.
func getScopedItems[T models.NamedFilterable](logger *zap.Logger, g map[string][]T,
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
			return getTableRow(logger, s, v)
		})
}
