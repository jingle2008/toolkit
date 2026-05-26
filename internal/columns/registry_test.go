package columns

import (
	"math"
	"reflect"
	"testing"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/pkg/models"
)

// Every concrete Category must have a registered column set.
//
// Skipped only during the bootstrap state (registered == 0).
// All 19 categories are now registered — this is a live invariant.
func TestRegistry_EveryCategoryRegistered(t *testing.T) {
	t.Parallel()
	var missing []domain.Category
	registered := 0
	for _, cat := range domain.Categories {
		if cat == domain.CategoryUnknown {
			continue
		}
		if IsRegistered(cat) {
			registered++
		} else {
			missing = append(missing, cat)
		}
	}
	if registered == 0 {
		t.Skip("bootstrap state: no categories registered yet")
	}
	if len(missing) > 0 {
		t.Errorf("missing %d of %d categories: %v",
			len(missing), registered+len(missing), missing)
	}
}

// Keys must be unique, non-empty; `help` is reserved.
func TestRegistry_KeysValid(t *testing.T) {
	t.Parallel()
	for _, cat := range domain.Categories {
		if cat == domain.CategoryUnknown || !IsRegistered(cat) {
			continue
		}
		keys := KeysFor(cat)
		if len(keys) == 0 {
			t.Errorf("%s: no columns registered", cat)
			continue
		}
		seen := make(map[string]bool, len(keys))
		for _, k := range keys {
			if k == "" {
				t.Errorf("%s: empty key", cat)
			}
			if k == "help" {
				t.Errorf("%s: key %q is reserved", cat, k)
			}
			if seen[k] {
				t.Errorf("%s: duplicate key %q", cat, k)
			}
			seen[k] = true
		}
	}
}

// Ratios per set must sum to ~1.0 (±0.02).
func TestRegistry_RatiosSumToOne(t *testing.T) {
	t.Parallel()
	for _, cat := range domain.Categories {
		if cat == domain.CategoryUnknown || !IsRegistered(cat) {
			continue
		}
		sum := RatioSumFor(cat)
		if math.Abs(sum-1.0) > 0.02 {
			t.Errorf("%s: ratio sum %.3f outside ±0.02 of 1.0", cat, sum)
		}
	}
}

// TestTitlesFor covers the registry lookup that exposes per-column
// titles to the CLI's --columns help output. Verifies both a
// registered category (titles non-empty, length matches keys) and
// an unregistered one (nil).
func TestTitlesFor(t *testing.T) {
	t.Parallel()
	titles := TitlesFor(domain.Tenant)
	if len(titles) == 0 {
		t.Fatalf("TitlesFor(Tenant): empty")
	}
	if len(titles) != len(KeysFor(domain.Tenant)) {
		t.Errorf("Titles length %d != Keys length %d", len(titles), len(KeysFor(domain.Tenant)))
	}
	if TitlesFor(domain.CategoryUnknown) != nil {
		t.Error("TitlesFor(CategoryUnknown): want nil")
	}
}

// TestRenderTable_Flat drives RenderTable end-to-end for a flat
// category. Pins the registry dispatch path: registry[Tenant] →
// newFlatEntry's render closure → renderFlat.
func TestRenderTable_Flat(t *testing.T) {
	t.Parallel()
	items := []models.Tenant{
		{Name: "alpha", IDs: []string{"ocid1.tenancy.oc1..a"}, IsInternal: true, Note: "x"},
	}
	headers, rows, err := RenderTable(domain.Tenant, items, nil)
	if err != nil {
		t.Fatalf("RenderTable: %v", err)
	}
	if len(headers) == 0 || len(rows) != 1 {
		t.Fatalf("headers=%d rows=%d", len(headers), len(rows))
	}
	if rows[0][0] != "alpha" {
		t.Errorf("row[0][0] = %q, want alpha", rows[0][0])
	}
}

