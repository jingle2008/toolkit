package tui

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/mutation"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"
	logging "github.com/jingle2008/toolkit/pkg/infra/logging"
	"github.com/jingle2008/toolkit/pkg/models"
)

// The golden snapshots lock what every view renders, ANSI stripped, so a
// change that moves a single cell shows up as a diff. Regenerate with
//
//	go test ./internal/ui/tui/ -run TestGolden -update
//
// and review the diff: an unintended change is a regression.
var updateGolden = flag.Bool("update", false, "rewrite golden files")

// logClock masks the log overlay's wall-clock timestamps, which the ring
// sink stamps at record time.
var logClock = regexp.MustCompile(`\b\d{2}:\d{2}:\d{2}\b`)

// dirStat masks the export picker's mode and size columns, which vary by
// OS and filesystem (64B on APFS, 4.0K on ext4).
var dirStat = regexp.MustCompile(`d[rwx-]{9}\s+\S+ `)

// assertGolden compares the ANSI-stripped render against
// testdata/golden/<name>.txt, rewriting it under -update.
func assertGolden(t *testing.T, name, got string) {
	t.Helper()
	got = normalizeRender(got)
	path := filepath.Join("testdata", "golden", name+".txt")
	if *updateGolden {
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
		require.NoError(t, os.WriteFile(path, []byte(got), 0o600))
		return
	}
	want, err := os.ReadFile(path) //nolint:gosec // path is built from the test-owned golden name
	require.NoError(t, err, "missing golden; run with -update")
	require.Equal(t, string(want), got, "render differs from %s", path)
}

// normalizeRender strips ANSI and trailing spaces so snapshots compare
// layout, not colors or padding that has no visible effect.
func normalizeRender(s string) string {
	lines := strings.Split(ansi.Strip(s), "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " ")
	}
	return strings.Join(lines, "\n") + "\n"
}

var goldenSizes = []struct{ w, h int }{{80, 24}, {120, 40}, {200, 50}}

// newGoldenModel builds a deterministic model: fixed env, version and k8s
// version, and one row set per snapshotted category, including a faulty
// GPU node and a DAC whose name has wide runes.
func newGoldenModel(t *testing.T, w, h int) *Model {
	t.Helper()
	ring := logging.NewRingSink(200)
	env := models.Environment{Realm: "oc1", Type: "prod", Region: "us-ashburn-1"}
	m, err := NewModel(
		WithRepoPath("testrepo"),
		WithEnvironment(env),
		WithLoader(fakeLoader{}),
		WithLogger(logging.NewNoOpLogger()),
		WithLogStore(ring),
		WithVersion("v1.2.3"),
	)
	require.NoError(t, err)
	m.k8sVersion = "v1.31.4"

	acme := models.Tenant{Name: "acme", ID: "ocid1.tenancy.oc1..acme", Status: "ACTIVE"}
	m.dataset = &models.Dataset{
		Environments: []models.Environment{env},
		Tenants: []models.Tenant{
			acme,
			{Name: "globex", ID: "ocid1.tenancy.oc1..globex", IsInternal: true, Status: "ACTIVE"},
			{Name: "initech", ID: "ocid1.tenancy.oc1..initech", Status: "DELETING"},
		},
		DedicatedAIClusterMap: map[string][]models.DedicatedAICluster{
			"acme": {
				{Name: "dac-alpha", Status: "Active", TenantID: "acme", Type: "Hosting", UnitShape: "A100_80G", Size: 4, Owner: &acme, TotalReplicas: 4, IdleReplicas: 1, Age: "12d"},
				{Name: "dac-日本語", Status: "Failed", TenantID: "acme", Type: "Fine-tuning", UnitShape: "H100", Size: 2, Owner: &acme, TotalReplicas: 2, Age: "3d"},
			},
			"orphan": {
				{Name: "dac-orphan", Status: "Active", TenantID: "orphan", Type: "Hosting", UnitShape: "A10", Size: 1, TotalReplicas: 1, IdleReplicas: 1, Age: "41d"},
			},
		},
		GPUNodeMap: map[string][]models.GPUNode{
			"pool-a": {
				{Name: "10.0.0.11", InstanceType: "BM.GPU.H100.8", NodePool: "pool-a", Allocatable: 8, Allocated: 6, IsReady: true, Age: "90d"},
				{Name: "10.0.0.12", InstanceType: "BM.GPU.H100.8", NodePool: "pool-a", Allocatable: 8, Allocated: 0, IsReady: false, IsSchedulingDisabled: true, Age: "90d", Issues: []string{"NotReady"}},
			},
		},
	}
	for i := range 60 {
		ring.Infow(fmt.Sprintf("loaded category %d", i), "rows", i*3)
	}
	m.updateLayout(w, h)
	return m
}

// showCategory switches the golden model to cat's list view.
func showCategory(t *testing.T, m *Model, cat domain.Category) {
	t.Helper()
	m.category = cat
	m.viewMode = common.ListView
	m.keys = keys.ResolveKeys(cat, common.ListView)
	m.updateColumns()
	m.updateRows(true)
	m.updateLayout(m.viewWidth, m.viewHeight)
	require.NotEmpty(t, m.rawRows, "no rows for %s", cat)
}

