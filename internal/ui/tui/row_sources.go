package tui

import (
	"maps"
	"slices"

	"github.com/charmbracelet/bubbles/table"

	"github.com/jingle2008/toolkit/internal/collections"
	"github.com/jingle2008/toolkit/internal/columns"
	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/pkg/models"
)

// rowCtx bundles every input the per-category row builders consume.
// Display callers leave realm/region empty and export=false; export
// callers populate realm/region and set export=true so each cell
// prefers columns.Column.RenderForExport over Render.
type rowCtx struct {
	dataset *models.Dataset
	scope   *domain.Scope
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
	// parents lists the categories this source can be scoped by — the
	// keys of its parent-key map. TestRowSources_ParentKeysMatchRelations
	// requires it to equal the domain relations into this category.
	parents []domain.Category
}

// parentKey says how a grouped child row is matched to a parent scope,
// comparing against Scope.Name (the parent row's first column). It has
// two forms, and both filter:
//
//   - byGroup (of == nil): the row's storage group — the map key, e.g. a
//     DAC's tenant. A nil of does NOT mean "match everything": it selects
//     the single group keyed by the scope name. filterScopedGroups
//     handles it as its own branch (a direct map lookup) rather than as
//     a func returning the key, so the free-text filter keeps matching
//     row fields only, exactly as before relations had keys.
//   - byField (of != nil): a key read from the row itself, e.g. a
//     workload's namespace for its DAC; rows are kept across all groups
//     when of(row) == Scope.Name.
type parentKey[T any] struct {
	of func(T) string // nil selects byGroup; see above
}

// byGroup matches a grouped child row by its storage group key: only the
// group named by the scope is listed.
func byGroup[T any]() parentKey[T] { return parentKey[T]{} }

// byField matches a child row by the key of returns for it: rows whose
// key equals the scope name are listed, from whichever groups hold them.
func byField[T any](of func(T) string) parentKey[T] { return parentKey[T]{of: of} }

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
// scope is active. parents maps each parent category to the key a row
// carries toward it (e.g. LimitRegionalOverride's GetName for
// LimitDefinition); when rc.scope names one of them, only rows whose key
// equals scope.Name are rendered. Keying on the scope's category (rather
// than "any non-nil scope") keeps the unguarded export path from
// filtering on an unrelated lingering scope.
func flatScopedSource[T models.NamedFilterable](
	cols columns.Set[T],
	parents map[domain.Category]func(T) string,
	pick func(*models.Dataset) []T,
) rowSource {
	return rowSource{
		rows: func(rc rowCtx) []table.Row {
			var keep func(T) bool
			if rc.scope != nil {
				if of, ok := parents[rc.scope.Category]; ok {
					name := rc.scope.Name
					keep = func(it T) bool { return of(it) == name }
				}
			}
			items := pick(rc.dataset)
			if rc.export {
				return tuiRowsFlatForExport(cols, items, keep, rc.realm, rc.region, rc.filter, rc.faulty)
			}
			return tuiRowsFlat(cols, items, keep, rc.filter, rc.faulty)
		},
		headers: headersFromSet(cols.Columns),
		find: func(d *models.Dataset, key models.ItemKey) any {
			name, ok := key.(string)
			if !ok {
				return nil
			}
			return collections.FindByName(pick(d), name)
		},
		parents: slices.Sorted(maps.Keys(parents)),
	}
}

