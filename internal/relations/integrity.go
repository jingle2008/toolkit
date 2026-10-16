package relations

import (
	"fmt"
	"maps"
	"slices"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/pkg/models"
)

// Severity grades an integrity check that found dangling rows.
type Severity string

const (
	// SeverityFail marks broken references: a row names a parent that
	// doesn't exist (an endpoint whose DAC was deleted).
	SeverityFail Severity = "FAIL"
	// SeverityWarn marks gaps that aren't broken state, such as a
	// tenancy missing from the tenant metadata.
	SeverityWarn Severity = "WARN"
)

// Dangling is one row that references a missing parent.
type Dangling struct {
	Row     string `json:"row"`
	Missing string `json:"missing"`
}

// Check is the outcome of one integrity check: a relation, or the
// tenancy-resolution rule. Severity applies when Dangling is non-empty.
type Check struct {
	Child    domain.Category `json:"child"`
	Parent   domain.Category `json:"parent"`
	Name     string          `json:"name"`
	Severity Severity        `json:"severity"`
	Checked  int             `json:"checked"`
	Dangling []Dangling      `json:"dangling,omitempty"`
	// Note adds context that is not a finding, e.g. how many endpoints
	// leave their runtime to deploy-time selection.
	Note string `json:"note,omitempty"`
}

// ref is one child row's reference toward a parent: the row's identity
// and the parent name it points at.
type ref struct{ row, key string }

// childRows lists, for one parent category, the references a scoped
// category's rows make toward it.
type childRows interface {
	refs(ds *models.Dataset, parent domain.Category) []ref
	// rowID names row (stored under group) the way refs does; row is
	// the category's item type.
	rowID(group string, row any) string
}

// groupedRows adapts a grouped category: its join keys, its rows, how a
// row is named in a report, and which rows a relation applies to (nil:
// every row with a key).
type groupedRows[T models.NamedFilterable] struct {
	keys    Grouped[T]
	pick    func(*models.Dataset) map[string][]T
	id      func(group string, it T) string
	applies func(parent domain.Category, it T) bool
}

func (g groupedRows[T]) rowID(group string, row any) string {
	it, ok := row.(T)
	if !ok {
		return ""
	}
	return g.id(group, it)
}

func (g groupedRows[T]) refs(ds *models.Dataset, parent domain.Category) []ref {
	m := g.pick(ds)
	var out []ref
	for _, group := range slices.Sorted(maps.Keys(m)) {
		for _, it := range m[group] {
			if g.applies != nil && !g.applies(parent, it) {
				continue
			}
			if k, ok := g.keys.ParentName(parent, group, it); ok {
				out = append(out, ref{row: g.id(group, it), key: k})
			}
		}
	}
	return out
}

// flatRows is groupedRows for a flat category.
type flatRows[T any] struct {
	keys Flat[T]
	pick func(*models.Dataset) []T
	id   func(T) string
}

func (f flatRows[T]) rowID(_ string, row any) string {
	it, ok := row.(T)
	if !ok {
		return ""
	}
	return f.id(it)
}

func (f flatRows[T]) refs(ds *models.Dataset, parent domain.Category) []ref {
	var out []ref
	for _, it := range f.pick(ds) {
		if k, ok := f.keys.ParentName(parent, it); ok {
			out = append(out, ref{row: f.id(it), key: k})
		}
	}
	return out
}

func nsName(ns, name string) string { return ns + "/" + name }

