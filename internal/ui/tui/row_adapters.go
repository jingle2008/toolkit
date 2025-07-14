// Package toolkit: row_adapters.go
// Contains row adapter types, ToRow methods, GetTableRow, and GetScopedItems for UI table rendering.

package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/jingle2008/toolkit/internal/collections"
	"github.com/jingle2008/toolkit/internal/domain"
	logging "github.com/jingle2008/toolkit/pkg/infra/logging"
	"github.com/jingle2008/toolkit/pkg/models"
)

// LimitTenancyOverrideRow adapts models.LimitTenancyOverride for table rendering.
type LimitTenancyOverrideRow models.LimitTenancyOverride

func (l LimitTenancyOverrideRow) ToRow(scope string) table.Row {
	return table.Row([]string{
		l.Name,
		scope,
		strings.Join(l.Regions, ", "),
		fmt.Sprint(l.Values[0].Min),
		fmt.Sprint(l.Values[0].Max),
	})
}

// ConsolePropertyTenancyOverrideRow is a wrapper to implement RowMarshaler for models.ConsolePropertyTenancyOverride.
type ConsolePropertyTenancyOverrideRow models.ConsolePropertyTenancyOverride

func (c ConsolePropertyTenancyOverrideRow) ToRow(scope string) table.Row {
	return table.Row([]string{
		c.Name,
		scope,
		strings.Join(c.GetRegions(), ", "),
		c.GetValue(),
	})
}

// PropertyTenancyOverrideRow is a wrapper to implement RowMarshaler for models.PropertyTenancyOverride.
type PropertyTenancyOverrideRow models.PropertyTenancyOverride

func (p PropertyTenancyOverrideRow) ToRow(scope string) table.Row {
	return table.Row([]string{
		p.Name,
		scope,
		strings.Join(p.GetRegions(), ", "),
		p.GetValue(),
	})
}

// GpuNodeRow is a wrapper to implement RowMarshaler for models.GpuNode.
type GpuNodeRow models.GpuNode

func (n GpuNodeRow) ToRow(scope string) table.Row {
	return table.Row([]string{
		n.Name,
		n.NodePool,
		n.InstanceType,
		fmt.Sprint(n.Allocatable),
		fmt.Sprint(n.Allocatable - n.Allocated),
		fmt.Sprint(n.IsHealthy),
		fmt.Sprint(n.IsReady),
		n.Age,
		models.GpuNode(n).GetStatus(),
	})
}

// DedicatedAIClusterRow is a wrapper to implement RowMarshaler for models.DedicatedAICluster.
type DedicatedAIClusterRow models.DedicatedAICluster

func (d DedicatedAIClusterRow) ToRow(scope string) table.Row {
	unitShapeOrProfile := d.UnitShape
	if unitShapeOrProfile == "" {
		unitShapeOrProfile = d.Profile
	}
	return table.Row([]string{
		d.Name,
		scope,
		models.DedicatedAICluster(d).GetOwnerState(),
		models.DedicatedAICluster(d).GetUsage(),
		d.Type,
		unitShapeOrProfile,
		fmt.Sprint(d.Size),
		d.Age,
		d.Status,
	})
}

// TenantRow is a wrapper to implement RowMarshaler for models.Tenant.
type TenantRow models.Tenant

func (t TenantRow) ToRow(scope string) table.Row {
	return table.Row([]string{
		t.Name,
		models.Tenant(t).GetTenantID(),
		fmt.Sprint(t.IsInternal),
		t.Note,
	})
}

// ServiceTenancyRow is a wrapper to implement RowMarshaler for models.ServiceTenancy.
type ServiceTenancyRow models.ServiceTenancy

func (s ServiceTenancyRow) ToRow(scope string) table.Row {
	return table.Row([]string{
		s.Name,
		s.Realm,
		s.Environment,
		s.HomeRegion,
		strings.Join(s.Regions, ", "),
	})
}

// EnvironmentRow is a wrapper to implement RowMarshaler for models.Environment.
type EnvironmentRow models.Environment

func (e EnvironmentRow) ToRow(scope string) table.Row {
	return table.Row([]string{
		models.Environment(e).GetName(),
		e.Realm,
		e.Type,
		e.Region,
	})
}

// CategoryRow adapts alias information for table rendering.
type CategoryRow domain.Category

func (r CategoryRow) ToRow(scope string) table.Row {
	c := domain.Category(r)
	return table.Row([]string{c.String(), strings.Join(c.GetAliases(), ", ")})
}

// ModelArtifactRow adapts models.ModelArtifact for table rendering.
type ModelArtifactRow models.ModelArtifact

func (val ModelArtifactRow) ToRow(scope string) table.Row {
	return table.Row([]string{
		val.Name,
		val.ModelName,
		models.ModelArtifact(val).GetGpuConfig(),
		val.TensorRTVersion,
	})
}

/*
GetTableRow returns a table.Row for a given item.
If the item implements RowMarshaler, it is used directly.
Otherwise, falls back to legacy type switch for backward compatibility.
*/
func GetTableRow(logger logging.Logger, scope string, item any) table.Row {
	switch val := item.(type) {
	case models.LimitTenancyOverride:
		return LimitTenancyOverrideRow(val).ToRow(scope)
	case models.ConsolePropertyTenancyOverride:
		return ConsolePropertyTenancyOverrideRow(val).ToRow(scope)
	case models.PropertyTenancyOverride:
		return PropertyTenancyOverrideRow(val).ToRow(scope)
	case models.GpuNode:
		return GpuNodeRow(val).ToRow(scope)
	case models.DedicatedAICluster:
		return DedicatedAIClusterRow(val).ToRow(scope)
	case models.ModelArtifact:
		return ModelArtifactRow(val).ToRow(scope)
	default:
		if logger != nil {
			logger.Errorw("unexpected type in GetTableRow", "type", fmt.Sprintf("%T", val))
		}
	}
	return nil
}

// GetScopedItems is used for tenancy and other scoped overrides.
// Accepts a Logger interface for decoupling from zap.
func GetScopedItems[T models.NamedFilterable](
	logger logging.Logger,
	g map[string][]T,
	scopeCategory domain.Category,
	ctx *domain.ToolkitContext,
	filter string,
	pred func(T) bool,
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

	return collections.FilterMap(g, key, name, filter, pred,
		func(s string, v T) table.Row {
			return GetTableRow(logger, s, v)
		})
}
