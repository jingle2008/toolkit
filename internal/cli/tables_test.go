package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/cli/output"
	"github.com/jingle2008/toolkit/pkg/models"
)

// Tests for the table renderers in get.go. These are pure functions:
// take typed input, return (headers, rows). Exercising them directly
// keeps coverage honest for the column specs and the generic
// tableFromSlice/tableFromGrouped helpers — the cmd-level tests in
// get_test.go never feed real data through them.

func TestBoolStr(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "true", boolStr(true))
	assert.Equal(t, "false", boolStr(false))
}

func TestSortedKeys(t *testing.T) {
	t.Parallel()
	m := map[string][]int{"c": nil, "a": nil, "b": nil}
	assert.Equal(t, []string{"a", "b", "c"}, sortedKeys(m))
	assert.Empty(t, sortedKeys(map[string][]int{}))
}

func TestTenantTable(t *testing.T) {
	t.Parallel()
	items := []models.Tenant{
		{Name: "t1", IDs: []string{"ocid1.a", "ocid1.b"}, IsInternal: true, Note: "n"},
	}
	headers, rows := tenantTable(items)
	assert.Equal(t, []string{"NAME", "IDS", "INTERNAL", "NOTE"}, headers)
	assert.Equal(t, [][]string{{"t1", "ocid1.a,ocid1.b", "true", "n"}}, rows)

	// Empty input still returns headers + empty rows.
	hdr, r := tenantTable(nil)
	assert.Equal(t, headers, hdr)
	assert.Empty(t, r)
}

func TestBaseModelTable(t *testing.T) {
	t.Parallel()
	items := []models.BaseModel{
		{Name: "m1", InternalName: "i", Vendor: "v", Type: "t", Version: "1", Status: "READY"},
	}
	headers, rows := baseModelTable(items)
	assert.Equal(t, []string{"NAME", "INTERNAL", "VENDOR", "TYPE", "VERSION", "STATUS", "FLAGS"}, headers)
	assert.Len(t, rows, 1)
	assert.Equal(t, "m1", rows[0][0])
}

func TestGpuPoolTable(t *testing.T) {
	t.Parallel()
	items := []models.GpuPool{{Name: "p1", Shape: "BM.GPU", Size: 8, CapacityType: "ondemand"}}
	headers, rows := gpuPoolTable(items)
	assert.Equal(t, []string{"NAME", "SHAPE", "SIZE", "CAPACITY TYPE"}, headers)
	assert.Equal(t, [][]string{{"p1", "BM.GPU", "8", "ondemand"}}, rows)
}

func TestGpuNodeTable(t *testing.T) {
	t.Parallel()
	grouped := map[string][]models.GpuNode{
		"pool-b": {{Name: "n2", InstanceType: "BM", Age: "1d"}},
		"pool-a": {{Name: "n1", InstanceType: "BM", Age: "2d"}},
	}
	headers, rows := gpuNodeTable(grouped)
	assert.Equal(t, []string{"POOL", "NAME", "STATUS", "INSTANCE TYPE", "AGE"}, headers)
	// Sorted keys → pool-a first.
	assert.Equal(t, "pool-a", rows[0][0])
	assert.Equal(t, "pool-b", rows[1][0])
}

func TestDacTable(t *testing.T) {
	t.Parallel()
	grouped := map[string][]models.DedicatedAICluster{
		"tenant-a": {{Name: "d1", Status: "ACTIVE", Type: "HOSTING", UnitShape: "LARGE_COHERE", Size: 2, ModelName: "cohere-1"}},
	}
	headers, rows := dacTable(grouped)
	assert.Equal(t, []string{"TENANT", "NAME", "STATUS", "TYPE", "UNIT SHAPE", "SIZE", "MODEL"}, headers)
	assert.Equal(t, [][]string{{"tenant-a", "d1", "ACTIVE", "HOSTING", "LARGE_COHERE", "2", "cohere-1"}}, rows)
}

func TestTenancyOverrideTable(t *testing.T) {
	t.Parallel()
	grouped := map[string][]models.LimitTenancyOverride{
		"tenant-a": {{LimitRegionalOverride: models.LimitRegionalOverride{Name: "limit-1"}}},
	}
	headers, rows := tenancyOverrideTable(grouped)
	assert.Equal(t, []string{"TENANT", "NAME"}, headers)
	assert.Equal(t, [][]string{{"tenant-a", "limit-1"}}, rows)
}

func TestLimitDefinitionTable(t *testing.T) {
	t.Parallel()
	items := []models.LimitDefinition{{Name: "l1", Description: "d", Scope: "AD", DefaultMin: "0", DefaultMax: "10"}}
	headers, rows := limitDefinitionTable(items)
	assert.Equal(t, []string{"NAME", "DESCRIPTION", "SCOPE", "DEFAULT MIN", "DEFAULT MAX"}, headers)
	assert.Equal(t, [][]string{{"l1", "d", "AD", "0", "10"}}, rows)
}

