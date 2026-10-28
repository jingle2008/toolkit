package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/table"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
	logging "github.com/jingle2008/toolkit/internal/logging"
	"github.com/jingle2008/toolkit/internal/recentenv"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	"github.com/jingle2008/toolkit/pkg/models"
)

var (
	envPHX = models.Environment{Type: "dev", Region: "us-phoenix-1", Realm: "oc1"}
	envIAD = models.Environment{Type: "prod", Region: "us-ashburn-1", Realm: "oc1"}
	envORD = models.Environment{Type: "dev", Region: "us-chicago-1", Realm: "oc1"}
)

// newQuickSwitchModel returns a model in envPHX with recent slots
// 0=envPHX, 1=envIAD, 2=envORD, browsing GPUNode in list view.
func newQuickSwitchModel(t *testing.T, path string) *Model {
	t.Helper()
	recent := &recentenv.List{}
	recent.Touch(envPHX)
	recent.Touch(envIAD)
	recent.Touch(envORD)
	recent.Touch(envPHX)
	m, err := NewModel(
		WithRepoPath("repo"),
		WithEnvironment(envPHX),
		WithLoader(fakeLoader{}),
		WithLogger(logging.Discard()),
		WithCategory(domain.GPUNode),
		WithRecentEnvs(recent, path),
	)
	require.NoError(t, err)
	m.dataset = &models.Dataset{Environments: []models.Environment{envPHX, envIAD, envORD}}
	m.viewWidth = 120
	m.viewHeight = 40
	return m
}

func helpKeysAndDescs(bindings []key.Binding) ([]string, []string) {
	ks := make([]string, 0, len(bindings))
	ds := make([]string, 0, len(bindings))
	for _, b := range bindings {
		ks = append(ks, b.Help().Key)
		ds = append(ds, b.Help().Desc)
	}
	return ks, ds
}

func TestQuickSwitchBindings_SlotOrderWithCurrentMarker(t *testing.T) {
	t.Parallel()
	m := newQuickSwitchModel(t, "")
	ks, ds := helpKeysAndDescs(m.quickSwitchBindings(common.ListView))
	assert.Equal(t, []string{"0", "1", "2"}, ks)
	assert.Equal(t, []string{"dp-dev-phx" + currentSlotMarker, "dp-prod-iad", "dp-dev-ord"}, ds)
}

func TestQuickSwitchBindings_Hidden(t *testing.T) {
	t.Parallel()
	t.Run("details view", func(t *testing.T) {
		t.Parallel()
		m := newQuickSwitchModel(t, "")
		assert.Nil(t, m.quickSwitchBindings(common.DetailsView))
	})
	t.Run("environment-independent category", func(t *testing.T) {
		t.Parallel()
		m := newQuickSwitchModel(t, "")
		m.category = domain.Environment
		assert.Nil(t, m.quickSwitchBindings(common.ListView))
	})
	t.Run("nothing to switch to", func(t *testing.T) {
		t.Parallel()
		m := newQuickSwitchModel(t, "")
		m.recentEnvs = &recentenv.List{}
		m.recentEnvs.Touch(envPHX)
		assert.Nil(t, m.quickSwitchBindings(common.ListView))
	})
}

func TestQuickSwitch_StaysInCategoryAndClearsScope(t *testing.T) {
	t.Parallel()
	m := newQuickSwitchModel(t, "")
	m.scope = &domain.Scope{Category: domain.GPUPool, Name: "pool-a"}
	m.watch.k8sActive = true
	m.k8sVersion = "v1.30.0"

	cmds := m.handleNormalKeys(keyPress("1"))
	require.Len(t, cmds, 1)
	assert.NotNil(t, cmds[0])

	assert.True(t, m.environment.Equals(envIAD))
	assert.Equal(t, domain.GPUNode, m.category)
	assert.Nil(t, m.scope)
	assert.False(t, m.watch.k8sActive)
	assert.Empty(t, m.k8sVersion)
	// Slots are stable: envIAD is now current but still in slot 1.
	_, ds := helpKeysAndDescs(m.quickSwitchBindings(common.ListView))
	assert.Equal(t, []string{"dp-dev-phx", "dp-prod-iad" + currentSlotMarker, "dp-dev-ord"}, ds)
}

