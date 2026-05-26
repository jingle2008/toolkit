package tui

import (
	"github.com/charmbracelet/bubbles/table"

	"github.com/jingle2008/toolkit/internal/collections"
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
	scope   *domain.ToolkitContext
	realm   string
	region  string
	filter  string
	faulty  bool
	export  bool
}

// rowSource bundles a category's per-cell row builder, its
// precomputed headers, and a by-key item lookup. Each entry in
// rowSources is constructed by flatSource or groupedSource, which
// capture the typed column set and dataset accessor in a closure so
// the dispatch map can stay non-generic. All three fields are
// derived from the same pick accessor at construction time, so the
// live table, the export, the header strip, and findItem can never
// drift. find may be nil for categories that don't represent
// addressable entities (e.g. Alias, which is a categories index).
type rowSource struct {
	rows    func(rowCtx) []table.Row
	headers []header
	find    func(*models.Dataset, models.ItemKey) any
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
		find: func(d *models.Dataset, key models.ItemKey) any {
			name, ok := key.(string)
			if !ok {
				return nil
			}
			return collections.FindByName(pick(d), name)
		},
	}
}

// groupedSource is the grouped counterpart to flatSource. pick
// projects the dataset to the typed scope→items map; scopeCategory
// is the category that owns the grouping key (e.g. domain.Tenant
// for DedicatedAICluster). Filter/faulty/scope-context handling is
// shared between display and export.
func groupedSource[T models.NamedFilterable](
	cols columns.GroupedSet[T],
	scopeCategory domain.Category,
	pick func(*models.Dataset) map[string][]T,
) rowSource {
	return rowSource{
		rows: func(rc rowCtx) []table.Row {
			data := pick(rc.dataset)
			if rc.export {
				return tuiRowsGroupedForExport(cols, data, scopeCategory, rc.scope, rc.realm, rc.region, rc.filter, rc.faulty)
			}
			return tuiRowsGrouped(cols, data, scopeCategory, rc.scope, rc.filter, rc.faulty)
		},
		headers: headersFromGroupedSet(cols.Columns),
		find: func(d *models.Dataset, key models.ItemKey) any {
			k, ok := key.(models.ScopedItemKey)
			if !ok {
				return nil
			}
			if items, ok := pick(d)[k.Scope]; ok {
				return collections.FindByName(items, k.Name)
			}
			return nil
		},
	}
}

// flatSourceNoFind builds a rowSource like flatSource but without a
// find closure. Used for categories whose rows are an index rather
// than a set of addressable entities (Alias today — its rows are
// category names, not items the user can open in a detail view).
// Structurally distinct from a flatSource whose find has been
// mutated post-construction: the absence of find is part of the
// constructor's contract, so readers and future maintainers can't
// accidentally re-enable it.
func flatSourceNoFind[T models.NamedFilterable](
	cols columns.Set[T],
	pick func(*models.Dataset) []T,
) rowSource {
	s := flatSource(cols, pick)
	s.find = nil
	return s
}

// rowSources is the single per-category dispatch shared by the live
// table and the CSV export. Adding a new list-view category requires
// exactly one entry here. TestRowSources_CoversAllCategories asserts
// every domain.Category has one — a missing entry would otherwise
// silently emit a header-only CSV on <e>.
var rowSources = map[domain.Category]rowSource{
	// Alias rows index category names rather than addressable entities,
	// so flatSourceNoFind omits the find closure — findItem returns nil
	// for Alias and the TUI copy/detail actions degrade gracefully to
	// "no item selected".
	domain.Alias: flatSourceNoFind(columns.AliasColumns,
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
	domain.GPUPool: flatSource(columns.GPUPoolColumns,
		func(d *models.Dataset) []models.GPUPool { return d.GPUPools }),
	domain.GPUNode: groupedSource(columns.GPUNodeColumns, domain.GPUPool,
		func(d *models.Dataset) map[string][]models.GPUNode { return d.GPUNodeMap }),
	domain.DedicatedAICluster: groupedSource(columns.DACColumns, domain.Tenant,
		func(d *models.Dataset) map[string][]models.DedicatedAICluster { return d.DedicatedAIClusterMap }),
}