// showDetails opens the details view on the first row and renders its
// body synchronously.
func showDetails(t *testing.T, m *Model) {
	t.Helper()
	showCategory(t, m, domain.DedicatedAICluster)
	require.NotNil(t, m.enterDetailView())
	msg, ok := m.updateContentAsync()().(detailContentRenderedMsg)
	require.True(t, ok)
	m.handleDetailContentRenderedMsg(msg)
}

// showExport opens the export popup over a fixed directory tree.
func showExport(t *testing.T, m *Model) {
	t.Helper()
	showCategory(t, m, domain.DedicatedAICluster)
	dir := t.TempDir()
	for _, d := range []string{"alpha", "beta", "gamma"} {
		require.NoError(t, os.Mkdir(filepath.Join(dir, d), 0o750))
	}
	m.dirPicker.CurrentDirectory = dir
	m.enterExportView()
	updated, _ := m.dirPicker.Update(m.dirPicker.Init()())
	m.dirPicker = &updated
}

var goldenViews = []struct {
	name  string
	setup func(t *testing.T, m *Model)
}{
	{"list-dac", func(t *testing.T, m *Model) { showCategory(t, m, domain.DedicatedAICluster) }},
	{"list-gpunode", func(t *testing.T, m *Model) { showCategory(t, m, domain.GPUNode) }},
	{"list-tenant", func(t *testing.T, m *Model) { showCategory(t, m, domain.Tenant) }},
	{"details", showDetails},
	{"loading", func(t *testing.T, m *Model) {
		showCategory(t, m, domain.DedicatedAICluster)
		m.viewMode = common.LoadingView
	}},
	{"help-from-list", func(t *testing.T, m *Model) {
		showCategory(t, m, domain.GPUNode)
		m.enterHelpView()
	}},
	{"help-from-details", func(t *testing.T, m *Model) {
		showDetails(t, m)
		m.enterHelpView()
	}},
	{"export", showExport},
	{"edit-tenant", func(t *testing.T, m *Model) {
		showCategory(t, m, domain.DedicatedAICluster)
		orphan := &m.dataset.DedicatedAIClusterMap["orphan"][0]
		m.openTenantForm(orphan)
		require.Equal(t, common.EditTenantView, m.viewMode)
	}},
	{"log-following", func(t *testing.T, m *Model) {
		showCategory(t, m, domain.DedicatedAICluster)
		m.viewMode = common.LogView
	}},
	{"log-paused", func(t *testing.T, m *Model) {
		showCategory(t, m, domain.DedicatedAICluster)
		m.viewMode = common.LogView
		_ = m.render() // first render follows the tail
		m.log.viewport.SetYOffset(0)
	}},
	{"confirm-recoverable", func(t *testing.T, m *Model) {
		showCategory(t, m, domain.GPUNode)
		m.requestConfirm(confirmOverlay{act: mutation.DrainNode, target: "10.0.0.11", run: nopRun})
	}},
	{"confirm-irreversible", func(t *testing.T, m *Model) {
		showCategory(t, m, domain.DedicatedAICluster)
		m.requestConfirm(confirmOverlay{
			act: mutation.DeleteDAC, target: "dac-alpha", run: nopRun,
			warning: "This permanently deletes the cluster and its endpoints.",
		})
	}},
	{"row-picker", func(t *testing.T, m *Model) {
		showCategory(t, m, domain.DedicatedAICluster)
		m.openRowPicker("Go to", "go", []domain.Scope{
			{Category: domain.Tenant, Name: "acme"},
			{Category: domain.BaseModel, Name: "llama-3.1-70b"},
			{Category: domain.ServingRuntime, Name: "vllm"},
		}, nil)
		m.rowPicker.cursor = 1
	}},
	{"toast-error", func(t *testing.T, m *Model) {
		showCategory(t, m, domain.DedicatedAICluster)
		m.showToast("render failed: boom", toastError)
	}},
}

func nopRun() tea.Cmd { return nil }

func TestGolden(t *testing.T) {
	t.Parallel()
	for _, v := range goldenViews {
		for _, sz := range goldenSizes {
			name := fmt.Sprintf("%s-%dx%d", v.name, sz.w, sz.h)
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				m := newGoldenModel(t, sz.w, sz.h)
				v.setup(t, m)
				out := ansi.Strip(m.render())
				switch {
				case strings.HasPrefix(v.name, "log-"):
					out = logClock.ReplaceAllString(out, "hh:mm:ss")
				case v.name == "export":
					out = dirStat.ReplaceAllString(out, "drwxr-xr-x  <size> ")
				}
				assertGolden(t, name, out)
			})
		}
	}
}

// TestGolden_HelpShortTerminal documents help on a short terminal, where
// today's single-column help is truncated.
func TestGolden_HelpShortTerminal(t *testing.T) {
	t.Parallel()
	m := newGoldenModel(t, 80, 15)
	showCategory(t, m, domain.GPUNode)
	m.enterHelpView()
	assertGolden(t, "help-from-list-80x15", m.render())
}
