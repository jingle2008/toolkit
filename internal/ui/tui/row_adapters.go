// Package toolkit: row_adapters.go
// Contains row adapter types, ToRow methods, GetTableRow, and GetScopedItems for UI table rendering.

package toolkit

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/pkg/models"
	"github.com/jingle2008/toolkit/pkg/utils"
	"go.uber.org/zap"
)

// LimitTenancyOverrideRow is a wrapper to implement RowMarshaler for models.LimitTenancyOverride.
type LimitTenancyOverrideRow models.LimitTenancyOverride

// ToRow returns a table.Row for the LimitTenancyOverrideRow, scoped by the given string.
func (l LimitTenancyOverrideRow) ToRow(scope string) table.Row {
	return table.Row{
		scope,
		l.Name,
		strings.Join(l.Regions, ", "),
		fmt.Sprint(l.Values[0].Min),
		fmt.Sprint(l.Values[0].Max),
	}
}

// ConsolePropertyTenancyOverrideRow is a wrapper to implement RowMarshaler for models.ConsolePropertyTenancyOverride.
type ConsolePropertyTenancyOverrideRow models.ConsolePropertyTenancyOverride

// ToRow returns a table.Row for the ConsolePropertyTenancyOverrideRow, scoped by the given string.
func (c ConsolePropertyTenancyOverrideRow) ToRow(scope string) table.Row {
	return table.Row{
		scope,
		c.Name,
		strings.Join(c.GetRegions(), ", "),
		c.GetValue(),
	}
}

// PropertyTenancyOverrideRow is a wrapper to implement RowMarshaler for models.PropertyTenancyOverride.
type PropertyTenancyOverrideRow models.PropertyTenancyOverride

// ToRow returns a table.Row for the PropertyTenancyOverrideRow, scoped by the given string.
func (p PropertyTenancyOverrideRow) ToRow(scope string) table.Row {
	return table.Row{
		scope,
		p.Name,
		strings.Join(p.GetRegions(), ", "),
		p.GetValue(),
	}
}

// GpuNodeRow is a wrapper to implement RowMarshaler for models.GpuNode.
type GpuNodeRow models.GpuNode

// ToRow returns a table.Row for the GpuNodeRow.
func (n GpuNodeRow) ToRow(_ string) table.Row {
	return table.Row{
		n.NodePool,
		n.Name,
		n.InstanceType,
		fmt.Sprint(n.Allocatable),
		fmt.Sprint(n.Allocatable - n.Allocated),
		fmt.Sprint(n.IsHealthy),
		fmt.Sprint(n.IsReady),
		models.GpuNode(n).GetStatus(),
	}
}

// DedicatedAIClusterRow is a wrapper to implement RowMarshaler for models.DedicatedAICluster.
type DedicatedAIClusterRow models.DedicatedAICluster

// ToRow returns a table.Row for the DedicatedAIClusterRow, scoped by the given string.
func (d DedicatedAIClusterRow) ToRow(scope string) table.Row {
	unitShapeOrProfile := d.UnitShape
	if unitShapeOrProfile == "" {
		unitShapeOrProfile = d.Profile
	}
	return table.Row{
		scope,
		d.Name,
		d.Type,
		unitShapeOrProfile,
		fmt.Sprint(d.Size),
		d.Status,
	}
}

// GetTableRow returns a table.Row for a given item, using the appropriate adapter function based on type.
func GetTableRow(logger Logger, tenant string, item interface{}) table.Row {
	switch val := item.(type) {
	case models.LimitTenancyOverride:
		return LimitTenancyOverrideRow(val).ToRow(tenant)
	case models.ConsolePropertyTenancyOverride:
		return ConsolePropertyTenancyOverrideRow(val).ToRow(tenant)
	case models.PropertyTenancyOverride:
		return PropertyTenancyOverrideRow(val).ToRow(tenant)
	case models.GpuNode:
		return GpuNodeRow(val).ToRow(tenant)
	case models.DedicatedAICluster:
		return DedicatedAIClusterRow(val).ToRow(tenant)
	default:
		if logger != nil {
			logger.Warn("unexpected type in GetTableRow",
				zap.String("type", fmt.Sprintf("%T", val)),
			)
		}
	}
	return nil
}

// GetScopedItems is used for tenancy and other scoped overrides.
// Accepts a Logger interface for decoupling from zap.
func GetScopedItems[T models.NamedFilterable](
	logger Logger,
	g map[string][]T,
	scopeCategory domain.Category,
	ctx *domain.ToolkitContext,
	filter string,
) []table.Row {
	var (
		key  *string
		name *string
	)

	if ctx != nil {
		if ctx.Category == scopeCategory {
			key = &ctx.Name
		} else {
			name = &ctx.Name
		}
	}

	return utils.FilterMap(g, key, name, filter,
		func(s string, v T) table.Row {
			return GetTableRow(logger, s, v)
		})
}