// children holds the rows of every scoped category, for the integrity
// checks. TestChildren_CoverScopedCategories keeps it complete.
var children = map[domain.Category]childRows{
	domain.LimitTenancyOverride: groupedRows[models.LimitTenancyOverride]{
		keys: LimitTenancyOverride,
		pick: func(d *models.Dataset) map[string][]models.LimitTenancyOverride { return d.LimitTenancyOverrideMap },
		id:   func(g string, o models.LimitTenancyOverride) string { return nsName(g, o.Name) },
	},
	domain.ConsolePropertyTenancyOverride: groupedRows[models.ConsolePropertyTenancyOverride]{
		keys: ConsolePropertyTenancyOverride,
		pick: func(d *models.Dataset) map[string][]models.ConsolePropertyTenancyOverride {
			return d.ConsolePropertyTenancyOverrideMap
		},
		id: func(g string, o models.ConsolePropertyTenancyOverride) string { return nsName(g, o.Name) },
	},
	domain.PropertyTenancyOverride: groupedRows[models.PropertyTenancyOverride]{
		keys: PropertyTenancyOverride,
		pick: func(d *models.Dataset) map[string][]models.PropertyTenancyOverride {
			return d.PropertyTenancyOverrideMap
		},
		id: func(g string, o models.PropertyTenancyOverride) string { return nsName(g, o.Name) },
	},
	domain.LimitRegionalOverride: flatRows[models.LimitRegionalOverride]{
		keys: LimitRegionalOverride,
		pick: func(d *models.Dataset) []models.LimitRegionalOverride { return d.LimitRegionalOverrides },
		id:   models.LimitRegionalOverride.GetName,
	},
	domain.ConsolePropertyRegionalOverride: flatRows[models.ConsolePropertyRegionalOverride]{
		keys: ConsolePropertyRegionalOverride,
		pick: func(d *models.Dataset) []models.ConsolePropertyRegionalOverride {
			return d.ConsolePropertyRegionalOverrides
		},
		id: models.ConsolePropertyRegionalOverride.GetName,
	},
	domain.PropertyRegionalOverride: flatRows[models.PropertyRegionalOverride]{
		keys: PropertyRegionalOverride,
		pick: func(d *models.Dataset) []models.PropertyRegionalOverride { return d.PropertyRegionalOverrides },
		id:   models.PropertyRegionalOverride.GetName,
	},
	domain.ImportedModel: groupedRows[models.ImportedModel]{
		keys: ImportedModel,
		pick: func(d *models.Dataset) map[string][]models.ImportedModel { return d.ImportedModelMap },
		id:   func(_ string, m models.ImportedModel) string { return nsName(m.Namespace, m.Name) },
	},
	domain.GPUNode: groupedRows[models.GPUNode]{
		keys: GPUNode,
		pick: func(d *models.Dataset) map[string][]models.GPUNode { return d.GPUNodeMap },
		id:   func(_ string, n models.GPUNode) string { return n.Name },
	},
	domain.GPUWorkload: groupedRows[models.GPUWorkload]{
		keys: GPUWorkload,
		pick: func(d *models.Dataset) map[string][]models.GPUWorkload { return d.GPUWorkloadMap },
		id:   func(_ string, w models.GPUWorkload) string { return nsName(w.Namespace, w.Name) },
		// Every pod has a namespace, but only one named after a DAC
		// points at a DAC; on-demand model namespaces don't.
		applies: func(parent domain.Category, w models.GPUWorkload) bool {
			return parent != domain.DedicatedAICluster || models.IsDACName(w.Namespace)
		},
	},
	domain.DedicatedAICluster: groupedRows[models.DedicatedAICluster]{
		keys: DedicatedAICluster,
		pick: func(d *models.Dataset) map[string][]models.DedicatedAICluster { return d.DedicatedAIClusterMap },
		id:   func(_ string, d models.DedicatedAICluster) string { return d.Name },
	},
	domain.InferenceService: groupedRows[models.InferenceService]{
		keys: InferenceService,
		pick: func(d *models.Dataset) map[string][]models.InferenceService { return d.InferenceServiceMap },
		id:   func(_ string, s models.InferenceService) string { return nsName(s.Namespace, s.Name) },
		// Only a dedicated endpoint's namespace is a DAC; an on-demand
		// one is named after its model.
		applies: func(parent domain.Category, s models.InferenceService) bool {
			return parent != domain.DedicatedAICluster || s.Type == models.InferenceServiceDedicated
		},
	},
}

// parentNameSets lists, per parent category, the names of its rows —
// what a child's key must match. TestParentNames_CoverParents requires an
// entry for every relation's parent.
var parentNameSets = map[domain.Category]func(*models.Dataset) []string{
	domain.Tenant:                    func(d *models.Dataset) []string { return names(d.Tenants) },
	domain.LimitDefinition:           func(d *models.Dataset) []string { return names(d.LimitDefinitionGroup.Values) },
	domain.ConsolePropertyDefinition: func(d *models.Dataset) []string { return names(d.ConsolePropertyDefinitionGroup.Values) },
	domain.PropertyDefinition:        func(d *models.Dataset) []string { return names(d.PropertyDefinitionGroup.Values) },
	domain.BaseModel:                 func(d *models.Dataset) []string { return names(d.BaseModels) },
	domain.ImportedModel:             func(d *models.Dataset) []string { return groupedNames(d.ImportedModelMap) },
	domain.ServingRuntime:            func(d *models.Dataset) []string { return names(d.ServingRuntimes) },
	domain.GPUPool:                   func(d *models.Dataset) []string { return names(d.GPUPools) },
	domain.GPUNode:                   func(d *models.Dataset) []string { return groupedNames(d.GPUNodeMap) },
	domain.DedicatedAICluster:        func(d *models.Dataset) []string { return groupedNames(d.DedicatedAIClusterMap) },
}

// names lists the items' names; nil when xs is (the category isn't
// loaded).
func names[T models.NamedItem](xs []T) []string {
	if xs == nil {
		return nil
	}
	out := make([]string, len(xs))
	for i, x := range xs {
		out[i] = x.GetName()
	}
	return out
}

// groupedNames is names across a grouped category; nil when m is.
func groupedNames[T models.NamedItem](m map[string][]T) []string {
	if m == nil {
		return nil
	}
	n := 0
	for _, xs := range m {
		n += len(xs)
	}
	out := make([]string, 0, n)
	for _, xs := range m {
		out = append(out, names(xs)...)
	}
	return out
}

// parentNames returns the set of a parent category's row names, nil when
// the category isn't loaded. ok is false for a category that parents
// nothing.
func parentNames(ds *models.Dataset, cat domain.Category) (map[string]bool, bool) {
	list, ok := parentNameSets[cat]
	if !ok {
		return nil, false
	}
	ns := list(ds)
	if ns == nil {
		return nil, true
	}
	set := make(map[string]bool, len(ns))
	for _, n := range ns {
		set[n] = true
	}
	return set, true
}

