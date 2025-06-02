package toolkit

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCategory_String(t *testing.T) {
	tests := []struct {
		cat     Category
		wantStr string
	}{
		{Tenant, "Tenant"},
		{LimitDefinition, "Limit Definition"},
		{ConsolePropertyDefinition, "Console Property Definition"},
		{PropertyDefinition, "Property Definition"},
		{LimitTenancyOverride, "Limit Tenancy Override"},
		{ConsolePropertyTenancyOverride, "Console Property Tenancy Override"},
		{PropertyTenancyOverride, "Property Tenancy Override"},
		{ConsolePropertyRegionalOverride, "Console Property Regional Override"},
		{PropertyRegionalOverride, "Property Regional Override"},
		{BaseModel, "Base Model"},
		{ModelArtifact, "Model Artifact"},
		{Environment, "Environment"},
		{ServiceTenancy, "Service Tenancy"},
		{GpuPool, "GPU Pool"},
		{GpuNode, "GPU Node"},
		{DedicatedAICluster, "Dedicated AI Cluster"},
		{Category(99), "99"},
	}
	for _, tt := range tests {
		t.Run(tt.wantStr, func(t *testing.T) {
			assert.Equal(t, tt.wantStr, tt.cat.String())
		})
	}
}

func TestCategory_IsScope(t *testing.T) {
	scopeCases := []Category{
		Tenant, LimitDefinition, ConsolePropertyDefinition, PropertyDefinition, GpuPool,
	}
	nonScopeCases := []Category{
		LimitTenancyOverride, ConsolePropertyTenancyOverride, PropertyTenancyOverride,
		ConsolePropertyRegionalOverride, PropertyRegionalOverride, BaseModel, ModelArtifact,
		Environment, ServiceTenancy, GpuNode, DedicatedAICluster,
	}
	for _, c := range scopeCases {
		assert.True(t, c.IsScope(), fmt.Sprintf("%v should be scope", c))
	}
	for _, c := range nonScopeCases {
		assert.False(t, c.IsScope(), fmt.Sprintf("%v should not be scope", c))
	}
}

func TestCategory_ScopedCategories(t *testing.T) {
	type want struct {
		scope Category
		want  []Category
	}
	cases := []want{
		{Tenant, []Category{LimitTenancyOverride, ConsolePropertyTenancyOverride, PropertyTenancyOverride, DedicatedAICluster}},
		{LimitDefinition, []Category{LimitTenancyOverride}},
		{ConsolePropertyDefinition, []Category{ConsolePropertyTenancyOverride, ConsolePropertyRegionalOverride}},
		{PropertyDefinition, []Category{PropertyTenancyOverride, PropertyRegionalOverride}},
		{GpuPool, []Category{GpuNode}},
	}
	for _, tc := range cases {
		got := tc.scope.ScopedCategories()
		assert.ElementsMatch(t, tc.want, got)
	}
	// panic case
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic for non-scope category")
		}
	}()
	_ = BaseModel.ScopedCategories()
}

func TestCategory_IsScopeOf(t *testing.T) {
	assert.True(t, Tenant.IsScopeOf(LimitTenancyOverride))
	assert.True(t, Tenant.IsScopeOf(ConsolePropertyTenancyOverride))
	assert.True(t, Tenant.IsScopeOf(PropertyTenancyOverride))
	assert.True(t, Tenant.IsScopeOf(DedicatedAICluster))
	assert.False(t, Tenant.IsScopeOf(GpuNode))
	assert.False(t, LimitTenancyOverride.IsScopeOf(Tenant))
	assert.False(t, BaseModel.IsScopeOf(ModelArtifact))
}

func TestCategory_Definition(t *testing.T) {
	assert.Equal(t, LimitDefinition, LimitTenancyOverride.Definition())
	assert.Equal(t, ConsolePropertyDefinition, ConsolePropertyTenancyOverride.Definition())
	assert.Equal(t, ConsolePropertyDefinition, ConsolePropertyRegionalOverride.Definition())
	assert.Equal(t, PropertyDefinition, PropertyTenancyOverride.Definition())
	assert.Equal(t, PropertyDefinition, PropertyRegionalOverride.Definition())
	assert.Equal(t, GpuPool, GpuNode.Definition())

	// panic case
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic for non-override category")
		}
	}()
	_ = Tenant.Definition()
}
