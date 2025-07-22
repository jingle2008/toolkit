package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/table"
	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/pkg/models"
	"github.com/stretchr/testify/assert"
)

func Test_aliasToRow(t *testing.T) {
	cat := domain.Tenant
	row := aliasToRow(cat)
	assert.Equal(t, table.Row{cat.String(), "t, tenant"}, row)
}

func Test_tenantToRow(t *testing.T) {
	tenant := models.Tenant{
		Name:       "T1",
		IDs:        []string{"tid1"},
		IsInternal: true,
		Note:       "note",
	}
	row := tenantToRow(tenant)
	assert.Equal(t, table.Row{"T1", "tid1", "true", "note"}, row)
}

func Test_limitDefinitionToRow(t *testing.T) {
	ld := models.LimitDefinition{
		Name:        "LD1",
		Description: "desc",
		Scope:       "scope",
		DefaultMin:  "1",
		DefaultMax:  "10",
	}
	row := limitDefinitionToRow(ld)
	assert.Equal(t, table.Row{"LD1", "desc", "scope", "1", "10"}, row)
}

type fakeDef struct {
	name, desc, value string
}

func (f fakeDef) GetName() string               { return f.name }
func (f fakeDef) GetDescription() string        { return f.desc }
func (f fakeDef) GetValue() string              { return f.value }
func (f fakeDef) GetFilterableFields() []string { return []string{f.name, f.desc, f.value} }
func (fakeDef) IsFaulty() bool                  { return false }

func Test_definitionToRow(t *testing.T) {
	def := fakeDef{"n", "d", "v"}
	row := definitionToRow(def)
	assert.Equal(t, table.Row{"n", "d", "v"}, row)
}

func Test_environmentToRow(t *testing.T) {
	env := models.Environment{
		Type:   "dev",
		Region: "us-phoenix-1",
		Realm:  "oc1",
	}
	row := environmentToRow(env)
	assert.Equal(t, table.Row{"dev-phx", "oc1", "dev", "us-phoenix-1"}, row)
}

func Test_serviceTenancyToRow(t *testing.T) {
	s := models.ServiceTenancy{
		Name:        "S1",
		Realm:       "realm",
		Environment: "env",
		HomeRegion:  "hr",
		Regions:     []string{"us", "eu"},
	}
	row := serviceTenancyToRow(s)
	assert.Equal(t, table.Row{"S1", "realm", "env", "hr", "us, eu"}, row)
}

func Test_gpuPoolToRow(t *testing.T) {
	g := models.GpuPool{
		Name:         "GP1",
		Shape:        "NVIDIA.A100.8",
		Size:         2,
		IsOkeManaged: true,
		CapacityType: "dedicated",
	}
	row := gpuPoolToRow(g)
	// GetGPUs: shape "NVIDIA.A100.8" * size 2 = 16
	assert.Equal(t, table.Row{"GP1", "NVIDIA.A100.8", "2", "16", "true", "dedicated"}, row)
}

func Test_limitTenancyOverrideToRow(t *testing.T) {
	lt := models.LimitTenancyOverride{
		LimitRegionalOverride: models.LimitRegionalOverride{
			Name:    "LTO1",
			Regions: []string{"us", "eu"},
			Values:  []models.LimitRange{{Min: 1, Max: 2}},
		},
	}
	row := limitTenancyOverrideToRow(lt, "tenant1")
	assert.Equal(t, table.Row{"LTO1", "tenant1", "us, eu", "1", "2"}, row)
}

type fakeDefOverride struct {
	name, value string
	regions     []string
}

func (f fakeDefOverride) GetName() string               { return f.name }
func (f fakeDefOverride) GetValue() string              { return f.value }
func (f fakeDefOverride) GetRegions() []string          { return f.regions }
func (f fakeDefOverride) GetFilterableFields() []string { return append(f.regions, f.name, f.value) }
func (fakeDefOverride) IsFaulty() bool                  { return false }

func Test_propertyTenancyOverrideToRow(t *testing.T) {
	def := fakeDefOverride{"PD1", "val", []string{"us", "eu"}}
	row := propertyTenancyOverrideToRow(def, "tenant2")
	assert.Equal(t, table.Row{"PD1", "tenant2", "us, eu", "val"}, row)
}

func Test_limitRegionalOverrideToRow(t *testing.T) {
	// With values
	lr := models.LimitRegionalOverride{
		Name:    "LR1",
		Regions: []string{"us"},
		Values:  []models.LimitRange{{Min: 3, Max: 5}},
	}
	row := limitRegionalOverrideToRow(lr)
	assert.Equal(t, table.Row{"LR1", "us", "3", "5"}, row)

	// No values
	lr2 := models.LimitRegionalOverride{
		Name:    "LR2",
		Regions: []string{"eu"},
		Values:  nil,
	}
	row2 := limitRegionalOverrideToRow(lr2)
	assert.Equal(t, table.Row{"LR2", "eu", "", ""}, row2)
}

func Test_propertyRegionalOverrideToRow(t *testing.T) {
	def := fakeDefOverride{"PR1", "v", []string{"us"}}
	row := propertyRegionalOverrideToRow(def)
	assert.Equal(t, table.Row{"PR1", "us", "v"}, row)
}

func Test_baseModelToRow(t *testing.T) {
	bm := models.BaseModel{
		Name:         "BM1",
		InternalName: "bm1",
		Version:      "v1",
		MaxTokens:    1024,
	}
	row := baseModelToRow(bm)
	assert.Equal(t, "BM1", row[0])
	assert.Equal(t, "", row[1])
	assert.Equal(t, "v1", row[2])
	assert.Equal(t, "1024", row[5])
}

func Test_modelArtifactToRow(t *testing.T) {
	ma := models.ModelArtifact{
		Name:            "artifactA",
		ModelName:       "M1",
		TensorRTVersion: "8.0",
		GpuCount:        0,
		GpuShape:        "",
	}
	row := modelArtifactToRow(ma, "")
	assert.Equal(t, table.Row{"artifactA", "M1", "0x ", "8.0"}, row)
}

func Test_gpuNodeToRow(t *testing.T) {
	gn := models.GpuNode{
		Name:         "node1",
		NodePool:     "pool1",
		InstanceType: "NVIDIA.A100.8",
		Allocatable:  8,
		Allocated:    2,
		IsHealthy:    true,
		IsReady:      false,
		Age:          "1d",
	}
	row := gpuNodeToRow(gn, "")
	// GetStatus: should be "ERROR: Not ready" since IsReady is false
	assert.Equal(t, table.Row{
		"node1", "pool1", "NVIDIA.A100.8", "8", "6", "true", "false", "1d", "ERROR: Not ready",
	}, row)
}

func Test_dedicatedAIClusterToRow(t *testing.T) {
	dac := models.DedicatedAICluster{
		Name:      "dac1",
		Type:      "t",
		UnitShape: "shape",
		Profile:   "",
		Size:      1,
		Age:       "2d",
		Status:    "active",
	}
	row := dedicatedAIClusterToRow(dac, "tenant1")
	assert.Equal(t, table.Row{
		"dac1", "tenant1", dac.GetOwnerState(), dac.GetUsage(), "t", "shape", "1", "2d", "active",
	}, row)
}
