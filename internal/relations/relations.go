// Package relations holds the join keys behind the domain relation
// registry (domain.Relations): for every category that is scoped by
// another, how one of its rows is matched to a parent row. The TUI's
// drill-in and `toolkit get --related-to` both filter through these, so a
// relation means the same thing on every surface.
package relations

import (
	"maps"
	"slices"
	"strings"

	"github.com/jingle2008/toolkit/internal/collections"
	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/pkg/models"
)

// Key says how a row of a grouped (map) category is matched to a parent
// scope, comparing against Scope.Name — the parent row's first column. It
// has two forms, and both filter:
//
//   - ByGroup (of == nil): the row's storage group — the map key, e.g. a
//     DAC's tenant. A nil of does NOT mean "match everything": it selects
//     the single group keyed by the scope name. Grouped.Filter handles it
//     as its own branch (a direct map lookup) rather than as a func
//     returning the key, so the free-text filter keeps matching row
//     fields only, exactly as before relations had keys.
//   - ByField (of != nil): a key read from the row itself, e.g. a
//     workload's namespace for its DAC; rows are kept across all groups
//     when of(row) == Scope.Name.
type Key[T any] struct {
	of func(T) string // nil selects ByGroup; see above
}

// ByGroup matches a grouped row by its storage group key: only the group
// named by the scope is kept.
func ByGroup[T any]() Key[T] { return Key[T]{} }

// ByField matches a row by the key of returns for it: rows whose key
// equals the scope name are kept, from whichever groups hold them.
func ByField[T any](of func(T) string) Key[T] { return Key[T]{of: of} }

// Grouped maps each parent category of a grouped category to how its rows
// match that parent.
type Grouped[T models.NamedFilterable] map[domain.Category]Key[T]

// Filter applies the parent-scope gate, then the free-text filter and
// pred (nil for none). A nil scope, or one whose category isn't a parent
// here, doesn't narrow — so a lingering unrelated scope is harmless.
func (g Grouped[T]) Filter(m map[string][]T, scope *domain.Scope, filter string, pred func(T) bool) map[string][]T {
	if scope == nil {
		return collections.FilterMap(m, nil, nil, filter, pred)
	}
	k, ok := g[scope.Category]
	switch {
	case !ok:
		return collections.FilterMap(m, nil, nil, filter, pred)
	case k.of == nil:
		return collections.FilterMap(m, &scope.Name, nil, filter, pred)
	default:
		name := scope.Name
		return collections.FilterMapFunc(m, func(_ string, it T) bool { return k.of(it) == name }, filter, pred)
	}
}

// Parents returns the parent categories, sorted.
func (g Grouped[T]) Parents() []domain.Category { return slices.Sorted(maps.Keys(g)) }

// ParentName returns the name of the parent row that item (stored under
// group) relates to — the Scope.Name a drill-in from that parent would
// carry, run in reverse. ok is false when parent isn't a parent here or
// the item relates to no parent row (an empty key, e.g. an unresolved
// owner).
func (g Grouped[T]) ParentName(parent domain.Category, group string, item T) (string, bool) {
	k, ok := g[parent]
	if !ok {
		return "", false
	}
	name := group
	if k.of != nil {
		name = k.of(item)
	}
	return name, name != ""
}

// Flat maps each parent category of a flat (slice) category to the key
// its rows carry toward that parent. A flat row has no storage group, so
// every key is read from the row.
type Flat[T any] map[domain.Category]func(T) string

// Keep returns the predicate a scope imposes, or nil when the scope is
// nil or its category isn't a parent here (no narrowing).
func (f Flat[T]) Keep(scope *domain.Scope) func(T) bool {
	if scope == nil {
		return nil
	}
	of, ok := f[scope.Category]
	if !ok {
		return nil
	}
	name := scope.Name
	return func(it T) bool { return of(it) == name }
}

// Parents returns the parent categories, sorted.
func (f Flat[T]) Parents() []domain.Category { return slices.Sorted(maps.Keys(f)) }

// ParentName is Grouped.ParentName for a flat row.
func (f Flat[T]) ParentName(parent domain.Category, item T) (string, bool) {
	of, ok := f[parent]
	if !ok {
		return "", false
	}
	name := of(item)
	return name, name != ""
}

