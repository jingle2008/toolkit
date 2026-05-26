package cli

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/cli/output"
	"github.com/jingle2008/toolkit/internal/columns"
	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/pkg/models"
)

// Tests for the table renderers in get.go. These exercise the canonical
// columns.RenderTable registry surface from the CLI integration level,
// verifying that each category's canonical columns are consumable without
// going through the full `toolkit get` plumbing.

// --- Flat categories ---------------------------------------------------

func TestRenderTable_Tenant(t *testing.T) {
	t.Parallel()
	items := []models.Tenant{
		{Name: "t1", IDs: []string{"ocid1.a", "ocid1.b"}, IsInternal: true, Note: "n"},
	}
	headers, rows, err := columns.RenderTable(domain.Tenant, items, nil)
	require.NoError(t, err)
	assert.Equal(t, []string{"NAME", "OCIDS", "INTERNAL", "NOTE"}, headers)
	assert.Equal(t, [][]string{{"t1", "ocid1.a,ocid1.b", "true", "n"}}, rows)

	// Empty input still returns headers + empty rows.
	hdr, r, err2 := columns.RenderTable(domain.Tenant, []models.Tenant(nil), nil)
	require.NoError(t, err2)
	assert.Equal(t, headers, hdr)
	assert.Empty(t, r)
}

func TestRenderTable_BaseModel(t *testing.T) {
	t.Parallel()
	items := []models.BaseModel{
		{Name: "m1", DisplayName: "Model 1", Version: "1", Status: "READY"},
	}
	headers, rows, err := columns.RenderTable(domain.BaseModel, items, nil)
	require.NoError(t, err)
	// Internal/Vendor/Type dropped from the canonical set; 8 columns remain.
	assert.Equal(t, []string{"NAME", "DISPLAY NAME", "VERSION", "DAC SHAPE", "SIZE", "CONTEXT", "FLAGS", "STATUS"}, headers)
	require.Len(t, rows, 1)
	assert.Equal(t, "m1", rows[0][0])
	assert.Equal(t, "Model 1", rows[0][1])
	assert.Equal(t, "1", rows[0][2])
}

func TestRenderTable_GPUPool(t *testing.T) {
	t.Parallel()
	items := []models.GPUPool{{Name: "p1", Shape: "BM.GPU", AvailabilityDomain: "AD-1", Size: 8, ActualSize: 7, IsOkeManaged: true, CapacityType: "ondemand", Status: "RUNNING"}}
	headers, rows, err := columns.RenderTable(domain.GPUPool, items, nil)
	require.NoError(t, err)
	// All 9 columns are Default==true now.
	assert.Equal(t, []string{"NAME", "SHAPE", "AD", "SIZE", "ACTUAL SIZE", "GPUS", "OKE MANAGED", "CAPACITY TYPE", "STATUS"}, headers)
	assert.Equal(t, [][]string{{"p1", "BM.GPU", "AD-1", "8", "7", "0", "true", "ondemand", "RUNNING"}}, rows)
}

func TestRenderTable_LimitDefinition(t *testing.T) {
	t.Parallel()
	items := []models.LimitDefinition{{Name: "l1", Description: "d", Scope: "AD", DefaultMin: "0", DefaultMax: "10"}}
	headers, rows, err := columns.RenderTable(domain.LimitDefinition, items, nil)
	require.NoError(t, err)
	// Canonical headers: Name, Description, Scope, Min, Max (not "DEFAULT MIN"/"DEFAULT MAX")
	assert.Equal(t, []string{"NAME", "DESCRIPTION", "SCOPE", "MIN", "MAX"}, headers)
	assert.Equal(t, [][]string{{"l1", "d", "AD", "0", "10"}}, rows)
}

