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

// Renderer is a UI-agnostic interface for rendering a row as a slice of strings.
type Renderer interface {
	Render(scope string) []string
}

// LimitTenancyOverrideRow adapts models.LimitTenancyOverride for table rendering.
type LimitTenancyOverrideRow models.LimitTenancyOverride

// Render implements the Renderer interface for LimitTenancyOverrideRow.
func (l LimitTenancyOverrideRow) Render(scope string) []string {
	return []string{
		scope,
		l.Name,
		strings.Join(l.Regions, ", "),
		fmt.Sprint(l.Values[0].Min),
		fmt.Sprint(l.Values[0].Max),
	}
}

// ToRow returns a table.Row for the LimitTenancyOverrideRow, scoped by the given string.
func (l LimitTenancyOverrideRow) ToRow(scope string) table.Row {
	return table.Row(l.Render(scope))
}

// ConsolePropertyTenancyOverrideRow is a wrapper to implement RowMarshaler for models.ConsolePropertyTenancyOverride.
type ConsolePropertyTenancyOverrideRow models.ConsolePropertyTenancyOverride

// Render implements the Renderer interface for ConsolePropertyTenancyOverrideRow.
func (c ConsolePropertyTenancyOverrideRow) Render(scope string) []string {
	return []string{
		scope,
		c.Name,
		strings.Join(c.GetRegions(), ", "),
		c.GetValue(),
	}
}

// ToRow returns a table.Row for the ConsolePropertyTenancyOverrideRow, scoped by the given string.
func (c ConsolePropertyTenancyOverrideRow) ToRow(scope string) table.Row {
	return table.Row(c.Render(scope))
}

// PropertyTenancyOverrideRow is a wrapper to implement RowMarshaler for models.PropertyTenancyOverride.
type PropertyTenancyOverrideRow models.PropertyTenancyOverride

// Render implements the Renderer interface for PropertyTenancyOverrideRow.
func (p PropertyTenancyOverrideRow) Render(scope string) []string {
	return []string{
		scope,
		p.Name,
		strings.Join(p.GetRegions(), ", "),
		p.GetValue(),
	}
}

// ToRow returns a table.Row for the PropertyTenancyOverrideRow, scoped by the given string.
func (p PropertyTenancyOverrideRow) ToRow(scope string) table.Row {
	return table.Row(p.Render(scope))
}

// GpuNodeRow is a wrapper to implement RowMarshaler for models.GpuNode.
type GpuNodeRow models.GpuNode

// Render implements the Renderer interface for GpuNodeRow.
func (n GpuNodeRow) Render(_ string) []string {
	return []string{
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

// ToRow returns a table.Row for the GpuNodeRow.
func (n GpuNodeRow) ToRow(scope string) table.Row {
	return table.Row(n.Render(scope))
}

// DedicatedAIClusterRow is a wrapper to implement RowMarshaler for models.DedicatedAICluster.
type DedicatedAIClusterRow models.DedicatedAICluster

// Render implements the Renderer interface for DedicatedAIClusterRow.
func (d DedicatedAIClusterRow) Render(scope string) []string {
	unitShapeOrProfile := d.UnitShape
	if unitShapeOrProfile == "" {
		unitShapeOrProfile = d.Profile
	}
	return []string{
		scope,
		d.Name,
		d.Type,
		unitShapeOrProfile,
		fmt.Sprint(d.Size),
		d.Status,
	}
}

// ToRow returns a table.Row for the DedicatedAIClusterRow, scoped by the given string.
func (d DedicatedAIClusterRow) ToRow(scope string) table.Row {
	return table.Row(d.Render(scope))
}

// TenantRow is a wrapper to implement RowMarshaler for models.Tenant.
type TenantRow models.Tenant

// Render implements the Renderer interface for TenantRow.
func (t TenantRow) Render(_ string) []string {
	return []string{
		t.Name,
		models.Tenant(t).GetTenantID(),
		fmt.Sprint(t.IsInternal),
		t.Note,
	}
}

// ToRow returns a table.Row for the TenantRow.
func (t TenantRow) ToRow(scope string) table.Row {
	return table.Row(t.Render(scope))
}

// ServiceTenancyRow is a wrapper to implement RowMarshaler for models.ServiceTenancy.
type ServiceTenancyRow models.ServiceTenancy

// Render implements the Renderer interface for ServiceTenancyRow.
func (s ServiceTenancyRow) Render(_ string) []string {
	return []string{
		s.Name,
		s.Realm,
		s.Environment,
		s.HomeRegion,
		strings.Join(s.Regions, ", "),
	}
}

// ToRow returns a table.Row for the ServiceTenancyRow.
func (s ServiceTenancyRow) ToRow(scope string) table.Row {
	return table.Row(s.Render(scope))
}

// EnvironmentRow is a wrapper to implement RowMarshaler for models.Environment.
type EnvironmentRow models.Environment

// Render implements the Renderer interface for EnvironmentRow.
func (e EnvironmentRow) Render(_ string) []string {
	return []string{
		models.Environment(e).GetName(),
		e.Realm,
		e.Type,
		e.Region,
	}
}

// ToRow returns a table.Row for the EnvironmentRow.
func (e EnvironmentRow) ToRow(scope string) table.Row {
	return table.Row(e.Render(scope))
}

// ModelArtifactRow adapts models.ModelArtifact for table rendering.
type ModelArtifactRow models.ModelArtifact

// Render implements the Renderer interface for ModelArtifactRow.
func (val ModelArtifactRow) Render(_ string) []string {
	return []string{
		val.ModelName,
		models.ModelArtifact(val).GetGpuConfig(),
		val.Name,
		val.TensorRTVersion,
	}
}

// ToRow returns a table.Row for the ModelArtifactRow.
func (val ModelArtifactRow) ToRow(scope string) table.Row {
	return table.Row(val.Render(scope))
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

	return collections.FilterMap(g, key, name, filter,
		func(s string, v T) table.Row {
			return GetTableRow(logger, s, v)
		})
}
