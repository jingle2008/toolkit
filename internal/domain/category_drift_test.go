package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every category's kube-backed status must be a deliberate, listed choice.
func TestNeedsKubeConfig_EveryCategoryAccountedFor(t *testing.T) {
	t.Parallel()
	want := map[Category]bool{
		Tenant:                          false,
		LimitDefinition:                 false,
		ConsolePropertyDefinition:       false,
		PropertyDefinition:              false,
		LimitTenancyOverride:            false,
		ConsolePropertyTenancyOverride:  false,
		PropertyTenancyOverride:         false,
		LimitRegionalOverride:           false,
		ConsolePropertyRegionalOverride: false,
		PropertyRegionalOverride:        false,
		BaseModel:                       true,
		ImportedModel:                   true,
		ModelArtifact:                   false,
		Environment:                     false,
		ServiceTenancy:                  false,
		GPUPool:                         false,
		GPUNode:                         true,
		GPUWorkload:                     true,
		DedicatedAICluster:              true,
		InferenceService:                true,
		ServingRuntime:                  true,
		Alias:                           false,
	}
	require.Len(t, want, len(Categories), "every category must have an expected NeedsKubeConfig value")
	for _, c := range Categories {
		exp, ok := want[c]
		require.Truef(t, ok, "no expected NeedsKubeConfig value for %s", c)
		assert.Equalf(t, exp, c.NeedsKubeConfig(), "NeedsKubeConfig mismatch for %s", c)
	}
}

// Every category's environment dependence must be a deliberate, listed choice.
func TestIsEnvironmentScoped_EveryCategoryAccountedFor(t *testing.T) {
	t.Parallel()
	want := map[Category]bool{
		Tenant:                          true,
		LimitDefinition:                 false,
		ConsolePropertyDefinition:       false,
		PropertyDefinition:              false,
		LimitTenancyOverride:            true,
		ConsolePropertyTenancyOverride:  true,
		PropertyTenancyOverride:         true,
		LimitRegionalOverride:           true,
		ConsolePropertyRegionalOverride: true,
		PropertyRegionalOverride:        true,
		BaseModel:                       true,
		ImportedModel:                   true,
		ModelArtifact:                   false,
		Environment:                     false,
		ServiceTenancy:                  false,
		GPUPool:                         true,
		GPUNode:                         true,
		GPUWorkload:                     true,
		DedicatedAICluster:              true,
		InferenceService:                true,
		ServingRuntime:                  true,
		Alias:                           false,
	}
	require.Len(t, want, len(Categories), "every category must have an expected IsEnvironmentScoped value")
	for _, c := range Categories {
		exp, ok := want[c]
		require.Truef(t, ok, "no expected IsEnvironmentScoped value for %s", c)
		assert.Equalf(t, exp, c.IsEnvironmentScoped(), "IsEnvironmentScoped mismatch for %s", c)
	}
}

// Scope graph must be internally consistent: every Contains child lists
// its parent back, every References child does not, and every relation
// is a drill-in target of its parent.
func TestScopeGraph_RoundTrips(t *testing.T) {
	t.Parallel()
	for _, r := range Relations() {
		assert.Containsf(t, r.Parent.ScopedCategories(), r.Child, "%v: child not scoped by parent", r)
		assert.Containsf(t, r.Child.ScopedBy(), r.Parent, "%v: parent missing from %s.ScopedBy()", r, r.Child)
		if r.Kind == Contains {
			assert.Containsf(t, r.Child.Parents(), r.Parent, "%v: parent missing from %s.Parents()", r, r.Child)
		} else {
			assert.NotContainsf(t, r.Child.Parents(), r.Parent, "%v: a reference is not a parent", r)
		}
	}
}

// Aliases must be unique across categories and round-trip through ParseCategory.
func TestAliases_UniqueAndRoundTrip(t *testing.T) {
	t.Parallel()
	seen := map[string]Category{}
	for _, c := range Categories {
		aliases := c.Aliases()
		assert.NotEmptyf(t, aliases, "%s must have at least one alias", c)
		for _, a := range aliases {
			if other, dup := seen[a]; dup {
				t.Errorf("alias %q is shared by %s and %s", a, other, c)
			}
			seen[a] = c
			got, err := ParseCategory(a)
			require.NoErrorf(t, err, "alias %q does not parse", a)
			assert.Equalf(t, c, got, "alias %q parses to %s, want %s", a, got, c)
		}
	}
}

// The relation registry must be well-formed: every endpoint is a data
// category, nothing relates to itself, no edge is declared twice, and
// the relation graph is acyclic (drill-in and the parent jump must
// terminate).
func TestRelations_WellFormed(t *testing.T) {
	t.Parallel()
	seen := make(map[Relation]bool)
	for _, r := range Relations() {
		for _, c := range []Category{r.Parent, r.Child} {
			assert.Truef(t, c >= Tenant && c <= LastDataCategory, "%v: endpoint %v is not a data category", r, c)
		}
		assert.NotEqualf(t, r.Parent, r.Child, "%v relates a category to itself", r)
		assert.Containsf(t, []RelationKind{Contains, References}, r.Kind, "%v: unknown kind", r)
		assert.Falsef(t, seen[r], "%v declared twice", r)
		seen[r] = true
	}

	// Depth-first search for a cycle through drill-in children.
	const (
		unvisited = iota
		visiting
		done
	)
	state := make(map[Category]int)
	var visit func(c Category, path []Category)
	visit = func(c Category, path []Category) {
		switch state[c] {
		case visiting:
			t.Errorf("relation cycle: %v → %v", path, c)
			return
		case done:
			return
		}
		state[c] = visiting
		for _, child := range c.ScopedCategories() {
			visit(child, append(path, c))
		}
		state[c] = done
	}
	for _, c := range Categories {
		visit(c, nil)
	}
}

// ScopedCategories and Parents hand out copies: a caller mutating the
// result must not corrupt the shared registry index.
func TestScopeGraph_ReturnsCopies(t *testing.T) {
	t.Parallel()
	got := Tenant.ScopedCategories()
	got[0] = Alias
	assert.Equal(t, LimitTenancyOverride, Tenant.ScopedCategories()[0])

	parents := LimitTenancyOverride.Parents()
	parents[0] = Alias
	assert.Equal(t, Tenant, LimitTenancyOverride.Parents()[0])
}