func TestQuickSwitch_NoOps(t *testing.T) {
	t.Parallel()
	cases := map[string]func(m *Model) string{
		"current slot":  func(*Model) string { return "0" },
		"empty slot":    func(*Model) string { return "4" },
		"out of range":  func(*Model) string { return "6" },
		"env-agnostic":  func(m *Model) string { m.category = domain.LimitDefinition; return "1" },
		"alias browser": func(m *Model) string { m.category = domain.Alias; return "1" },
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			m := newQuickSwitchModel(t, "")
			k := setup(m)
			m.handleNormalKeys(keyPress(k))
			assert.True(t, m.environment.Equals(envPHX))
		})
	}
}

func TestQuickSwitch_DigitsGoToFilterInEditMode(t *testing.T) {
	t.Parallel()
	m := newQuickSwitchModel(t, "")
	m.enterFilterMode()
	m.handleKeyMsg(keyPress("1"))
	assert.True(t, m.environment.Equals(envPHX))
	assert.Equal(t, "1", m.textInput.Value())
}

func TestEnterContext_RecordsRecentEnv(t *testing.T) {
	t.Parallel()
	m := newQuickSwitchModel(t, "")
	envSEA := models.Environment{Type: "dev", Region: "uk-london-1", Realm: "oc1"}
	m.dataset.Environments = append(m.dataset.Environments, envSEA)
	m.category = domain.Environment
	m.table.SetColumns([]table.Column{{Title: "Name", Width: 10}})
	m.rawRows = []table.Row{{envSEA.GetName()}}
	m.table.SetRows(m.rawRows)

	require.NotNil(t, m.enterContext())
	assert.Equal(t, domain.Tenant, m.category)
	_, ok := m.recentEnvs.Get(3)
	require.False(t, ok, "remembered once its kube context is known to exist")

	m.handleKubeVersionMsg(kubeVersionMsg{Context: envSEA.KubeContext(), Version: "v1.32.10"})
	got, ok := m.recentEnvs.Get(3)
	require.True(t, ok)
	assert.True(t, got.Equals(envSEA))
}

// Switching to an environment the kubeconfig has no context for (the
// palette lists every repo environment) must not take a quick-switch slot
// from a reachable one.
func TestSwitchEnvironment_NoContextNotRemembered(t *testing.T) {
	t.Parallel()
	m := newQuickSwitchModel(t, "")
	before := m.recentEnvs.BySlot()
	envSEA := models.Environment{Type: "dev", Region: "uk-london-1", Realm: "oc1"}

	m.switchEnvironment(envSEA, m.category)
	m.handleKubeVersionMsg(kubeVersionMsg{Context: envSEA.KubeContext(), Version: "n/a", NoContext: true})
	assert.Equal(t, before, m.recentEnvs.BySlot())
}

// A scope still naming a row after the switch is kept: a definition's in
// any environment, a tenant's within the realm. A cluster row's is not.
func TestScopeSurvives(t *testing.T) {
	t.Parallel()
	def := &domain.Scope{Category: domain.LimitDefinition, Name: "l"}
	tenant := &domain.Scope{Category: domain.Tenant, Name: "acme"}
	dac := &domain.Scope{Category: domain.DedicatedAICluster, Name: "d"}
	assert.True(t, scopeSurvives(def, false))
	assert.True(t, scopeSurvives(tenant, true))
	assert.False(t, scopeSurvives(tenant, false))
	assert.False(t, scopeSurvives(dac, true))
	assert.False(t, scopeSurvives(nil, true))
}

