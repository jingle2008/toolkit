package tui

import (
	"charm.land/bubbles/v2/table"

	"github.com/jingle2008/toolkit/internal/collections"
	"github.com/jingle2008/toolkit/internal/columns"
	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/relations"
	"github.com/jingle2008/toolkit/pkg/models"
)

// rowCtx bundles every input the per-category row builders consume.
// Display callers leave realm/region empty and export=false; export
// callers populate realm/region and set export=true so each cell
// prefers columns.Column.RenderForExport over Render.
type rowCtx struct {
	dataset  *models.Dataset
	category domain.Category
	scope    *domain.Scope
	realm    string
	region   string
	filter   string
	faulty   bool
	export   bool
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
	// parents lists the categories this source can be scoped by — the
	// parents of its relations key set. TestRowSources_ParentKeysMatchRelations
	// requires it to equal the domain relations into this category.
	parents []domain.Category
	// related returns the parent rows the item at key relates to, one
	// scope per parent category with a non-empty key, in parents order.
	// nil for categories that nothing scopes.
	related func(*models.Dataset, models.ItemKey) []domain.Scope
}

// flatSource builds a rowSource for a category backed by a flat
// columns.Set. pick projects the dataset to the typed slice. The
// returned closure dispatches to the display or export row helper
// based on rc.export — both share the same filter/faulty pipeline.
func flatSource[T models.NamedFilterable](
	cols columns.Set[T],
	pick func(*models.Dataset) []T,
) rowSource {
	return flatScopedSource(cols, nil, pick)
}

// flatScopedSource is a flatSource whose rows are narrowed when a parent
// scope is active, by the join keys in parents (see relations.Flat).
// Keying on the scope's category (rather than "any non-nil scope") keeps
// the unguarded export path from filtering on an unrelated lingering
// scope.
func flatScopedSource[T models.NamedFilterable](
	cols columns.Set[T],
	parents relations.Flat[T],
	pick func(*models.Dataset) []T,
) rowSource {
	return rowSource{
		rows: func(rc rowCtx) []table.Row {
			keep := parents.Keep(rc.scope)
			items := pick(rc.dataset)
			faulty := faultyGateFor(rc.faulty, rc.dataset, rc.category)
			if rc.export {
				return tuiRowsFlatForExport(cols, items, keep, rc.realm, rc.region, rc.filter, faulty)
			}
			return tuiRowsFlat(cols, items, keep, rc.filter, faulty)
		},
		headers: headersFromSet(cols.Columns),
		find: func(d *models.Dataset, key models.ItemKey) any {
			name, ok := key.(string)
			if !ok {
				return nil
			}
			return collections.FindByName(pick(d), name)
		},
		parents: parents.Parents(),
		related: func(d *models.Dataset, key models.ItemKey) []domain.Scope {
			name, ok := key.(string)
			if !ok || len(parents) == 0 {
				return nil
			}
			it := collections.FindByName(pick(d), name)
			if it == nil {
				return nil
			}
			var out []domain.Scope
			for _, p := range parents.Parents() {
				if n, ok := parents.ParentName(p, *it); ok {
					out = append(out, domain.Scope{Category: p, Name: n})
				}
			}
			return out
		},
	}
}