// TestRenderTable_Grouped drives RenderTable for a grouped category.
// Pins registry[GPUNode] → newGroupedEntry's render closure →
// renderGrouped, including the sorted-key iteration.
func TestRenderTable_Grouped(t *testing.T) {
	t.Parallel()
	items := map[string][]models.GPUNode{
		"pool-a": {{Name: "node-1", InstanceType: "BM.GPU4.8", Allocatable: 8, Allocated: 1, IsReady: true, Age: "1d"}},
	}
	headers, rows, err := RenderTable(domain.GPUNode, items, nil)
	if err != nil {
		t.Fatalf("RenderTable: %v", err)
	}
	if len(headers) == 0 || len(rows) != 1 {
		t.Fatalf("headers=%d rows=%d", len(headers), len(rows))
	}
}

// TestRenderTable_Unregistered ensures unknown categories surface
// as an error (not a panic) from the registry-miss path.
func TestRenderTable_Unregistered(t *testing.T) {
	t.Parallel()
	if _, _, err := RenderTable(domain.CategoryUnknown, nil, nil); err == nil {
		t.Error("RenderTable(CategoryUnknown): want error")
	}
}

// TestRenderTableForExport_ShortCircuit exercises the empty-env
// branch: with realm or region missing, the export path falls
// through to RenderTable (raw display output, no malformed OCIDs).
func TestRenderTableForExport_ShortCircuit(t *testing.T) {
	t.Parallel()
	items := []models.Tenant{
		{Name: "alpha", IDs: []string{"ocid1.tenancy.oc1..a"}},
	}
	_, exportRows, err := RenderTableForExport(domain.Tenant, items, "", "", nil)
	if err != nil {
		t.Fatalf("RenderTableForExport with empty env: %v", err)
	}
	_, plainRows, err := RenderTable(domain.Tenant, items, nil)
	if err != nil {
		t.Fatalf("RenderTable baseline: %v", err)
	}
	if !reflect.DeepEqual(exportRows, plainRows) {
		t.Errorf("short-circuit path should equal RenderTable; got %v vs %v", exportRows, plainRows)
	}
}

// TestRenderTableForExport_Grouped covers the grouped RenderForExport
// path for DAC, which is the canonical RenderForExport-bearing
// category (Name → full DAC OCID; Tenant → full tenancy OCID).
func TestRenderTableForExport_Grouped(t *testing.T) {
	t.Parallel()
	items := map[string][]models.DedicatedAICluster{
		"aaaaaaaatenant": {{Name: "amaaaaaadac", Status: "ACTIVE", TenantID: "aaaaaaaatenant"}},
	}
	_, rows, err := RenderTableForExport(domain.DedicatedAICluster, items, "oc1", "me-dubai-1", nil)
	if err != nil {
		t.Fatalf("RenderTableForExport: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows=%d", len(rows))
	}
	// Name column (row[0]) should be the fully-qualified DAC OCID.
	if rows[0][0] != "ocid1.generativeaidedicatedaicluster.oc1.me-dubai-1.amaaaaaadac" {
		t.Errorf("row[0][0] = %q, want full DAC OCID", rows[0][0])
	}
}

// TestRenderTableForExport_Unregistered covers the registry-miss
// error path on the export branch (separate code from RenderTable's
// equivalent, hence its own assertion).
func TestRenderTableForExport_Unregistered(t *testing.T) {
	t.Parallel()
	if _, _, err := RenderTableForExport(domain.CategoryUnknown, nil, "oc1", "iad", nil); err == nil {
		t.Error("RenderTableForExport(CategoryUnknown): want error")
	}
}

// TestHelpTable covers the (Key, Title) help output for both a
// registered and an unregistered category. Help is what `--columns
// help` shows in the CLI.
func TestHelpTable(t *testing.T) {
	t.Parallel()
	headers, rows := HelpTable(domain.Tenant)
	if !reflect.DeepEqual(headers, []string{"KEY", "TITLE"}) {
		t.Errorf("headers = %v, want [KEY TITLE]", headers)
	}
	if len(rows) == 0 {
		t.Error("HelpTable(Tenant): empty rows")
	}
	headers, rows = HelpTable(domain.CategoryUnknown)
	if headers != nil || rows != nil {
		t.Errorf("HelpTable(CategoryUnknown): want nil/nil, got %v/%v", headers, rows)
	}
}