func TestRenderTable_PropertyDefinition(t *testing.T) {
	t.Parallel()
	items := []models.PropertyDefinition{{Name: "p1", Description: "d", DefaultValue: "v"}}
	headers, rows, err := columns.RenderTable(domain.PropertyDefinition, items, nil)
	require.NoError(t, err)
	// All 3 columns Default==true now.
	assert.Equal(t, []string{"NAME", "DESCRIPTION", "VALUE"}, headers)
	assert.Equal(t, [][]string{{"p1", "d", "v"}}, rows)
}

func TestRenderTable_ConsolePropertyDefinition(t *testing.T) {
	t.Parallel()
	items := []models.ConsolePropertyDefinition{{Name: "cp1", Description: "desc", Value: "v"}}
	headers, rows, err := columns.RenderTable(domain.ConsolePropertyDefinition, items, nil)
	require.NoError(t, err)
	assert.Equal(t, []string{"NAME", "DESCRIPTION", "VALUE"}, headers)
	assert.Equal(t, [][]string{{"cp1", "desc", "v"}}, rows)
}

func TestRenderTable_PropertyRegionalOverride(t *testing.T) {
	t.Parallel()
	items := []models.PropertyRegionalOverride{{
		Name:    "p1",
		Regions: []string{"us-ashburn-1", "us-phoenix-1"},
		Values: []struct {
			Value string `json:"value"`
		}{{Value: "v"}},
	}}
	headers, rows, err := columns.RenderTable(domain.PropertyRegionalOverride, items, nil)
	require.NoError(t, err)
	// All 3 columns Default==true now.
	assert.Equal(t, []string{"NAME", "REGIONS", "VALUE"}, headers)
	assert.Equal(t, [][]string{{"p1", "us-ashburn-1, us-phoenix-1", "v"}}, rows)
}

func TestRenderTable_LimitRegionalOverride(t *testing.T) {
	t.Parallel()
	items := []models.LimitRegionalOverride{{
		Name: "l1", Regions: []string{"us-ashburn-1"},
		Values: []models.LimitRange{{Min: 0, Max: 50}},
	}}
	headers, rows, err := columns.RenderTable(domain.LimitRegionalOverride, items, nil)
	require.NoError(t, err)
	// All 4 columns Default==true now.
	assert.Equal(t, []string{"NAME", "REGIONS", "MIN", "MAX"}, headers)
	assert.Equal(t, [][]string{{"l1", "us-ashburn-1", "0", "50"}}, rows)
}

func TestRenderTable_Environment(t *testing.T) {
	t.Parallel()
	items := []models.Environment{{Type: "dev", Region: "us-ashburn-1", Realm: "oc1"}}
	headers, rows, err := columns.RenderTable(domain.Environment, items, nil)
	require.NoError(t, err)
	// Canonical order: Name, Realm, Type, Region (different from legacy CLI: Name, Type, Region, Realm)
	assert.Equal(t, []string{"NAME", "REALM", "TYPE", "REGION"}, headers)
	require.Len(t, rows, 1)
	assert.Equal(t, "oc1", rows[0][1])
	assert.Equal(t, "dev", rows[0][2])
	assert.Equal(t, "us-ashburn-1", rows[0][3])
}

func TestRenderTable_ServiceTenancy(t *testing.T) {
	t.Parallel()
	items := []models.ServiceTenancy{{
		Name: "svc-a", Realm: "oc1", Environment: "dev",
		HomeRegion: "us-ashburn-1", Regions: []string{"us-ashburn-1", "us-phoenix-1"},
	}}
	headers, rows, err := columns.RenderTable(domain.ServiceTenancy, items, nil)
	require.NoError(t, err)
	// Type column renders the Environment field; separator is ", " not ","
	assert.Equal(t, []string{"NAME", "REALM", "TYPE", "HOME REGION", "REGIONS"}, headers)
	assert.Len(t, rows, 1)
	assert.Equal(t, "svc-a", rows[0][0])
	assert.Equal(t, "us-ashburn-1, us-phoenix-1", rows[0][4])
}