// groupedSource is the grouped counterpart to flatSource. pick
// projects the dataset to the typed group→items map; parents maps each
// parent category to how a row is matched to it (see parentKey), e.g.
// domain.Tenant: byGroup for DedicatedAICluster, whose map is keyed by
// tenant. Filter/faulty/scope handling is shared between display and
// export.
func groupedSource[T models.NamedFilterable](
	cols columns.GroupedSet[T],
	parents map[domain.Category]parentKey[T],
	pick func(*models.Dataset) map[string][]T,
) rowSource {
	return rowSource{
		rows: func(rc rowCtx) []table.Row {
			data := pick(rc.dataset)
			if rc.export {
				return tuiRowsGroupedForExport(cols, data, parents, rc.scope, rc.realm, rc.region, rc.filter, rc.faulty)
			}
			return tuiRowsGrouped(cols, data, parents, rc.scope, rc.filter, rc.faulty)
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
		parents: slices.Sorted(maps.Keys(parents)),
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
		map[domain.Category]parentKey[models.LimitTenancyOverride]{
			domain.Tenant:          byGroup[models.LimitTenancyOverride](),
			domain.LimitDefinition: byField(models.LimitTenancyOverride.GetName),
		},
		func(d *models.Dataset) map[string][]models.LimitTenancyOverride { return d.LimitTenancyOverrideMap }),
	domain.ConsolePropertyTenancyOverride: groupedSource(columns.ConsolePropertyTenancyOverrideColumns,
		map[domain.Category]parentKey[models.ConsolePropertyTenancyOverride]{
			domain.Tenant:                    byGroup[models.ConsolePropertyTenancyOverride](),
			domain.ConsolePropertyDefinition: byField(models.ConsolePropertyTenancyOverride.GetName),
		},
		func(d *models.Dataset) map[string][]models.ConsolePropertyTenancyOverride {
			return d.ConsolePropertyTenancyOverrideMap
		}),
	domain.PropertyTenancyOverride: groupedSource(columns.PropertyTenancyOverrideColumns,
		map[domain.Category]parentKey[models.PropertyTenancyOverride]{
			domain.Tenant:             byGroup[models.PropertyTenancyOverride](),
			domain.PropertyDefinition: byField(models.PropertyTenancyOverride.GetName),
		},
		func(d *models.Dataset) map[string][]models.PropertyTenancyOverride {
			return d.PropertyTenancyOverrideMap
		}),
	// Regional overrides are scoped by their definition (e.g. drilling into
	// a LimitDefinition then tabbing to its regional overrides). They are a
	// flat slice named after the owning definition, so they narrow by
	// GetName.
	domain.LimitRegionalOverride: flatScopedSource(columns.LimitRegionalOverrideColumns,
		map[domain.Category]func(models.LimitRegionalOverride) string{
			domain.LimitDefinition: models.LimitRegionalOverride.GetName,
		},
		func(d *models.Dataset) []models.LimitRegionalOverride { return d.LimitRegionalOverrides }),
	domain.ConsolePropertyRegionalOverride: flatScopedSource(columns.ConsolePropertyRegionalOverrideColumns,
		map[domain.Category]func(models.ConsolePropertyRegionalOverride) string{
			domain.ConsolePropertyDefinition: models.ConsolePropertyRegionalOverride.GetName,
		},
		func(d *models.Dataset) []models.ConsolePropertyRegionalOverride {
			return d.ConsolePropertyRegionalOverrides
		}),
	domain.PropertyRegionalOverride: flatScopedSource(columns.PropertyRegionalOverrideColumns,
		map[domain.Category]func(models.PropertyRegionalOverride) string{
			domain.PropertyDefinition: models.PropertyRegionalOverride.GetName,
		},
		func(d *models.Dataset) []models.PropertyRegionalOverride { return d.PropertyRegionalOverrides }),
	domain.BaseModel: flatSource(columns.BaseModelColumns,
		func(d *models.Dataset) []models.BaseModel { return d.BaseModels }),
	// The TUI renders its own slimmer ServingRuntime set; the CLI keeps
	// the full ServingRuntimeColumns via the columns registry.
	domain.ServingRuntime: flatSource(columns.ServingRuntimeTUIColumns,
		func(d *models.Dataset) []models.ServingRuntime { return d.ServingRuntimes }),
	domain.ImportedModel: groupedSource(columns.ImportedModelColumns,
		map[domain.Category]parentKey[models.ImportedModel]{domain.Tenant: byGroup[models.ImportedModel]()},
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
		map[domain.Category]parentKey[models.GPUNode]{domain.GPUPool: byGroup[models.GPUNode]()},
		func(d *models.Dataset) map[string][]models.GPUNode { return d.GPUNodeMap }),
	// Workloads are grouped by node; a DAC's pods run in the namespace
	// named after it.
	domain.GPUWorkload: groupedSource(columns.GPUWorkloadColumns,
		map[domain.Category]parentKey[models.GPUWorkload]{
			domain.GPUNode:            byGroup[models.GPUWorkload](),
			domain.DedicatedAICluster: byField(func(w models.GPUWorkload) string { return w.Namespace }),
		},
		func(d *models.Dataset) map[string][]models.GPUWorkload { return d.GPUWorkloadMap }),
	domain.DedicatedAICluster: groupedSource(columns.DACColumns,
		map[domain.Category]parentKey[models.DedicatedAICluster]{domain.Tenant: byGroup[models.DedicatedAICluster]()},
		func(d *models.Dataset) map[string][]models.DedicatedAICluster { return d.DedicatedAIClusterMap }),
	// Endpoints are grouped by namespace (the DAC ID for a dedicated one)
	// and reference the model they serve.
	domain.InferenceService: groupedSource(columns.InferenceServiceColumns,
		map[domain.Category]parentKey[models.InferenceService]{
			domain.DedicatedAICluster: byGroup[models.InferenceService](),
			domain.BaseModel:          byField(sharedCatalogModel),
			domain.ImportedModel:      byField(currentAPIModel),
		},
		func(d *models.Dataset) map[string][]models.InferenceService { return d.InferenceServiceMap }),
}

// sharedCatalogModel returns the BaseModel (ClusterBaseModel) an endpoint
// serves, or "" when its model isn't in the shared catalog: a
// tenant-imported model, or a legacy endpoint's legacy Model CR, which
// could otherwise collide with a catalog name.
func sharedCatalogModel(s models.InferenceService) string {
	if s.ImportedModel || s.Legacy {
		return ""
	}
	return s.Model
}

// currentAPIModel returns the model an ome.io endpoint serves, or "" for a
// legacy endpoint (whose model is a legacy Model CR). Matching an
// ImportedModel scope keys on the name alone, not on ImportedModel: an
// imported model can also be a tenant-labelled ClusterBaseModel, which
// endpoints reference with kind ClusterBaseModel (so ImportedModel is
// false). Imported model names are OCIDs, so they can't collide with
// shared-catalog names.
func currentAPIModel(s models.InferenceService) string {
	if s.Legacy {
		return ""
	}
	return s.Model
}