// TestGroupedSet_Select covers the grouped variant of Select
// including the unknown-key error path. Mirrors the flat-Set
// equivalent that already had coverage.
func TestGroupedSet_Select(t *testing.T) {
	t.Parallel()
	type item struct {
		V string
	}
	g := GroupedSet[item]{Columns: []GroupedColumn[item]{
		{Title: "Key", Key: "key", Ratio: 0.5, Render: func(k string, _ item) string { return k }},
		{Title: "Val", Key: "val", Ratio: 0.5, Render: func(_ string, i item) string { return i.V }},
	}}
	got, err := g.Select([]string{"val"})
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	if len(got) != 1 || got[0].Key != "val" {
		t.Errorf("Select([val]): got %v", got)
	}
	// Unknown key surfaces via UnknownKeyError.Error.
	_, err = g.Select([]string{"bogus"})
	if err == nil {
		t.Fatal("Select([bogus]): want error")
	}
	if err.Error() == "" {
		t.Error("UnknownKeyError.Error: empty message")
	}
}

// TestRenderFlatExport mirrors TestRenderFlat for the export path:
// RenderForExport takes precedence over Render when set; columns
// without RenderForExport fall back to Render.
func TestRenderFlatExport(t *testing.T) {
	t.Parallel()
	type item struct {
		Name string
	}
	s := Set[item]{Columns: []Column[item]{
		{Title: "Plain", Key: "plain", Ratio: 0.5, Render: func(i item) string { return i.Name }},
		{
			Title: "Exported", Key: "exported", Ratio: 0.5,
			Render:          func(i item) string { return i.Name },
			RenderForExport: func(realm, region string, i item) string { return realm + "/" + region + "/" + i.Name },
		},
	}}
	items := []item{{"foo"}}
	_, rows, err := renderFlatForExport(s, items, "oc1", "iad", nil)
	if err != nil {
		t.Fatalf("renderFlatForExport: %v", err)
	}
	if rows[0][0] != "foo" || rows[0][1] != "oc1/iad/foo" {
		t.Errorf("rows[0] = %v, want [foo oc1/iad/foo]", rows[0])
	}
	// Wrong type surfaces as error, not panic.
	if _, _, err := renderFlatForExport(s, "not a slice", "oc1", "iad", nil); err == nil {
		t.Error("renderFlatForExport with wrong type: expected error")
	}
}

// TestRenderGroupedExport mirrors TestRenderGrouped for the export
// path: RenderForExport (with key + item) takes precedence over Render.
func TestRenderGroupedExport(t *testing.T) {
	t.Parallel()
	type item struct {
		V string
	}
	g := GroupedSet[item]{Columns: []GroupedColumn[item]{
		{Title: "Key", Key: "key", Ratio: 0.5, Render: func(k string, _ item) string { return k }},
		{
			Title: "Val", Key: "val", Ratio: 0.5,
			Render:          func(_ string, i item) string { return i.V },
			RenderForExport: func(realm, region, k string, i item) string { return realm + "/" + region + "/" + k + "/" + i.V },
		},
	}}
	items := map[string][]item{"g": {{V: "x"}}}
	_, rows, err := renderGroupedForExport(g, items, "oc1", "iad", nil)
	if err != nil {
		t.Fatalf("renderGroupedForExport: %v", err)
	}
	if rows[0][0] != "g" || rows[0][1] != "oc1/iad/g/x" {
		t.Errorf("rows[0] = %v", rows[0])
	}
	if _, _, err := renderGroupedForExport(g, []item{}, "oc1", "iad", nil); err == nil {
		t.Error("renderGroupedForExport with wrong type: expected error")
	}
}