func TestRenderTable_Alias(t *testing.T) {
	t.Parallel()
	// Alias is 1-row-per-category (canonical TUI shape), not 1-row-per-alias (legacy CLI).
	cats := []domain.Category{domain.Tenant}
	headers, rows, err := columns.RenderTable(domain.Alias, cats, nil)
	require.NoError(t, err)
	assert.Equal(t, []string{"NAME", "ALIASES"}, headers)
	require.Len(t, rows, 1)
	assert.Equal(t, "Tenant", rows[0][0])
	// Aliases string is non-empty
	assert.NotEmpty(t, rows[0][1])
}

// --- Grouped categories ------------------------------------------------

func TestRenderTable_ImportedModel(t *testing.T) {
	t.Parallel()
	grouped := map[string][]models.ImportedModel{
		"ocid1.tenancy.x": {{
			BaseModel: models.BaseModel{Name: "im-a", Vendor: "acme", Version: "v1", Status: "Ready"},
			Namespace: "team-x",
			TenantID:  "ocid1.tenancy.x",
		}},
		"ocid1.tenancy.y": {{
			BaseModel: models.BaseModel{Name: "im-b", Vendor: "acme", Version: "v2", Status: "Ready"},
			TenantID:  "ocid1.tenancy.y",
		}},
	}
	headers, rows, err := columns.RenderTable(domain.ImportedModel, grouped, nil)
	require.NoError(t, err)
	assert.Equal(t, []string{"NAME", "TENANT", "NAMESPACE", "DISPLAY NAME", "VENDOR", "STATUS"}, headers)
	// renderGrouped iterates sorted keys; both rows present.
	require.Len(t, rows, 2)
	assert.Equal(t, "im-a", rows[0][0])
	assert.Equal(t, "ocid1.tenancy.x", rows[0][1])
	assert.Equal(t, "team-x", rows[0][2])
}

func TestRenderTable_GPUNode(t *testing.T) {
	t.Parallel()
	grouped := map[string][]models.GPUNode{
		"pool-b": {{Name: "n2", InstanceType: "BM", Age: "1d"}},
		"pool-a": {{Name: "n1", InstanceType: "BM", Age: "2d"}},
	}
	headers, rows, err := columns.RenderTable(domain.GPUNode, grouped, nil)
	require.NoError(t, err)
	// All 9 columns Default==true now; name-first (Decision #4).
	assert.Equal(t, []string{"NAME", "POOL", "TYPE", "TOTAL", "FREE", "HEALTHY", "READY", "AGE", "STATUS"}, headers)
	require.Len(t, rows, 2)
	// Sorted keys → pool-a first; name-first column ordering
	assert.Equal(t, "n1", rows[0][0])
	assert.Equal(t, "pool-a", rows[0][1])
	assert.Equal(t, "n2", rows[1][0])
	assert.Equal(t, "pool-b", rows[1][1])
}

func TestRenderTable_DAC(t *testing.T) {
	t.Parallel()
	grouped := map[string][]models.DedicatedAICluster{
		"tenant-a": {{Name: "d1", Status: "ACTIVE", Type: "HOSTING", UnitShape: "LARGE_COHERE", Size: 2, ModelName: "cohere-1"}},
	}
	headers, rows, err := columns.RenderTable(domain.DedicatedAICluster, grouped, nil)
	require.NoError(t, err)
	// All 10 columns Default==true now; name-first (Decision #4).
	assert.Equal(t, []string{"NAME", "TENANT", "INTERNAL", "USAGE", "TYPE", "MODEL", "SHAPE/PROFILE", "SIZE", "AGE", "STATUS"}, headers)
	require.Len(t, rows, 1)
	assert.Equal(t, "d1", rows[0][0])
	assert.Equal(t, "tenant-a", rows[0][1])
}

