package tui

import (
	"github.com/charmbracelet/bubbles/table"

	"github.com/jingle2008/toolkit/internal/columns"
	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/pkg/models"
)

// rowCtx bundles every input the per-category row builders consume.
// Display callers leave realm/region empty and export=false; export
// callers populate realm/region and set export=true so each cell
// prefers columns.Column.ExportRender over Render.
type rowCtx struct {
	dataset *models.Dataset
	context *domain.ToolkitContext
	realm   string
	region  string
	filter  string
	faulty  bool
	export  bool
}

// rowSource bundles a category's per-cell row builder and its
// precomputed headers. Each entry in rowSources is constructed by
// flatSource or groupedSource, which capture the typed column set
// and dataset accessor in a closure so the dispatch map can stay
// non-generic. headers is derived from the same column set at
// construction time, so the live table, the export, and the header
// strip can never drift.
type rowSource struct {
	rows    func(rowCtx) []table.Row
	headers []header
}

// flatSource builds a rowSource for a category backed by a flat
// columns.Set. pick projects the dataset to the typed slice. The
// returned closure dispatches to the display or export row helper
// based on rc.export — both share the same filter/faulty pipeline.
func flatSource[T models.NamedFilterable](
	cols columns.Set[T],
	pick func(*models.Dataset) []T,
) rowSource {
	return rowSource{
		rows: func(rc rowCtx) []table.Row {
			items := pick(rc.dataset)
			if rc.export {
				return tuiRowsFlatForExport(cols, items, rc.realm, rc.region, rc.filter, rc.faulty)
			}
			return tuiRowsFlat(cols, items, rc.filter, rc.faulty)
		},
		headers: headersFromSet(cols.Columns),
	}
}

// groupedSource is the grouped counterpart to flatSource. pick
// projects the dataset to the typed scope→items map; scope is the
// category that owns the grouping key (e.g. domain.Tenant for
// DedicatedAICluster). Filter/faulty/scope-context handling is
// shared between display and export.
func groupedSource[T models.NamedFilterable](
	cols columns.GroupedSet[T],
	scope domain.Category,
	pick func(*models.Dataset) map[string][]T,
) rowSource {
	return rowSource{
		rows: func(rc rowCtx) []table.Row {
			data := pick(rc.dataset)
			if rc.export {
				return tuiRowsGroupedForExport(cols, data, scope, rc.context, rc.realm, rc.region, rc.filter, rc.faulty)
			}
			return tuiRowsGrouped(cols, data, scope, rc.context, rc.filter, rc.faulty)
		},
		headers: headersFromGroupedSet(cols.Columns),
	}
}

// rowSources is the single per-category dispatch shared by the live
// table and the CSV export. Adding a new list-view category requires
// exactly one entry here. TestRowSources_CoversAllCategories asserts
// every domain.Category has one — a missing entry would otherwise
// silently emit a header-only CSV on <e>.
var rowSources = map[domain.Category]rowSource{
	domain.Alias: flatSource(columns.AliasColumns,
		func(*models.Dataset) []domain.Category { return domain.Categories }),
	domain.Tenant: flatSource(columns.TenantColumns,
		func(d *models.Dataset) []models.Tenant { return d.Tenants }),
	domain.LimitDefinition: flatSource(columns.LimitDefinitionColumns,
		func(d *models.Dataset) []models.LimitDefinition { return d.LimitDefinitionGroup.Values }),
	domain.ConsolePropertyDefinition: flatSource(columns.ConsolePropertyDefinitionColumns,
		func(d *models.Dataset) []models.ConsolePropertyDefinition {
			return d.ConsolePropertyDefinitionGroup.Values
		}),
	domain.PropertyDefinition: flatSource(columns.PropertyDefinitionColumns,
		func(d *models.Dataset) []models.PropertyDefinition { return d.PropertyDefinitionGroup.Values }),
	domain.LimitTenancyOverride: groupedSource(columns.LimitTenancyOverrideColumns, domain.Tenant,
		func(d *models.Dataset) map[string][]models.LimitTenancyOverride { return d.LimitTenancyOverrideMap }),
	domain.ConsolePropertyTenancyOverride: groupedSource(columns.ConsolePropertyTenancyOverrideColumns, domain.Tenant,
		func(d *models.Dataset) map[string][]models.ConsolePropertyTenancyOverride {
			return d.ConsolePropertyTenancyOverrideMap
		}),
	domain.PropertyTenancyOverride: groupedSource(columns.PropertyTenancyOverrideColumns, domain.Tenant,
		func(d *models.Dataset) map[string][]models.PropertyTenancyOverride {
			return d.PropertyTenancyOverrideMap
		}),
	domain.LimitRegionalOverride: flatSource(columns.LimitRegionalOverrideColumns,
		func(d *models.Dataset) []models.LimitRegionalOverride { return d.LimitRegionalOverrides }),
	domain.ConsolePropertyRegionalOverride: flatSource(columns.ConsolePropertyRegionalOverrideColumns,
		func(d *models.Dataset) []models.ConsolePropertyRegionalOverride {
			return d.ConsolePropertyRegionalOverrides
		}),
	domain.PropertyRegionalOverride: flatSource(columns.PropertyRegionalOverrideColumns,
		func(d *models.Dataset) []models.PropertyRegionalOverride { return d.PropertyRegionalOverrides }),
	domain.BaseModel: flatSource(columns.BaseModelColumns,
		func(d *models.Dataset) []models.BaseModel { return d.BaseModels }),
	domain.ImportedModel: groupedSource(columns.ImportedModelColumns, domain.Tenant,
		func(d *models.Dataset) map[string][]models.ImportedModel { return d.ImportedModelMap }),
	domain.ModelArtifact: groupedSource(columns.ModelArtifactColumns, domain.BaseModel,
		func(d *models.Dataset) map[string][]models.ModelArtifact { return d.ModelArtifactMap }),
	domain.Environment: flatSource(columns.EnvironmentColumns,
		func(d *models.Dataset) []models.Environment { return d.Environments }),
	domain.ServiceTenancy: flatSource(columns.ServiceTenancyColumns,
		func(d *models.Dataset) []models.ServiceTenancy { return d.ServiceTenancies }),
	domain.GpuPool: flatSource(columns.GpuPoolColumns,
		func(d *models.Dataset) []models.GpuPool { return d.GpuPools }),
	domain.GpuNode: groupedSource(columns.GpuNodeColumns, domain.GpuPool,
		func(d *models.Dataset) map[string][]models.GpuNode { return d.GpuNodeMap }),
	domain.DedicatedAICluster: groupedSource(columns.DacColumns, domain.Tenant,
		func(d *models.Dataset) map[string][]models.DedicatedAICluster { return d.DedicatedAIClusterMap }),
}
