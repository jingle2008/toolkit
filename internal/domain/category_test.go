package domain

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
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
		{ModelArtifact, "ModelArtifact"},
		{Environment, "Environment"},
		{ServiceTenancy, "ServiceTenancy"},
		{GpuPool, "GpuPool"},
		{GpuNode, "GpuNode"},
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
		Tenant, LimitDefinition, ConsolePropertyDefinition, PropertyDefinition, GpuPool, BaseModel,
	}
	nonScopeCases := []Category{
		LimitTenancyOverride, ConsolePropertyTenancyOverride, PropertyTenancyOverride,
		ConsolePropertyRegionalOverride, PropertyRegionalOverride, ModelArtifact,
		Environment, ServiceTenancy, GpuNode, DedicatedAICluster,
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
		{Tenant, []Category{LimitTenancyOverride, ConsolePropertyTenancyOverride, PropertyTenancyOverride, DedicatedAICluster}},
		{LimitDefinition, []Category{LimitTenancyOverride, LimitRegionalOverride}},
		{ConsolePropertyDefinition, []Category{ConsolePropertyTenancyOverride, ConsolePropertyRegionalOverride}},
		{PropertyDefinition, []Category{PropertyTenancyOverride, PropertyRegionalOverride}},
		{GpuPool, []Category{GpuNode}},
		{BaseModel, []Category{ModelArtifact}},
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

func TestCategory_IsScopeOf(t *testing.T) {
	t.Parallel()
	assert.True(t, Tenant.IsScopeOf(LimitTenancyOverride))
	assert.True(t, Tenant.IsScopeOf(ConsolePropertyTenancyOverride))
	assert.True(t, Tenant.IsScopeOf(PropertyTenancyOverride))
	assert.True(t, Tenant.IsScopeOf(DedicatedAICluster))
	assert.False(t, Tenant.IsScopeOf(GpuNode))
	assert.False(t, LimitTenancyOverride.IsScopeOf(Tenant))
	assert.True(t, BaseModel.IsScopeOf(ModelArtifact))
}

func TestCategory_Definition(t *testing.T) {
	t.Parallel()
	assert.Equal(t, LimitDefinition, LimitTenancyOverride.Definition())
	assert.Equal(t, ConsolePropertyDefinition, ConsolePropertyTenancyOverride.Definition())
	assert.Equal(t, ConsolePropertyDefinition, ConsolePropertyRegionalOverride.Definition())
	assert.Equal(t, PropertyDefinition, PropertyTenancyOverride.Definition())
	assert.Equal(t, PropertyDefinition, PropertyRegionalOverride.Definition())
	assert.Equal(t, GpuPool, GpuNode.Definition())

	// non-override category should return Category(-1)
	assert.Equal(t, Category(-1), Tenant.Definition())
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

	// Check that every category's GetAliases() are present in Aliases
	aliasSet := make(map[string]struct{}, len(aliases))
	for _, a := range aliases {
		aliasSet[a] = struct{}{}
	}
	for c := Tenant; c <= Alias; c++ {
		for _, a := range c.GetAliases() {
			_, ok := aliasSet[strings.ToLower(strings.TrimSpace(a))]
			assert.True(t, ok, "Category %v alias %q should be present in Aliases", c, a)
		}
	}
}