func TestRenderTable_LimitTenancyOverride(t *testing.T) {
	t.Parallel()
	grouped := map[string][]models.LimitTenancyOverride{
		"tenant-a": {{LimitRegionalOverride: models.LimitRegionalOverride{Name: "limit-1"}}},
	}
	headers, rows, err := columns.RenderTable(domain.LimitTenancyOverride, grouped, nil)
	require.NoError(t, err)
	// Canonical default columns: Name, Tenant, Regions, Min, Max (widened vs legacy TENANT|NAME)
	assert.Equal(t, []string{"NAME", "TENANT", "REGIONS", "MIN", "MAX"}, headers)
	require.Len(t, rows, 1)
	assert.Equal(t, "limit-1", rows[0][0])
	assert.Equal(t, "tenant-a", rows[0][1])
}

func TestRenderTable_ModelArtifact(t *testing.T) {
	t.Parallel()
	grouped := map[string][]models.ModelArtifact{
		"cohere-1": {{Name: "art-1", TensorRTVersion: "8.5"}},
	}
	headers, rows, err := columns.RenderTable(domain.ModelArtifact, grouped, nil)
	require.NoError(t, err)
	// Canonical default columns: Name, Model Internal Name, GPU Config, TensorRT
	assert.Equal(t, []string{"NAME", "MODEL INTERNAL NAME", "GPU CONFIG", "TENSORRT"}, headers)
	require.Len(t, rows, 1)
	assert.Equal(t, "art-1", rows[0][0])
	assert.Equal(t, "8.5", rows[0][3])
}

// --- writeSlice / writeMap dispatch tests --------------------------------

// TestWriteSlice_GPUPool_JSONShape pins the v0.3.0 lowercase JSON
// contract for `toolkit get gpupool -o json`. Enrichment fills
// `actualSize` and `status` (previously placeholders) but must not
// rename or drop any key. Regression bait against accidental struct-tag
// changes or struct renames during refactor.
func TestWriteSlice_GPUPool_JSONShape(t *testing.T) {
	t.Parallel()
	items := []models.GPUPool{{
		Name:               "p1",
		Shape:              "BM.GPU.A100-v2.8",
		Size:               8,
		ActualSize:         7,
		Status:             "RUNNING",
		CapacityType:       "on-demand",
		AvailabilityDomain: "AD-1",
		IsOkeManaged:       true,
	}}
	var buf bytes.Buffer
	require.NoError(t, writeSlice(&buf, items, 0, output.Options{Format: output.FormatJSON}, domain.GPUPool, models.Environment{}, nil))

	var arr []map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &arr))
	require.Len(t, arr, 1)
	item := arr[0]
	// Every key must use the lowercase JSON tag from pkg/models/gpu_pool.go.
	assert.Equal(t, "p1", item["name"])
	assert.Equal(t, "BM.GPU.A100-v2.8", item["shape"])
	assert.Equal(t, float64(8), item["size"], "size key (not Size)")
	assert.Equal(t, float64(7), item["actualSize"], "actualSize key (the enrichment landing site)")
	assert.Equal(t, "RUNNING", item["status"], "status key (the enrichment landing site)")
	assert.Equal(t, "on-demand", item["capacityType"])
	assert.Equal(t, "AD-1", item["availabilityDomain"])
	assert.Equal(t, true, item["isOkeManaged"])

	// Capitalized keys must NOT appear — would indicate a struct-tag regression.
	for _, k := range []string{"Name", "Shape", "Size", "ActualSize", "Status", "CapacityType"} {
		_, present := item[k]
		assert.False(t, present, "capitalized key %q must not appear (struct tag regression)", k)
	}
}