func TestPruneRecentEnvs(t *testing.T) {
	t.Parallel()
	t.Run("drops unknown, keeps current", func(t *testing.T) {
		t.Parallel()
		m := newQuickSwitchModel(t, "")
		m.dataset.Environments = []models.Environment{envIAD}
		assert.Nil(t, m.pruneRecentEnvs(), "no path: nothing to save")
		assert.Equal(t, 2, m.recentEnvs.Len())
		_, ok := m.recentEnvs.Get(2)
		assert.False(t, ok, "envORD dropped")
		_, ok = m.recentEnvs.Get(0)
		assert.True(t, ok, "current envPHX kept")
	})
	t.Run("skipped before environments load", func(t *testing.T) {
		t.Parallel()
		m := newQuickSwitchModel(t, "")
		m.dataset.Environments = nil
		m.pruneRecentEnvs()
		assert.Equal(t, 3, m.recentEnvs.Len())
	})
}

func TestQuickSwitch_PersistsToFile(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "state.yaml")
	m := newQuickSwitchModel(t, path)
	m.recentEnvs.Touch(envIAD)
	cmd := m.saveRecentEnvsCmd()
	require.NotNil(t, cmd)
	cmd()

	got, err := recentenv.Load(path)
	require.NoError(t, err)
	assert.Equal(t, m.recentEnvs.BySlot(), got.BySlot())
}

func TestSaveRecentEnvsCmd_SkipsStaleSnapshot(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "state.yaml")
	m := newQuickSwitchModel(t, path)
	stale := m.saveRecentEnvsCmd()
	m.recentEnvs.Touch(envIAD)
	fresh := m.saveRecentEnvsCmd()
	fresh()
	stale()

	// The file lists entries most recently used first.
	data, err := os.ReadFile(path) //nolint:gosec // test temp file
	require.NoError(t, err)
	assert.Less(t, strings.Index(string(data), envIAD.Region), strings.Index(string(data), envPHX.Region),
		"stale snapshot (envPHX most recent) must not overwrite the fresh one")
}

func TestInit_RecordsStartupEnv(t *testing.T) {
	t.Parallel()
	m, err := NewModel(
		WithRepoPath("repo"),
		WithEnvironment(envIAD),
		WithLoader(fakeLoader{}),
		WithLogger(logging.Discard()),
	)
	require.NoError(t, err)
	_ = m.Init()
	got, ok := m.recentEnvs.Get(0)
	require.True(t, ok)
	assert.True(t, got.Equals(envIAD))
}

func TestFrame_QuickSwitchIsFirstHelpColumn(t *testing.T) {
	t.Parallel()
	m := newQuickSwitchModel(t, "")
	m.updateLayout(200, 40)
	km := m.keys
	km.QuickSwitch = m.quickSwitchBindings(common.ListView)
	full := km.FullHelp()
	require.NotEmpty(t, full)
	ks, _ := helpKeysAndDescs(full[0])
	assert.Equal(t, []string{"0", "1", "2"}, ks)
	assert.Contains(t, m.frame("", ""), "dp-prod-iad")
}

// Help capitalizes only the bubbles navigation descriptions; quick-switch
// contexts keep their names.
func TestHelp_SentenceCaseOnlyForNavigation(t *testing.T) {
	t.Parallel()
	m := newQuickSwitchModel(t, "")
	m.updateLayout(200, 40)
	m.viewStack = []common.ViewMode{common.ListView}
	var lines []string
	for _, sec := range m.helpSections() {
		lines = append(lines, sec...)
	}
	all := strings.Join(lines, "\n")
	assert.Contains(t, all, "dp-prod-iad")
	assert.NotContains(t, all, "Dp-prod-iad")
	assert.Contains(t, all, "Up")

	b := sentenceCase([]key.Binding{key.NewBinding(key.WithKeys("u"), key.WithHelp("u", "½ page up"))})
	assert.Equal(t, "½ page up", b[0].Help().Desc, "no letter to capitalize")
}