func TestDefinitionTable(t *testing.T) {
	t.Parallel()
	items := []models.PropertyDefinition{{Name: "p1", Description: "d"}}
	headers, rows := definitionTable(items)
	assert.Equal(t, []string{"NAME", "DESCRIPTION"}, headers)
	assert.Equal(t, [][]string{{"p1", "d"}}, rows)
}

func TestDefinitionOverrideTable(t *testing.T) {
	t.Parallel()
	items := []models.PropertyRegionalOverride{{Name: "p1", Regions: []string{"us-ashburn-1", "us-phoenix-1"}}}
	headers, rows := definitionOverrideTable(items)
	assert.Equal(t, []string{"NAME", "REGIONS"}, headers)
	assert.Equal(t, [][]string{{"p1", "us-ashburn-1,us-phoenix-1"}}, rows)
}

func TestEnvironmentTable(t *testing.T) {
	t.Parallel()
	items := []models.Environment{{Type: "dev", Region: "us-ashburn-1", Realm: "oc1"}}
	headers, rows := environmentTable(items)
	assert.Equal(t, []string{"NAME", "TYPE", "REGION", "REALM"}, headers)
	assert.Len(t, rows, 1)
	assert.Equal(t, "dev", rows[0][1])
	assert.Equal(t, "us-ashburn-1", rows[0][2])
	assert.Equal(t, "oc1", rows[0][3])
}

func TestServiceTenancyTable(t *testing.T) {
	t.Parallel()
	items := []models.ServiceTenancy{{
		Name: "svc-a", Realm: "oc1", Environment: "dev",
		HomeRegion: "us-ashburn-1", Regions: []string{"us-ashburn-1", "us-phoenix-1"},
	}}
	headers, rows := serviceTenancyTable(items)
	assert.Equal(t, []string{"NAME", "REALM", "ENVIRONMENT", "HOME REGION", "REGIONS"}, headers)
	assert.Equal(t, [][]string{{"svc-a", "oc1", "dev", "us-ashburn-1", "us-ashburn-1,us-phoenix-1"}}, rows)
}

func TestLimitRegionalOverrideTable(t *testing.T) {
	t.Parallel()
	items := []models.LimitRegionalOverride{{Name: "l1", Regions: []string{"us-ashburn-1"}}}
	headers, rows := limitRegionalOverrideTable(items)
	assert.Equal(t, []string{"NAME", "REGIONS"}, headers)
	assert.Equal(t, [][]string{{"l1", "us-ashburn-1"}}, rows)
}

func TestModelArtifactTable(t *testing.T) {
	t.Parallel()
	grouped := map[string][]models.ModelArtifact{
		"cohere-1": {{Name: "art-1", TensorRTVersion: "8.5"}},
	}
	headers, rows := modelArtifactTable(grouped)
	assert.Equal(t, []string{"MODEL", "NAME", "GPU CONFIG", "TENSORRT"}, headers)
	assert.Len(t, rows, 1)
	assert.Equal(t, "cohere-1", rows[0][0])
	assert.Equal(t, "art-1", rows[0][1])
	assert.Equal(t, "8.5", rows[0][3])
}

// writeSlice / writeMap dispatch between table and json/jsonl/yaml
// encoders. Exercise each branch so neither writer's format-switch
// can silently regress.

func TestWriteSlice_Formats(t *testing.T) {
	t.Parallel()
	items := []models.Tenant{{Name: "t1"}}
	for _, fmt := range []output.Format{output.FormatJSON, output.FormatJSONL, output.FormatYAML, output.FormatTable} {
		var buf bytes.Buffer
		err := writeSlice(&buf, items, output.Options{Format: fmt}, tenantTable)
		require.NoError(t, err, "format=%s", fmt)
		assert.Contains(t, buf.String(), "t1", "format=%s output: %q", fmt, buf.String())
	}

	// Unsupported format must surface a clear error.
	var buf bytes.Buffer
	err := writeSlice(&buf, items, output.Options{Format: "toml"}, tenantTable)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported format")
}

func TestWriteMap_Formats(t *testing.T) {
	t.Parallel()
	grouped := map[string][]models.GpuNode{"pool-a": {{Name: "n1"}}}
	for _, fmt := range []output.Format{output.FormatJSON, output.FormatJSONL, output.FormatYAML, output.FormatTable} {
		var buf bytes.Buffer
		err := writeMap(&buf, grouped, output.Options{Format: fmt}, gpuNodeTable, "pool")
		require.NoError(t, err, "format=%s", fmt)
		got := buf.String()
		assert.True(t, strings.Contains(got, "n1") || strings.Contains(got, "pool-a"),
			"format=%s output should mention the group or item: %q", fmt, got)
	}

	var buf bytes.Buffer
	err := writeMap(&buf, grouped, output.Options{Format: "toml"}, gpuNodeTable, "pool")
	require.Error(t, err)
}