func TestWriteSlice_Formats(t *testing.T) {
	t.Parallel()
	items := []models.Tenant{{Name: "t1"}}
	for _, fmt := range []output.Format{output.FormatJSON, output.FormatJSONL, output.FormatYAML, output.FormatTable} {
		var buf bytes.Buffer
		err := writeSlice(&buf, items, 0, output.Options{Format: fmt}, domain.Tenant, models.Environment{}, nil)
		require.NoError(t, err, "format=%s", fmt)
		assert.Contains(t, buf.String(), "t1", "format=%s output: %q", fmt, buf.String())
	}

	// Unsupported format must surface a clear error.
	var buf bytes.Buffer
	err := writeSlice(&buf, items, 0, output.Options{Format: "toml"}, domain.Tenant, models.Environment{}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported format")
}

func TestWriteMap_Formats(t *testing.T) {
	t.Parallel()
	// Match what the loader produces: NodePool is set to the same
	// value as the map key (internal/infra/k8s/gpu_node.go:151).
	grouped := map[string][]models.GPUNode{"pool-a": {{Name: "n1", NodePool: "pool-a"}}}
	for _, fmt := range []output.Format{output.FormatJSON, output.FormatJSONL, output.FormatYAML, output.FormatTable} {
		var buf bytes.Buffer
		err := writeMap(&buf, grouped, 0, output.Options{Format: fmt}, domain.GPUNode, models.Environment{}, nil)
		require.NoError(t, err, "format=%s", fmt)
		got := buf.String()
		assert.True(t, strings.Contains(got, "n1") || strings.Contains(got, "pool-a"),
			"format=%s output should mention the group or item: %q", fmt, got)
	}

	var buf bytes.Buffer
	err := writeMap(&buf, grouped, 0, output.Options{Format: "toml"}, domain.GPUNode, models.Environment{}, nil)
	require.Error(t, err)
}

// TestWriteMap_GPUNodes_NoInjectedPoolField pins the no-inject
// contract for grouped categories whose value already carries the
// group key: the top-level JSON object must NOT have a redundant
// `pool` field — only the GPUNode-native `poolName` survives.
// Regression bait against accidentally re-injecting.
func TestWriteMap_GPUNodes_NoInjectedPoolField(t *testing.T) {
	t.Parallel()
	grouped := map[string][]models.GPUNode{"pool-a": {{Name: "n1", NodePool: "pool-a", IsReady: true}}}
	var buf bytes.Buffer
	err := writeMap(&buf, grouped, 0, output.Options{Format: output.FormatJSON, Pretty: true}, domain.GPUNode, models.Environment{}, nil)
	require.NoError(t, err)

	var arr []map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &arr))
	require.Len(t, arr, 1)
	item := arr[0]
	assert.Equal(t, "pool-a", item["poolName"], "originating pool should come through as poolName")
	assert.Equal(t, "n1", item["name"])
	_, hasPool := item["pool"]
	assert.False(t, hasPool, "redundant `pool` field should not be injected")
}

// TestWriteSlice_Limit pins that --limit caps the rendered output
// after filtering. Limit 0 means no cap (matches kubectl).
func TestWriteSlice_Limit(t *testing.T) {
	t.Parallel()
	items := []models.Tenant{{Name: "t1"}, {Name: "t2"}, {Name: "t3"}}

	var buf bytes.Buffer
	require.NoError(t, writeSlice(&buf, items, 2, output.Options{Format: output.FormatJSON, Pretty: true}, domain.Tenant, models.Environment{}, nil))
	var arr []map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &arr))
	assert.Len(t, arr, 2, "limit=2 should keep 2 of 3 items")
	assert.Equal(t, "t1", arr[0]["name"])
	assert.Equal(t, "t2", arr[1]["name"])

	buf.Reset()
	require.NoError(t, writeSlice(&buf, items, 0, output.Options{Format: output.FormatJSON, Pretty: true}, domain.Tenant, models.Environment{}, nil))
	require.NoError(t, json.Unmarshal(buf.Bytes(), &arr))
	assert.Len(t, arr, 3, "limit=0 should keep all 3 items")

	buf.Reset()
	require.NoError(t, writeSlice(&buf, items, 99, output.Options{Format: output.FormatJSON, Pretty: true}, domain.Tenant, models.Environment{}, nil))
	require.NoError(t, json.Unmarshal(buf.Bytes(), &arr))
	assert.Len(t, arr, 3, "limit > len(items) should be a no-op")
}