// CheckIntegrity checks every relation in the domain registry against ds,
// which must have every category loaded and tenant owners resolved: a row
// whose key names a parent row that doesn't exist is dangling. Relations
// to Tenant are not checked this way — a row whose tenancy didn't resolve
// has no tenant name to look up — and are covered by one tenancy check
// (WARN) instead, appended last. Results follow registry order.
func CheckIntegrity(ds *models.Dataset) []Check {
	var out []Check
	for _, r := range domain.Relations() {
		if r.Parent == domain.Tenant {
			continue
		}
		rows, ok := children[r.Child]
		if !ok {
			continue
		}
		names, _ := parentNames(ds, r.Parent)
		c := checkRelation(ds, r, rows, names)
		if r.Child == domain.InferenceService && r.Parent == domain.ServingRuntime {
			c.Note = unnamedRuntimeNote(ds)
		}
		out = append(out, c)
	}
	return append(out, checkTenancy(ds))
}

// checkRelation checks one relation: every child row whose key isn't in
// names (the parent's row names) is dangling.
func checkRelation(ds *models.Dataset, r domain.Relation, rows childRows, names map[string]bool) Check {
	c := Check{
		Child: r.Child, Parent: r.Parent, Severity: SeverityFail,
		Name: fmt.Sprintf("%s → %s", r.Child, r.Parent),
	}
	for _, ref := range rows.refs(ds, r.Parent) {
		c.Checked++
		if !names[ref.key] {
			c.Dangling = append(c.Dangling, Dangling{Row: ref.row, Missing: ref.key})
		}
	}
	return c
}

// DanglingRows returns a predicate reporting whether a row of child, stored
// under group, is one CheckIntegrity's FAIL checks report: it names a
// parent row that doesn't exist. Tenancy (WARN) is not included. It
// serves a partially loaded dataset — the TUI loads categories on
// demand — so a relation is checked only when its parent is loaded: a
// parent not loaded yet would make every row dangle. nil when no row
// dangles.
func DanglingRows(ds *models.Dataset, child domain.Category) func(group string, row any) bool {
	rows, ok := children[child]
	if !ok || ds == nil {
		return nil
	}
	set := make(map[string]bool)
	for _, r := range domain.Relations() {
		if r.Child != child || r.Parent == domain.Tenant {
			continue
		}
		names, _ := parentNames(ds, r.Parent)
		if names == nil {
			continue
		}
		for _, d := range checkRelation(ds, r, rows, names).Dangling {
			set[d.Row] = true
		}
	}
	if len(set) == 0 {
		return nil
	}
	return func(group string, row any) bool { return set[rows.rowID(group, row)] }
}

// unnamedRuntimeNote counts the ome.io endpoints that name no runtime:
// OME selects one at deploy time, so this is context, not a finding.
func unnamedRuntimeNote(ds *models.Dataset) string {
	n := 0
	for _, ss := range ds.InferenceServiceMap {
		for _, s := range ss {
			if !s.Legacy && s.Runtime == "" {
				n++
			}
		}
	}
	if n == 0 {
		return ""
	}
	return fmt.Sprintf("%d endpoint(s) name no runtime; OME selects one at deploy time", n)
}

// checkTenancy reports tenant-owned rows whose tenancy-id resolved to no
// tenant: a gap in the tenant list (override directories plus metadata),
// so WARN. Missing is the tenancy-id to add.
func checkTenancy(ds *models.Dataset) Check {
	c := Check{Child: domain.CategoryUnknown, Parent: domain.Tenant, Name: "tenancy resolution", Severity: SeverityWarn}
	visit := func(row, tenancyID string, owner *models.Tenant) {
		if tenancyID == "" {
			return
		}
		c.Checked++
		if owner == nil {
			c.Dangling = append(c.Dangling, Dangling{Row: row, Missing: tenancyID})
		}
	}
	for _, g := range slices.Sorted(maps.Keys(ds.DedicatedAIClusterMap)) {
		for _, d := range ds.DedicatedAIClusterMap[g] {
			visit("DedicatedAICluster "+d.Name, d.TenantID, d.Owner)
		}
	}
	for _, g := range slices.Sorted(maps.Keys(ds.ImportedModelMap)) {
		for _, m := range ds.ImportedModelMap[g] {
			visit("ImportedModel "+nsName(m.Namespace, m.Name), m.TenantID, m.Owner)
		}
	}
	for _, g := range slices.Sorted(maps.Keys(ds.GPUWorkloadMap)) {
		for _, w := range ds.GPUWorkloadMap[g] {
			visit("GPUWorkload "+nsName(w.Namespace, w.Name), w.TenantID, w.Owner)
		}
	}
	for _, g := range slices.Sorted(maps.Keys(ds.InferenceServiceMap)) {
		for _, s := range ds.InferenceServiceMap[g] {
			visit("InferenceService "+nsName(s.Namespace, s.Name), s.TenantID, s.Owner)
		}
	}
	return c
}
