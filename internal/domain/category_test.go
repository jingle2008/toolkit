package domain

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCategory_String(t *testing.T) {
	t.Parallel()
	tests := []struct {
		cat     Category
		wantStr string
	}{
		{Tenant, "Tenant"},
		{LimitDefinition, "LimitDefinition"},
		{ConsolePropertyDefinition, "ConsolePropertyDefinition"},
		{PropertyDefinition, "PropertyDefinition"},
		{LimitTenancyOverride, "LimitTenancyOverride"},
		{ConsolePropertyTenancyOverride, "ConsolePropertyTenancyOverride"},
		{PropertyTenancyOverride, "PropertyTenancyOverride"},
		{ConsolePropertyRegionalOverride, "ConsolePropertyRegionalOverride"},
		{PropertyRegionalOverride, "PropertyRegionalOverride"},
		{BaseModel, "BaseModel"},
		{ImportedModel, "ImportedModel"},
		{ModelArtifact, "ModelArtifact"},
		{Environment, "Environment"},
		{ServiceTenancy, "ServiceTenancy"},
		{GPUPool, "GPUPool"},
		{GPUNode, "GPUNode"},
		{DedicatedAICluster, "DedicatedAICluster"},
		{Category(99), "Category(99)"},
	}
	for _, tt := range tests {
		t.Run(tt.wantStr, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.wantStr, tt.cat.String())
		})
	}
}

func TestCategory_IsScope(t *testing.T) {
	t.Parallel()
	scopeCases := []Category{
		Tenant, LimitDefinition, ConsolePropertyDefinition, PropertyDefinition, GPUPool,
	}
	nonScopeCases := []Category{
		LimitTenancyOverride, ConsolePropertyTenancyOverride, PropertyTenancyOverride,
		ConsolePropertyRegionalOverride, PropertyRegionalOverride, ModelArtifact,
		Environment, ServiceTenancy, GPUNode, DedicatedAICluster,
	}
	for _, c := range scopeCases {
		t.Run("scope_"+c.String(), func(t *testing.T) {
			t.Parallel()
			assert.True(t, c.IsScope(), "%v should be scope", c)
		})
	}
	for _, c := range nonScopeCases {
		t.Run("non_scope_"+c.String(), func(t *testing.T) {
			t.Parallel()
			assert.False(t, c.IsScope(), "%v should not be scope", c)
		})
	}
}

func TestCategory_ScopedCategories(t *testing.T) {
	t.Parallel()
	type want struct {
		scope Category
		want  []Category
	}
	cases := []want{
		{Tenant, []Category{LimitTenancyOverride, ConsolePropertyTenancyOverride, PropertyTenancyOverride, DedicatedAICluster, ImportedModel}},
		{LimitDefinition, []Category{LimitTenancyOverride, LimitRegionalOverride}},
		{ConsolePropertyDefinition, []Category{ConsolePropertyTenancyOverride, ConsolePropertyRegionalOverride}},
		{PropertyDefinition, []Category{PropertyTenancyOverride, PropertyRegionalOverride}},
		{GPUPool, []Category{GPUNode}},
	}
	for _, tc := range cases {
		t.Run(tc.scope.String(), func(t *testing.T) {
			t.Parallel()
			got := tc.scope.ScopedCategories()
			assert.ElementsMatch(t, tc.want, got)
		})
	}
	// non-scope category should return nil
	assert.Nil(t, ModelArtifact.ScopedCategories())
}