// TestWriteMap_Limit_CapsAcrossGroups pins that the limit applies to
// the flattened output, not per group. Two groups of two items each;
// limit=3 should yield 3 items across both groups (key-sorted).
func TestWriteMap_Limit_CapsAcrossGroups(t *testing.T) {
	t.Parallel()
	grouped := map[string][]models.GPUNode{
		"pool-a": {{Name: "a1", NodePool: "pool-a"}, {Name: "a2", NodePool: "pool-a"}},
		"pool-b": {{Name: "b1", NodePool: "pool-b"}, {Name: "b2", NodePool: "pool-b"}},
	}
	var buf bytes.Buffer
	require.NoError(t, writeMap(&buf, grouped, 3, output.Options{Format: output.FormatJSON, Pretty: true}, domain.GPUNode, models.Environment{}, nil))
	var arr []map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &arr))
	assert.Len(t, arr, 3, "limit=3 across 4 flattened items should yield 3")
	assert.Equal(t, "a1", arr[0]["name"])
	assert.Equal(t, "a2", arr[1]["name"])
	assert.Equal(t, "b1", arr[2]["name"], "should spill into pool-b's first item")
}

// TestWriteMap_DAC_CSVUsesFullOCIDs pins the CLI integration with
// columns.RenderTableForExport. With a fully-populated env, the
// csv branch in writeMap should route DAC Name and Tenant through
// RenderForExport, producing fully-qualified OCIDs instead of the
// short suffixes that -o table would show. Mirrors the TUI's
// TestExportTableCSV_DACUsesFullOCIDs assertion but locks the CLI
// path independently — a regression in writeMap's csv/tsv branch
// wouldn't be caught by the TUI test alone.
func TestWriteMap_DAC_CSVUsesFullOCIDs(t *testing.T) {
	t.Parallel()
	grouped := map[string][]models.DedicatedAICluster{
		"aaaaaaaatenant": {{
			Name:     "amaaaaaadac",
			Status:   "ACTIVE",
			TenantID: "aaaaaaaatenant",
		}},
	}
	env := models.Environment{Realm: "oc1", Region: "me-dubai-1"}
	var buf bytes.Buffer
	err := writeMap(&buf, grouped, 0, output.Options{Format: output.FormatCSV}, domain.DedicatedAICluster, env, nil)
	require.NoError(t, err)

	records, err := csv.NewReader(&buf).ReadAll()
	require.NoError(t, err)
	require.Len(t, records, 2) // header + 1 row
	// row[0]=NAME → full DAC OCID; row[1]=TENANT → full tenancy OCID.
	assert.Equal(t, "ocid1.generativeaidedicatedaicluster.oc1.me-dubai-1.amaaaaaadac", records[1][0])
	assert.Equal(t, "ocid1.tenancy.oc1..aaaaaaaatenant", records[1][1])
}

// TestWriteMap_DAC_CSVEmptyEnvFallsBackToRender pins the
// short-circuit in columns.RenderTableForExport: when realm OR
// region is empty, csv output uses display-mode Render rather
// than RenderForExport, producing short suffixes (not malformed
// ocid1.<type>.oc1..foo OCIDs). Belt-and-suspenders for the
// `if realm == "" || region == ""` guard.
func TestWriteMap_DAC_CSVEmptyEnvFallsBackToRender(t *testing.T) {
	t.Parallel()
	grouped := map[string][]models.DedicatedAICluster{
		"aaaaaaaatenant": {{Name: "amaaaaaadac", Status: "ACTIVE", TenantID: "aaaaaaaatenant"}},
	}
	// Realm set, region empty — partial env. Should fall back.
	env := models.Environment{Realm: "oc1"}
	var buf bytes.Buffer
	err := writeMap(&buf, grouped, 0, output.Options{Format: output.FormatCSV}, domain.DedicatedAICluster, env, nil)
	require.NoError(t, err)

	records, err := csv.NewReader(&buf).ReadAll()
	require.NoError(t, err)
	require.Len(t, records, 2)
	assert.Equal(t, "amaaaaaadac", records[1][0], "partial env must not produce malformed OCIDs")
	assert.Equal(t, "aaaaaaaatenant", records[1][1])
}

