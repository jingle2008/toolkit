package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/jingle2008/toolkit/internal/cli/output"
	"github.com/jingle2008/toolkit/internal/columns"
	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/pkg/models"
)

// TestCSVSnapshots pins the canonical-column CSV output for every
// category against a fixed in-memory fixture. 12 categories preserve
// today's CLI behavior; 7 carry intentional diffs (3 widened tenancy
// overrides + 4 grouped reordered to name-first) — those snapshot
// files reflect the new canonical output.
//
// Run with UPDATE_SNAPSHOTS=1 to regenerate (e.g., when adding a
// column or after a deliberate behavior change).
func TestCSVSnapshots(t *testing.T) {
	t.Parallel()
	for _, cat := range domain.Categories {
		if cat == domain.CategoryUnknown {
			continue
		}
		t.Run(cat.String(), func(t *testing.T) {
			got := renderCanonicalCSV(t, cat)
			path := filepath.Join("testdata", "snapshots", cat.String()+".csv")
			if os.Getenv("UPDATE_SNAPSHOTS") == "1" {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read snapshot %s: %v (run with UPDATE_SNAPSHOTS=1 to seed)", path, err)
			}
			if string(want) != got {
				t.Errorf("%s csv changed (run UPDATE_SNAPSHOTS=1 if expected):\n--- want\n%s\n--- got\n%s",
					cat, want, got)
			}
		})
	}
}

// TestCSVSnapshotsExport pins the export-mode CSV output for the
// categories that declare ExportRender closures (DAC, ImportedModel
// today). Drives columns.RenderTableForExport with a fixed realm +
// region so the fully-qualified OCID format is recorded explicitly
// and any regression in ExportRender — for either of those columns
// or any future addition — fails this test at the snapshot level.
// Categories without ExportRender are skipped; their snapshots are
// already pinned by TestCSVSnapshots above.
func TestCSVSnapshotsExport(t *testing.T) {
	t.Parallel()
	const (
		realm  = "oc1"
		region = "me-dubai-1"
	)
	for _, cat := range []domain.Category{
		domain.DedicatedAICluster,
		domain.ImportedModel,
	} {
		t.Run(cat.String(), func(t *testing.T) {
			items := fixtureFor(t, cat)
			headers, rows, err := columns.RenderTableForExport(cat, items, realm, region, nil)
			if err != nil {
				t.Fatalf("RenderTableForExport(%s): %v", cat, err)
			}
			var buf bytes.Buffer
			if err := output.WriteDelimited(&buf, headers, rows, output.Options{}, ','); err != nil {
				t.Fatalf("WriteDelimited: %v", err)
			}
			got := buf.String()

			path := filepath.Join("testdata", "snapshots", cat.String()+".export.csv")
			if os.Getenv("UPDATE_SNAPSHOTS") == "1" {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read snapshot %s: %v (run with UPDATE_SNAPSHOTS=1 to seed)", path, err)
			}
			if string(want) != got {
				t.Errorf("%s export csv changed (run UPDATE_SNAPSHOTS=1 if expected):\n--- want\n%s\n--- got\n%s",
					cat, want, got)
			}
		})
	}
}

// renderCanonicalCSV drives columns.RenderTable directly with a
// per-category fixture, then csv-encodes the result. Bypasses the
// CLI's runGet loader so the snapshot is deterministic.
func renderCanonicalCSV(t *testing.T, cat domain.Category) string {
	t.Helper()
	items := fixtureFor(t, cat)
	headers, rows, err := columns.RenderTable(cat, items, nil)
	if err != nil {
		t.Fatalf("RenderTable(%s): %v", cat, err)
	}
	var buf bytes.Buffer
	if err := output.WriteDelimited(&buf, headers, rows, output.Options{}, ','); err != nil {
		t.Fatalf("WriteDelimited: %v", err)
	}
	return buf.String()
}

