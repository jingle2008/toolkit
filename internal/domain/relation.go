package domain

import "slices"

// RelationKind classifies how two categories relate.
type RelationKind int

const (
	// Contains links a parent to the child rows it owns or hosts: each
	// child row belongs to one parent row in that hierarchy (a child may
	// sit in more than one hierarchy — the tenancy overrides are owned by
	// both a Tenant and a Definition). Contains relations drive drill-in
	// (Enter), scoped listing, and the parent jump (o) — including from a
	// row reached without drilling in.
	Contains RelationKind = iota + 1
	// References links a parent to child rows that point at it through a
	// field rather than belonging to it (an InferenceService names the
	// BaseModel it serves). References drive drill-in and scoped listing
	// — "who points at this?" — but are not structural parents: the
	// parent jump follows one only via the breadcrumb of a drill-in.
	References
)

// Relation is a directed link between two categories.
type Relation struct {
	Parent Category
	Child  Category
	Kind   RelationKind
}

// relations is the registry of category relationships — the single
// source of truth for the scope graph. ScopedCategories and Parents are
// derived from it. Order is significant: a parent's children are listed
// in declaration order, and the first one is where Enter drills in. The
// TUI's row sources declare, per relation, how a child row is matched to
// a parent row (TestRowSources_ParentKeysMatchRelations keeps the two in
// step).
var relations = []Relation{
	{Tenant, LimitTenancyOverride, Contains},
	{Tenant, ConsolePropertyTenancyOverride, Contains},
	{Tenant, PropertyTenancyOverride, Contains},
	{Tenant, DedicatedAICluster, Contains},
	{Tenant, ImportedModel, Contains},
	// A tenant's pods and endpoints, by their resolved owner. Listed after
	// the Contains children so Enter still opens the limit overrides.
	{Tenant, GPUWorkload, References},
	{Tenant, InferenceService, References},

	{LimitDefinition, LimitTenancyOverride, Contains},
	{LimitDefinition, LimitRegionalOverride, Contains},
	{ConsolePropertyDefinition, ConsolePropertyTenancyOverride, Contains},
	{ConsolePropertyDefinition, ConsolePropertyRegionalOverride, Contains},
	{PropertyDefinition, PropertyTenancyOverride, Contains},
	{PropertyDefinition, PropertyRegionalOverride, Contains},

	{GPUPool, GPUNode, Contains},
	{GPUNode, GPUWorkload, Contains},

	{DedicatedAICluster, InferenceService, Contains},
	// A DAC's pods: GPU workloads in the DAC's namespace. A pod is placed
	// on (contained by) a node; its DAC is a reference through namespace.
	{DedicatedAICluster, GPUWorkload, References},

	// Endpoints serving a model (spec.model): from the shared catalog, or
	// imported by a tenant.
	{BaseModel, InferenceService, References},
	{ImportedModel, InferenceService, References},

	// What uses a runtime: the endpoints naming it (spec.runtime), then
	// the pods actually running it (serving-runtime label) — which also
	// covers endpoints whose runtime OME selects at deploy time.
	{ServingRuntime, InferenceService, References},
	{ServingRuntime, GPUWorkload, References},
}

// Relations returns a copy of the relation registry, in declaration order.
func Relations() []Relation {
	return slices.Clone(relations)
}

// scopedChildren indexes every relation by parent (drill-in targets, any
// kind); scopingParents indexes every relation by child; containsParents
// indexes the Contains relations by child (structural parents). All are
// built once from the registry.
var scopedChildren, scopingParents, containsParents = indexRelations(relations)

func indexRelations(rels []Relation) (children, scoping, contains map[Category][]Category) {
	children = make(map[Category][]Category)
	scoping = make(map[Category][]Category)
	contains = make(map[Category][]Category)
	for _, r := range rels {
		children[r.Parent] = append(children[r.Parent], r.Child)
		scoping[r.Child] = append(scoping[r.Child], r.Parent)
		if r.Kind == Contains {
			contains[r.Child] = append(contains[r.Child], r.Parent)
		}
	}
	return children, scoping, contains
}

// ScopedBy returns every category that scopes the receiver, through any
// relation kind, in declaration order — the inverse of ScopedCategories.
// Unlike Parents it includes References (an InferenceService is scoped by
// the BaseModel it serves). Returns nil when nothing scopes the receiver.
func (e Category) ScopedBy() []Category {
	return slices.Clone(scopingParents[e])
}