// TestWriteMap_DACs_NoInjectedTenantField pins the no-inject contract
// for DAC: the loader keys by dac.TenantID
// (internal/infra/k8s/dac.go:157), and that value is already on the
// model as the flat `tenantId` field. Injecting `tenant` would just
// duplicate. Regression bait against accidentally re-wrapping.
func TestWriteMap_DACs_NoInjectedTenantField(t *testing.T) {
	t.Parallel()
	grouped := map[string][]models.DedicatedAICluster{
		"acme": {{Name: "dac-1", Status: "READY", TenantID: "acme"}},
	}
	var buf bytes.Buffer
	err := writeMap(&buf, grouped, 0, output.Options{Format: output.FormatJSON, Pretty: true}, domain.DedicatedAICluster, models.Environment{}, nil)
	require.NoError(t, err)

	var arr []map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &arr))
	require.Len(t, arr, 1)
	item := arr[0]
	assert.Equal(t, "acme", item["tenantId"], "originating tenant should come through as tenantId")
	assert.Equal(t, "dac-1", item["name"])
	_, hasTenant := item["tenant"]
	assert.False(t, hasTenant, "redundant `tenant` field should not be injected")
}

// TestWriteMap_TenancyOverride_EmitsTenantName pins that the JSON
// output for tenancy override categories carries the tenant short
// name under "tenant". This used to come from FlattenWithKey
// injecting the map key; now the struct itself carries TenantName
// (populated by the configloader from the directory name) and
// writeMap is sufficient. The on-wire JSON shape is unchanged.
func TestWriteMap_TenancyOverride_EmitsTenantName(t *testing.T) {
	t.Parallel()
	grouped := map[string][]models.LimitTenancyOverride{
		"tenant-a": {{
			LimitRegionalOverride: models.LimitRegionalOverride{Name: "limit-1"},
			TenantName:            "tenant-a",
		}},
	}
	var buf bytes.Buffer
	err := writeMap(&buf, grouped, 0, output.Options{Format: output.FormatJSON, Pretty: true},
		domain.LimitTenancyOverride, models.Environment{}, nil)
	require.NoError(t, err)

	var arr []map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &arr))
	require.Len(t, arr, 1)
	item := arr[0]
	assert.Equal(t, "tenant-a", item["tenant"], "tenant field must come through from struct's TenantName")
}

// TestWriteAliases_CanonicalShape verifies the new writeAliases emits
// 1-row-per-category through the registry (canonical TUI shape,
// spec Decision #4).
func TestWriteAliases_CanonicalShape(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	err := writeAliases(&buf, "", 0, output.Options{Format: output.FormatCSV, NoHeaders: false}, nil)
	require.NoError(t, err)
	out := buf.String()
	// Header row must have the canonical column names
	lines := strings.Split(strings.TrimSpace(out), "\n")
	require.NotEmpty(t, lines)
	assert.Equal(t, "NAME,ALIASES", lines[0])
	// Must have at least one data row
	assert.Greater(t, len(lines), 1, "should have at least one category row")
}

// TestWriteAliases_Filter ensures the filter reduces the output.
func TestWriteAliases_Filter(t *testing.T) {
	t.Parallel()
	var bufAll, bufFiltered bytes.Buffer
	require.NoError(t, writeAliases(&bufAll, "", 0, output.Options{Format: output.FormatCSV}, nil))
	require.NoError(t, writeAliases(&bufFiltered, "tenant", 0, output.Options{Format: output.FormatCSV}, nil))
	linesAll := strings.Split(strings.TrimSpace(bufAll.String()), "\n")
	linesFiltered := strings.Split(strings.TrimSpace(bufFiltered.String()), "\n")
	assert.Less(t, len(linesFiltered), len(linesAll), "filtered output should have fewer rows than unfiltered")
}