// The join keys, one set per scoped category. TestKeysMatchRelations
// requires each set's parents to equal the domain relations into it.
var (
	// Tenancy overrides are grouped by tenant and named after their
	// definition, so both parents match without a lookup.
	LimitTenancyOverride = Grouped[models.LimitTenancyOverride]{
		domain.Tenant:          ByGroup[models.LimitTenancyOverride](),
		domain.LimitDefinition: ByField(models.LimitTenancyOverride.GetName),
	}
	ConsolePropertyTenancyOverride = Grouped[models.ConsolePropertyTenancyOverride]{
		domain.Tenant:                    ByGroup[models.ConsolePropertyTenancyOverride](),
		domain.ConsolePropertyDefinition: ByField(models.ConsolePropertyTenancyOverride.GetName),
	}
	PropertyTenancyOverride = Grouped[models.PropertyTenancyOverride]{
		domain.Tenant:             ByGroup[models.PropertyTenancyOverride](),
		domain.PropertyDefinition: ByField(models.PropertyTenancyOverride.GetName),
	}

	// Regional overrides are a flat slice named after their definition.
	LimitRegionalOverride = Flat[models.LimitRegionalOverride]{
		domain.LimitDefinition: models.LimitRegionalOverride.GetName,
	}
	ConsolePropertyRegionalOverride = Flat[models.ConsolePropertyRegionalOverride]{
		domain.ConsolePropertyDefinition: models.ConsolePropertyRegionalOverride.GetName,
	}
	PropertyRegionalOverride = Flat[models.PropertyRegionalOverride]{
		domain.PropertyDefinition: models.PropertyRegionalOverride.GetName,
	}

	ImportedModel = Grouped[models.ImportedModel]{domain.Tenant: ByGroup[models.ImportedModel]()}

	GPUNode = Grouped[models.GPUNode]{domain.GPUPool: ByGroup[models.GPUNode]()}

	// Workloads are grouped by node; a DAC's pods run in the namespace
	// named after it.
	GPUWorkload = Grouped[models.GPUWorkload]{
		domain.GPUNode:            ByGroup[models.GPUWorkload](),
		domain.DedicatedAICluster: ByField(func(w models.GPUWorkload) string { return w.Namespace }),
		domain.Tenant:             ByField(func(w models.GPUWorkload) string { return ownerName(w.Owner) }),
		domain.ServingRuntime:     ByField(func(w models.GPUWorkload) string { return w.Runtime }),
	}

	DedicatedAICluster = Grouped[models.DedicatedAICluster]{domain.Tenant: ByGroup[models.DedicatedAICluster]()}

	// Endpoints are grouped by namespace (the DAC ID for a dedicated one)
	// and reference the model they serve and the runtime they name.
	InferenceService = Grouped[models.InferenceService]{
		domain.DedicatedAICluster: ByGroup[models.InferenceService](),
		domain.BaseModel:          ByField(sharedCatalogModel),
		domain.ImportedModel:      ByField(importedModelRef),
		domain.Tenant:             ByField(func(s models.InferenceService) string { return ownerName(s.Owner) }),
		domain.ServingRuntime:     ByField(currentAPIRuntime),
	}
)

// importedModelNamePrefix leads every imported model's name: the name is
// its OCID resource id. Shared-catalog names never carry it (checked on
// dp-prod-ord: none of 200 BaseModels does, all 253 ImportedModels do).
const importedModelNamePrefix = "amaaaaaa"

// IsImportedModelName reports whether a model name is an imported model's
// (an OCID resource id) rather than a shared-catalog name.
func IsImportedModelName(name string) bool {
	return strings.HasPrefix(name, importedModelNamePrefix)
}

// sharedCatalogModel returns the BaseModel (ClusterBaseModel) an endpoint
// serves, or "" when its model isn't in the shared catalog: an imported
// model (by name, which also covers a tenant-labelled ClusterBaseModel
// ref, whose ImportedModel flag is false), or a legacy endpoint's legacy
// Model CR, which could otherwise collide with a catalog name.
func sharedCatalogModel(s models.InferenceService) string {
	if s.ImportedModel || s.Legacy || IsImportedModelName(s.Model) {
		return ""
	}
	return s.Model
}

// importedModelRef returns the imported model an ome.io endpoint serves,
// or "" for a shared-catalog model or a legacy endpoint (whose model is a
// legacy Model CR). It keys on the name, not on ImportedModel: an imported
// model can also be a tenant-labelled ClusterBaseModel, which endpoints
// reference with kind ClusterBaseModel (so ImportedModel is false). With
// sharedCatalogModel it splits endpoints between the two model
// categories, so going to an endpoint's model never offers both.
func importedModelRef(s models.InferenceService) string {
	if s.Legacy || !IsImportedModelName(s.Model) {
		return ""
	}
	return s.Model
}

// currentAPIRuntime returns the ClusterServingRuntime an ome.io endpoint
// names, or "" for a legacy endpoint: its runtime is a legacy
// ome.oracle.com runtime, which can share a name with an ome.io one (the
// legacy content-moderator's gais-contentmoderate does on dp-prod-ord).
func currentAPIRuntime(s models.InferenceService) string {
	if s.Legacy {
		return ""
	}
	return s.Runtime
}

// ownerName returns the resolved owning tenant's name, or "" when the
// row's tenancy didn't resolve — such a row is under no tenant. The raw
// tenancy-id is deliberately not used: it is an OCID suffix, never a
// Tenant name.
func ownerName(t *models.Tenant) string {
	if t == nil {
		return ""
	}
	return t.Name
}