// groupedSource is the grouped counterpart to flatSource. pick
// projects the dataset to the typed group→items map; parents is the
// category's join keys (see relations.Grouped), e.g. tenant-keyed for
// DedicatedAICluster. Filter/faulty/scope handling is shared between
// display and export.
func groupedSource[T models.NamedFilterable](
	cols columns.GroupedSet[T],
	parents relations.Grouped[T],
	pick func(*models.Dataset) map[string][]T,
) rowSource {
	return rowSource{
		rows: func(rc rowCtx) []table.Row {
			data := pick(rc.dataset)
			faulty := faultyGateFor(rc.faulty, rc.dataset, rc.category)
			if rc.export {
				return tuiRowsGroupedForExport(cols, data, parents, rc.scope, rc.realm, rc.region, rc.filter, faulty)
			}
			return tuiRowsGrouped(cols, data, parents, rc.scope, rc.filter, faulty)
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
		parents: parents.Parents(),
		related: func(d *models.Dataset, key models.ItemKey) []domain.Scope {
			k, ok := key.(models.ScopedItemKey)
			if !ok || len(parents) == 0 {
				return nil
			}
			it := collections.FindByName(pick(d)[k.Scope], k.Name)
			if it == nil {
				return nil
			}
			var out []domain.Scope
			for _, p := range parents.Parents() {
				if n, ok := parents.ParentName(p, k.Scope, *it); ok {
					out = append(out, domain.Scope{Category: p, Name: n})
				}
			}
			return out
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
	// Tenancy overrides are grouped by tenant and named after their
	// definition, so both parents match without a lookup.
	domain.LimitTenancyOverride: groupedSource(columns.LimitTenancyOverrideColumns,
		relations.LimitTenancyOverride,
		func(d *models.Dataset) map[string][]models.LimitTenancyOverride { return d.LimitTenancyOverrideMap }),
	domain.ConsolePropertyTenancyOverride: groupedSource(columns.ConsolePropertyTenancyOverrideColumns,
		relations.ConsolePropertyTenancyOverride,
		func(d *models.Dataset) map[string][]models.ConsolePropertyTenancyOverride {
			return d.ConsolePropertyTenancyOverrideMap
		}),
	domain.PropertyTenancyOverride: groupedSource(columns.PropertyTenancyOverrideColumns,
		relations.PropertyTenancyOverride,
		func(d *models.Dataset) map[string][]models.PropertyTenancyOverride {
			return d.PropertyTenancyOverrideMap
		}),
	// Regional overrides are scoped by their definition (e.g. drilling into
	// a LimitDefinition then tabbing to its regional overrides). They are a
	// flat slice named after the owning definition, so they narrow by
	// GetName.
	domain.LimitRegionalOverride: flatScopedSource(columns.LimitRegionalOverrideColumns,
		relations.LimitRegionalOverride,
		func(d *models.Dataset) []models.LimitRegionalOverride { return d.LimitRegionalOverrides }),
	domain.ConsolePropertyRegionalOverride: flatScopedSource(columns.ConsolePropertyRegionalOverrideColumns,
		relations.ConsolePropertyRegionalOverride,
		func(d *models.Dataset) []models.ConsolePropertyRegionalOverride {
			return d.ConsolePropertyRegionalOverrides
		}),
	domain.PropertyRegionalOverride: flatScopedSource(columns.PropertyRegionalOverrideColumns,
		relations.PropertyRegionalOverride,
		func(d *models.Dataset) []models.PropertyRegionalOverride { return d.PropertyRegionalOverrides }),
	domain.BaseModel: flatSource(columns.BaseModelColumns,
		func(d *models.Dataset) []models.BaseModel { return d.BaseModels }),
	// The TUI renders its own slimmer ServingRuntime set; the CLI keeps
	// the full ServingRuntimeColumns via the columns registry.
	domain.ServingRuntime: flatSource(columns.ServingRuntimeTUIColumns,
		func(d *models.Dataset) []models.ServingRuntime { return d.ServingRuntimes }),
	domain.ImportedModel: groupedSource(columns.ImportedModelColumns,
		relations.ImportedModel,
		func(d *models.Dataset) map[string][]models.ImportedModel { return d.ImportedModelMap }),
	// Grouped by model internal name, but not (yet) a relation: the
	// BaseModel catalog covers few artifacts' models.
	domain.ModelArtifact: groupedSource(columns.ModelArtifactColumns, nil,
		func(d *models.Dataset) map[string][]models.ModelArtifact { return d.ModelArtifactMap }),
	domain.Environment: flatSource(columns.EnvironmentColumns,
		func(d *models.Dataset) []models.Environment { return d.Environments }),
	domain.ServiceTenancy: flatSource(columns.ServiceTenancyColumns,
		func(d *models.Dataset) []models.ServiceTenancy { return d.ServiceTenancies }),
	domain.GPUPool: flatSource(columns.GPUPoolColumns,
		func(d *models.Dataset) []models.GPUPool { return d.GPUPools }),
	domain.GPUNode: groupedSource(columns.GPUNodeColumns,
		relations.GPUNode,
		func(d *models.Dataset) map[string][]models.GPUNode { return d.GPUNodeMap }),
	// Workloads are grouped by node; a DAC's pods run in the namespace
	// named after it.
	domain.GPUWorkload: groupedSource(columns.GPUWorkloadColumns,
		relations.GPUWorkload,
		func(d *models.Dataset) map[string][]models.GPUWorkload { return d.GPUWorkloadMap }),
	domain.DedicatedAICluster: groupedSource(columns.DACColumns,
		relations.DedicatedAICluster,
		func(d *models.Dataset) map[string][]models.DedicatedAICluster { return d.DedicatedAIClusterMap }),
	// Endpoints are grouped by namespace (the DAC ID for a dedicated one)
	// and reference the model they serve.
	domain.InferenceService: groupedSource(columns.InferenceServiceColumns,
		relations.InferenceService,
		func(d *models.Dataset) map[string][]models.InferenceService { return d.InferenceServiceMap }),
}
