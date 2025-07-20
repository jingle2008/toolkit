package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type testImpl struct{}

func (testImpl) GetName() string               { return "n" }
func (testImpl) GetFilterableFields() []string { return []string{"f"} }
func (testImpl) GetTenantID() string           { return "tid" }
func (testImpl) GetRegions() []string          { return []string{"r"} }
func (testImpl) GetValue() string              { return "v" }
func (testImpl) Environments() []Environment   { return nil }
func (testImpl) GetDescription() string        { return "desc" }
func (testImpl) IsFaulty() bool                { return false }

func TestDefinitionInterfaces(t *testing.T) {
	t.Parallel()
	var _ Filterable = testImpl{}
	var _ NamedItem = testImpl{}
	var _ NamedFilterable = testImpl{}
	var _ Definition = testImpl{}
	var _ TenancyOverride = testImpl{}
	var _ DefinitionOverride = testImpl{}
}

func TestLimitDefinitionGroup_AndOthers(t *testing.T) {
	t.Parallel()
	ldg := LimitDefinitionGroup{}
	assert.IsType(t, []LimitDefinition{}, ldg.Values)

	cpdg := ConsolePropertyDefinitionGroup{}
	assert.IsType(t, []ConsolePropertyDefinition{}, cpdg.Values)

	pdg := PropertyDefinitionGroup{}
	assert.IsType(t, []PropertyDefinition{}, pdg.Values)
}

func TestMetadata_GetTenants(t *testing.T) {
	t.Parallel()
	tenants := []TenantMetadata{
		{ID: "oc1.t1.dev"},
		{ID: "oc1.t2.prod"},
		{ID: "oc1.t3.dev"},
	}
	m := Metadata{Tenants: tenants}
	devs := m.GetTenants("dev")
	assert.Len(t, devs, 2)
	assert.Equal(t, "oc1.t1.dev", devs[0].ID)
	assert.Equal(t, "oc1.t3.dev", devs[1].ID)
}