// fixtureFor returns a small, deterministic typed payload for cat.
// Flat categories get a single-item slice; grouped categories get a
// single-key map with one item.
//
//nolint:cyclop // a per-category switch is the contract here
func fixtureFor(t *testing.T, cat domain.Category) any {
	t.Helper()
	switch cat { //nolint:exhaustive
	case domain.Tenant:
		return []models.Tenant{{Name: "alpha", IDs: []string{"ocid1.tenancy.oc1..a"}, IsInternal: true, Note: "n/a"}}
	case domain.LimitDefinition:
		return []models.LimitDefinition{{Name: "compute-cores", Description: "max OCPUs", Scope: "TENANT", DefaultMin: "0", DefaultMax: "200"}}
	case domain.ConsolePropertyDefinition:
		return []models.ConsolePropertyDefinition{{Name: "dark-mode", Description: "Enable dark mode", Value: "false"}}
	case domain.PropertyDefinition:
		return []models.PropertyDefinition{{Name: "timeout", Description: "Request timeout", DefaultValue: "30s"}}
	case domain.LimitRegionalOverride:
		return []models.LimitRegionalOverride{{Name: "compute-cores", Regions: []string{"us-ashburn-1", "us-phoenix-1"}, Values: []models.LimitRange{{Min: 0, Max: 50}}}}
	case domain.ConsolePropertyRegionalOverride:
		return []models.ConsolePropertyRegionalOverride{{Name: "dark-mode", Regions: []string{"us-ashburn-1"}, Values: []struct {
			Value string `json:"value"`
		}{{Value: "true"}}}}
	case domain.PropertyRegionalOverride:
		return []models.PropertyRegionalOverride{{Name: "timeout", Regions: []string{"us-ashburn-1"}, Values: []struct {
			Value string `json:"value"`
		}{{Value: "60s"}}}}
	case domain.LimitTenancyOverride:
		return map[string][]models.LimitTenancyOverride{
			"tenant-1": {{LimitRegionalOverride: models.LimitRegionalOverride{Name: "compute-cores", Regions: []string{"us-ashburn-1"}, Values: []models.LimitRange{{Min: 5, Max: 100}}}}},
		}
	case domain.ConsolePropertyTenancyOverride:
		return map[string][]models.ConsolePropertyTenancyOverride{
			"tenant-1": {{ConsolePropertyRegionalOverride: models.ConsolePropertyRegionalOverride{Name: "dark-mode", Regions: []string{"us-ashburn-1"}, Values: []struct {
				Value string `json:"value"`
			}{{Value: "true"}}}}},
		}
	case domain.PropertyTenancyOverride:
		return map[string][]models.PropertyTenancyOverride{
			"tenant-1": {{PropertyRegionalOverride: models.PropertyRegionalOverride{Name: "timeout", Regions: []string{"us-ashburn-1"}, Values: []struct {
				Value string `json:"value"`
			}{{Value: "45s"}}}}},
		}
	case domain.BaseModel:
		return []models.BaseModel{{
			Name: "cohere.command", InternalName: "command-r", Vendor: "cohere",
			Type: "TEXT", Version: "1.0", Status: "ACTIVE",
			DisplayName: "Command R", ParameterSize: "35B", MaxTokens: 4096,
		}}
	case domain.ImportedModel:
		// TenantID matches the map key — that's what the k8s loader
		// produces (tenancy-id label value drives both grouping and
		// the struct field), and the export snapshot relies on it
		// so the Tenant column's full OCID is realistic.
		return map[string][]models.ImportedModel{
			"tenant-1": {{BaseModel: models.BaseModel{Name: "my-model", DisplayName: "My Model", Vendor: "acme", Version: "v1", Status: "READY"}, Namespace: "ns1", TenantID: "tenant-1"}},
		}
	case domain.ModelArtifact:
		return map[string][]models.ModelArtifact{
			"cohere.command": {{Name: "artifact-v1", ModelName: "cohere.command", GpuCount: 8, GpuShape: "BM.GPU.H100.8", TensorRTVersion: "8.6.1"}},
		}
	case domain.Environment:
		return []models.Environment{{Type: "prod", Region: "us-ashburn-1", Realm: "oc1"}}
	case domain.ServiceTenancy:
		return []models.ServiceTenancy{{Name: "svc-prod", Realm: "oc1", Environment: "prod", HomeRegion: "us-ashburn-1", Regions: []string{"us-ashburn-1", "us-phoenix-1"}}}
	case domain.GpuPool:
		return []models.GpuPool{{Name: "pool-1", Shape: "BM.GPU.H100.8", Size: 4, ActualSize: 3, CapacityType: "OnDemand", Status: "ACTIVE", AvailabilityDomain: "AD-1", IsOkeManaged: true}}
	case domain.GpuNode:
		return map[string][]models.GpuNode{
			"pool-A": {{Name: "node-1", NodePool: "pool-A", InstanceType: "BM.GPU.H100.8", Allocatable: 8, Allocated: 3, IsReady: true, Age: "1d"}},
		}
	case domain.DedicatedAICluster:
		// TenantID matches the map key — same convention as
		// ImportedModel; the k8s loader keys by dac.TenantID.
		return map[string][]models.DedicatedAICluster{
			"tenant-1": {{Name: "dac-1", TenantID: "tenant-1", Type: "HOSTING", ModelName: "cohere.command", UnitShape: "AI.LARGE", Size: 4, Age: "1d", Status: "ACTIVE"}},
		}
	case domain.Alias:
		cats := make([]domain.Category, 0, len(domain.Categories))
		for _, c := range domain.Categories {
			if c != domain.CategoryUnknown {
				cats = append(cats, c)
			}
		}
		return cats
	}
	t.Fatalf("fixtureFor: unhandled category %s", cat)
	return nil
}
