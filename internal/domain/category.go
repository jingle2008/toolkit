// Package domain defines core business types and category enums for the toolkit application.
//
//go:generate stringer -type=Category
package domain

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"
)

// Category represents a logical grouping for toolkit data.
type Category int

const (
	// CategoryUnknown is the zero value for Category.
	CategoryUnknown Category = iota

	// Tenant is a category for tenant-level data.
	Tenant
	// LimitDefinition is a category for limit definitions.
	LimitDefinition
	// ConsolePropertyDefinition is a category for console property definitions.
	ConsolePropertyDefinition
	// PropertyDefinition is a category for property definitions.
	PropertyDefinition
	// LimitTenancyOverride is a category for limit tenancy overrides.
	LimitTenancyOverride
	// ConsolePropertyTenancyOverride is a category for console property tenancy overrides.
	ConsolePropertyTenancyOverride
	// PropertyTenancyOverride is a category for property tenancy overrides.
	PropertyTenancyOverride
	// LimitRegionalOverride is a category for limit regional overrides.
	LimitRegionalOverride
	// ConsolePropertyRegionalOverride is a category for console property regional overrides.
	ConsolePropertyRegionalOverride
	// PropertyRegionalOverride is a category for property regional overrides.
	PropertyRegionalOverride
	// BaseModel is a category for base models.
	BaseModel
	// ImportedModel is a category for tenant-imported models —
	// either a namespaced ome.io BaseModel CR or a ClusterBaseModel
	// CR carrying a `tenancy-id` label. Complementary to BaseModel,
	// which is the shared / public catalog.
	ImportedModel
	// ModelArtifact is a category for model artifacts.
	ModelArtifact
	// Environment is a category for environments.
	Environment
	// ServiceTenancy is a category for service tenancies.
	ServiceTenancy
	// GPUPool is a category for GPU pools.
	GPUPool
	// GPUNode is a category for GPU nodes.
	GPUNode
	// GPUWorkload is a category for GPU-consuming pods (any pod that
	// requests nvidia.com/gpu or amd.com/gpu). Scoped by GPUNode: GPUPool → GPUNode →
	// GPUWorkload.
	GPUWorkload
	// DedicatedAICluster is a category for dedicated AI clusters.
	DedicatedAICluster
	// InferenceService is a category for OME InferenceService CRs — the
	// serving endpoints. Scoped by DedicatedAICluster (grouped by
	// namespace, which is the DAC ID for dedicated endpoints):
	// Tenant → DedicatedAICluster → InferenceService.
	InferenceService
	// ServingRuntime is a category for OME ClusterServingRuntime CRs —
	// the cluster-scoped templates describing how a class of models is
	// served (hardware constraints, container image, resource caps).
	ServingRuntime
	// Alias is a category for reporting all aliases.
	Alias
)

/*
NOTE: Category iteration should use the explicit range [Tenant, Alias].
Do not rely on a sentinel value.
*/

// LastDataCategory is the last category that lists data rows — every
// category in [Tenant, LastDataCategory] is one, and Alias (the meta
// list of category aliases) is the only one after it. Category cycling
// in the TUI walks this range, so new data categories must be declared
// before Alias.
const LastDataCategory = Alias - 1

// IsScopeOf returns true if the receiver is a scope of the given category.
func (e Category) IsScopeOf(o Category) bool {
	return slices.Contains(scopedChildren[e], o)
}

// IsScope returns true if the category is a scope category — i.e. it
// scopes at least one other category. Derived from the relation registry
// so the set of scope categories has a single source of truth.
func (e Category) IsScope() bool {
	return len(scopedChildren[e]) > 0
}

// ScopedCategories returns the categories that are scoped by the receiver
// — its children in the relation registry (Contains and References), in
// declaration order (Enter drills into the first with rows for the
// scope). Returns nil for a category that scopes nothing.
func (e Category) ScopedCategories() []Category {
	return slices.Clone(scopedChildren[e])
}

// Parents returns the receiver's structural parents — the categories
// that scope it through a Contains relation. A category may have more
// than one: the tenancy overrides are scoped by both a Tenant and a
// Definition. References are excluded (an InferenceService's BaseModel is
// not its parent), so this is the inverse of ScopedCategories restricted
// to Contains. Returns nil for top-level categories that have no parent.
func (e Category) Parents() []Category {
	return slices.Clone(containsParents[e])
}

