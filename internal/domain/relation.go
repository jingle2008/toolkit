package domain

import "slices"

// RelationKind classifies how two categories relate.
type RelationKind int

const (
	// Contains links a parent to the child rows it owns or hosts: each
	// child row belongs to one parent row in that hierarchy (a child may
	// sit in more than one hierarchy — the tenancy overrides are owned by
	// both a Tenant and a Definition). Contains relations drive drill-in
	// (Enter), the parent jump (o), and scoped listing.
	Contains RelationKind = iota + 1
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
// in declaration order, and the first one is where Enter drills in.
var relations = []Relation{
	{Tenant, LimitTenancyOverride, Contains},
	{Tenant, ConsolePropertyTenancyOverride, Contains},
	{Tenant, PropertyTenancyOverride, Contains},
	{Tenant, DedicatedAICluster, Contains},
	{Tenant, ImportedModel, Contains},

	{LimitDefinition, LimitTenancyOverride, Contains},
	{LimitDefinition, LimitRegionalOverride, Contains},
	{ConsolePropertyDefinition, ConsolePropertyTenancyOverride, Contains},
	{ConsolePropertyDefinition, ConsolePropertyRegionalOverride, Contains},
	{PropertyDefinition, PropertyTenancyOverride, Contains},
	{PropertyDefinition, PropertyRegionalOverride, Contains},

	{GPUPool, GPUNode, Contains},
	{GPUNode, GPUWorkload, Contains},

	{DedicatedAICluster, InferenceService, Contains},
}

// Relations returns a copy of the relation registry, in declaration order.
func Relations() []Relation {
	return slices.Clone(relations)
}

// containsChildren and containsParents index the Contains relations by
// parent and by child, built once from the registry.
var containsChildren, containsParents = indexContains(relations)

func indexContains(rels []Relation) (children, parents map[Category][]Category) {
	children = make(map[Category][]Category)
	parents = make(map[Category][]Category)
	for _, r := range rels {
		if r.Kind != Contains {
			continue
		}
		children[r.Parent] = append(children[r.Parent], r.Child)
		parents[r.Child] = append(parents[r.Child], r.Parent)
	}
	return children, parents
}