func TestCategory_Parents(t *testing.T) {
	t.Parallel()
	cases := []struct {
		child Category
		want  []Category
	}{
		// Single-parent children.
		{DedicatedAICluster, []Category{Tenant}},
		{ImportedModel, []Category{Tenant}},
		{GPUNode, []Category{GPUPool}},
		{LimitRegionalOverride, []Category{LimitDefinition}},
		{ConsolePropertyRegionalOverride, []Category{ConsolePropertyDefinition}},
		{PropertyRegionalOverride, []Category{PropertyDefinition}},
		// Dual-parent children: scoped by both a Tenant and a Definition.
		{LimitTenancyOverride, []Category{Tenant, LimitDefinition}},
		{ConsolePropertyTenancyOverride, []Category{Tenant, ConsolePropertyDefinition}},
		{PropertyTenancyOverride, []Category{Tenant, PropertyDefinition}},
	}
	for _, tc := range cases {
		t.Run(tc.child.String(), func(t *testing.T) {
			t.Parallel()
			assert.ElementsMatch(t, tc.want, tc.child.Parents())
		})
	}
	// Top-level categories have no parent.
	assert.Nil(t, Tenant.Parents())
	assert.Nil(t, BaseModel.Parents())
}

func TestCategory_IsScopeOf(t *testing.T) {
	t.Parallel()
	assert.True(t, Tenant.IsScopeOf(LimitTenancyOverride))
	assert.True(t, Tenant.IsScopeOf(ConsolePropertyTenancyOverride))
	assert.True(t, Tenant.IsScopeOf(PropertyTenancyOverride))
	assert.True(t, Tenant.IsScopeOf(DedicatedAICluster))
	assert.False(t, Tenant.IsScopeOf(GPUNode))
	assert.False(t, LimitTenancyOverride.IsScopeOf(Tenant))
}

func TestAliases(t *testing.T) {
	t.Parallel()
	aliases := Aliases
	assert.NotEmpty(t, aliases, "Aliases should not be empty")

	// Check that all aliases in Aliases are present in the aliasToCat map
	for _, alias := range aliases {
		_, ok := aliasToCat[alias]
		assert.True(t, ok, "Alias %q should be present in aliasToCat", alias)
	}

	// Check that every category's Aliases() are present in Aliases
	aliasSet := make(map[string]struct{}, len(aliases))
	for _, a := range aliases {
		aliasSet[a] = struct{}{}
	}
	for c := Tenant; c <= Alias; c++ {
		for _, a := range c.Aliases() {
			_, ok := aliasSet[strings.ToLower(strings.TrimSpace(a))]
			assert.True(t, ok, "Category %v alias %q should be present in Aliases", c, a)
		}
	}

	// Aliases must be sorted so shell completion and TUI suggestions are stable.
	assert.IsIncreasing(t, aliases, "Aliases should be sorted")
}

func TestParseCategory_GPUNodeShortAlias(t *testing.T) {
	t.Parallel()
	cat, err := ParseCategory("gn")
	require.NoError(t, err)
	assert.Equal(t, GPUNode, cat)
}

func TestAliases_ContainsAllCatLookupKeys(t *testing.T) {
	t.Parallel()
	// catLookup is private, but we can check that all aliases in Aliases are parseable
	for _, alias := range Aliases {
		cat, err := ParseCategory(alias)
		require.NoError(t, err, "Alias %q should be parseable", alias)
		assert.NotEqual(t, CategoryUnknown, cat, "Alias %q should not map to CategoryUnknown", alias)
	}
}

func TestAliases_IterationRange(t *testing.T) {
	t.Parallel()
	for c := Tenant; c <= Alias; c++ {
		aliases := c.Aliases()
		assert.NotEmpty(t, aliases, "Category %v should have at least one alias", c)
	}
}

func TestCategory_NeedsKubeConfig(t *testing.T) {
	t.Parallel()
	tests := []struct {
		cat  Category
		want bool
	}{
		{BaseModel, true},
		{ImportedModel, true},
		{GPUNode, true},
		{DedicatedAICluster, true},
		{Tenant, false},
	}
	for _, tt := range tests {
		t.Run(tt.cat.String(), func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, tt.cat.NeedsKubeConfig())
		})
	}
}

func TestParseCategory_Unknown(t *testing.T) {
	t.Parallel()
	cat, err := ParseCategory("not-real")
	require.Error(t, err)
	assert.Equal(t, CategoryUnknown, cat)
}