/*
Aliases returns a list of aliases for the Category.
*/
func (e Category) Aliases() []string {
	cat := e.String()
	short := uppercaseLetters(cat)
	aliases := []string{strings.ToLower(short), strings.ToLower(cat)}

	// Preserve short aliases that users typed before the GPU/DAC
	// initialism normalization (Gpu→GPU, Dac→DAC) changed the
	// auto-computed initials. uppercaseLetters now yields GPUN/GPUP/DAIC
	// from these names; the old GN/GP/DAC short forms stay as aliases.
	switch e {
	case GPUNode:
		aliases = append(aliases, "gn")
	case GPUPool:
		aliases = append(aliases, "gp")
	case DedicatedAICluster:
		aliases = append(aliases, "dac")
	case GPUWorkload:
		aliases = append(aliases, "gw")
	default:
		// no pre-normalization short form
	}
	return aliases
}

/*
GetName returns the string name of the Category.
*/
func (e Category) GetName() string {
	return e.String()
}

/*
FilterableFields returns the filterable fields for the Category.
*/
func (e Category) FilterableFields() []string {
	return e.Aliases()
}

// IsFaulty always returns false. Category exists in the Filterable
// interface set but has no faulty notion of its own; the method is
// here only so Category satisfies models.Filterable alongside the
// real domain types.
func (e Category) IsFaulty() bool {
	return false
}

// NeedsKubeConfig reports whether loading this category requires a
// kubeconfig. These categories are sourced from a live cluster; the
// rest come from the on-disk repo.
func (e Category) NeedsKubeConfig() bool {
	switch e {
	case BaseModel, ImportedModel, GPUNode, DedicatedAICluster, GPUWorkload, InferenceService, ServingRuntime:
		return true
	default:
		return false
	}
}

// IsEnvironmentScoped reports whether this category's rows depend on the
// selected environment (realm/region or kube context), so switching
// environments reloads it. The rest are environment-independent.
func (e Category) IsEnvironmentScoped() bool {
	if e.NeedsKubeConfig() {
		return true
	}
	switch e {
	case Tenant, LimitTenancyOverride, ConsolePropertyTenancyOverride, PropertyTenancyOverride,
		LimitRegionalOverride, ConsolePropertyRegionalOverride, PropertyRegionalOverride, GPUPool:
		return true
	default:
		return false
	}
}

// uppercaseLetters returns the concatenated uppercase-letter glyphs
// of s (e.g. "DedicatedAICluster" → "DAIC"). Used by Aliases to
// build a short-form alias from a category name. Not the linguistic
// "initials" of words — purely a capital-letter filter.
func uppercaseLetters(s string) string {
	re := regexp.MustCompile(`[A-Z]`)
	initials := re.FindAllString(s, -1)
	return strings.Join(initials, "")
}

// Built once by init and never modified afterwards; callers get copies
// via Categories and AllAliases so none can corrupt the shared tables.
var (
	aliasToCat map[string]Category
	allAliases []string
	categories []Category
)

func init() {
	aliasToCat = make(map[string]Category)

	for c := Tenant; c <= Alias; c++ {
		for _, a := range c.Aliases() {
			aliasToCat[a] = c
		}
		categories = append(categories, c)
	}

	allAliases = make([]string, 0, len(aliasToCat))
	for k := range aliasToCat {
		allAliases = append(allAliases, k)
	}
	// Keep ordering stable so shell completion and TUI suggestions
	// don't reshuffle across runs of the program.
	sort.Strings(allAliases)
}

// Categories returns every defined category, in enum order. The slice is
// a fresh copy the caller may modify.
func Categories() []Category {
	return slices.Clone(categories)
}

// AllAliases returns every known category alias, sorted. The slice is a
// fresh copy the caller may modify.
func AllAliases() []string {
	return slices.Clone(allAliases)
}

// ErrUnknownCategory is returned when a string cannot be parsed into a known Category.
var ErrUnknownCategory = errors.New("unknown category")

/*
ParseCategory parses a string (case-insensitive, with common aliases) into a Category enum.
*/
func ParseCategory(s string) (Category, error) {
	key := strings.ToLower(strings.TrimSpace(s))
	if c, ok := aliasToCat[key]; ok {
		return c, nil
	}
	return CategoryUnknown, fmt.Errorf("parse category %q: %w", s, ErrUnknownCategory)
}

// AliasEntry is one category and the names it answers to: a row of `toolkit
// get alias`, of MCP's list_aliases, and of the TUI's Alias view.
type AliasEntry struct {
	Name    string   `json:"name"`
	Aliases []string `json:"aliases"`
}

// AliasEntries lists every category with its aliases, in category order.
func AliasEntries() []AliasEntry {
	out := make([]AliasEntry, 0, len(categories))
	for _, c := range categories {
		if c == CategoryUnknown {
			continue
		}
		out = append(out, AliasEntry{Name: c.String(), Aliases: c.Aliases()})
	}
	return out
}
