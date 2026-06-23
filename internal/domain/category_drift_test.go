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
		Alias:                           false,
	}
	require.Len(t, want, len(Categories), "every category must have an expected NeedsKubeConfig value")
	for _, c := range Categories {
		exp, ok := want[c]
		require.Truef(t, ok, "no expected NeedsKubeConfig value for %s", c)
		assert.Equalf(t, exp, c.NeedsKubeConfig(), "NeedsKubeConfig mismatch for %s", c)
	}
}

// Scope graph must be internally consistent: every child lists its parent back.
func TestScopeGraph_RoundTrips(t *testing.T) {
	t.Parallel()
	for _, parent := range Categories {
		for _, child := range parent.ScopedCategories() {
			assert.Containsf(t, child.Parents(), parent,
				"%s is scoped by %s but %s is not in %s.Parents()", child, parent, parent, child)
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
