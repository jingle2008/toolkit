package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	keys "github.com/jingle2008/toolkit/internal/ui/tui/keys"
)

func contextHasParent(km keys.KeyMap) bool {
	for _, b := range km.Context {
		if b.Help().Desc == "Parent" {
			return true
		}
	}
	return false
}

func TestJumpToParent(t *testing.T) {
	t.Parallel()

	t.Run("drilled-in: uses the active scope and keeps it", func(t *testing.T) {
		t.Parallel()
		m := newTestModel(t)
		m.category = domain.DedicatedAICluster
		m.scope = &domain.Scope{Category: domain.Tenant, Name: "tenant1"}

		cmd := m.jumpToParent()

		require.NotNil(t, cmd, "should produce a navigation command")
		require.Equal(t, domain.Tenant, m.category, "should land on the parent category")
		require.NotNil(t, m.scope, "scope must persist so the parent row is re-selected")
		require.Equal(t, "tenant1", m.scope.Name)
	})

	t.Run("no scope: derives the parent Tenant from the selected row", func(t *testing.T) {
		t.Parallel()
		m := newTestModel(t)
		m.category = domain.DedicatedAICluster
		m.scope = nil
		m.rawRows = []table.Row{{"dac1", "tenant1"}} // row[1] is the parent tenant

		cmd := m.jumpToParent()

		require.NotNil(t, cmd)
		require.Equal(t, domain.Tenant, m.category)
		require.NotNil(t, m.scope, "scope is set so the parent row is auto-selected")
		require.Equal(t, domain.Tenant, m.scope.Category)
		require.Equal(t, "tenant1", m.scope.Name)
	})

	t.Run("no scope: regional override derives its definition", func(t *testing.T) {
		t.Parallel()
		m := newTestModel(t)
		m.category = domain.LimitRegionalOverride
		m.scope = nil
		m.rawRows = []table.Row{{"some-limit", "us-region"}} // row[0] is the definition

		cmd := m.jumpToParent()

		require.NotNil(t, cmd)
		require.Equal(t, domain.LimitDefinition, m.category)
		require.Equal(t, "some-limit", m.scope.Name)
	})

	t.Run("multi-parent: drilled-in tenancy override returns to the breadcrumb parent", func(t *testing.T) {
		t.Parallel()
		// Drilled in from a Definition — jump back to that Definition,
		// not the Tenant; the breadcrumb disambiguates the two parents.
		m := newTestModel(t)
		m.category = domain.LimitTenancyOverride
		m.scope = &domain.Scope{Category: domain.LimitDefinition, Name: "some-limit"}

		cmd := m.jumpToParent()

		require.NotNil(t, cmd)
		require.Equal(t, domain.LimitDefinition, m.category)
		require.Equal(t, "some-limit", m.scope.Name)
	})

	t.Run("multi-parent: no scope is a no-op (ambiguous parent)", func(t *testing.T) {
		t.Parallel()
		// Reached directly (no breadcrumb): a tenancy override has two
		// parents, so we don't guess — jump is a no-op.
		m := newTestModel(t)
		m.category = domain.LimitTenancyOverride
		m.scope = nil
		m.rawRows = []table.Row{{"o1", "tenantX"}}
		require.Nil(t, m.jumpToParent())
	})

	t.Run("no-op when the category has no parent", func(t *testing.T) {
		t.Parallel()
		m := newTestModel(t)
		m.category = domain.Tenant
		m.scope = nil
		m.rawRows = []table.Row{{"tenant1"}}
		require.Nil(t, m.jumpToParent())
	})
}

func TestParentScope(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		cat  domain.Category
		row  table.Row
		want domain.Scope
		ok   bool
	}{
		{"dac", domain.DedicatedAICluster, table.Row{"dac1", "tenant1"}, domain.Scope{Category: domain.Tenant, Name: "tenant1"}, true},
		{"imported model", domain.ImportedModel, table.Row{"m1", "tenant2"}, domain.Scope{Category: domain.Tenant, Name: "tenant2"}, true},
		// Tenancy overrides have two parents (Tenant + Definition), so
		// there is no unambiguous parent to derive without a breadcrumb.
		{"limit tenancy override (ambiguous parent)", domain.LimitTenancyOverride, table.Row{"o1", "tenantX"}, domain.Scope{}, false},
		{"gpu node", domain.GPUNode, table.Row{"node1", "poolA"}, domain.Scope{Category: domain.GPUPool, Name: "poolA"}, true},
		{"regional override", domain.LimitRegionalOverride, table.Row{"lim1", "r1"}, domain.Scope{Category: domain.LimitDefinition, Name: "lim1"}, true},
		{"no parent", domain.Tenant, table.Row{"t1"}, domain.Scope{}, false},
		{"grouped missing parent column", domain.DedicatedAICluster, table.Row{"dac1"}, domain.Scope{}, false},
		{"empty row", domain.GPUNode, table.Row{}, domain.Scope{}, false},
		{"gpu workload", domain.GPUWorkload, table.Row{"pod1", "node-a"}, domain.Scope{Category: domain.GPUNode, Name: "node-a"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := parentScope(tc.cat, tc.row)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestParentKey_DispatchesInListView(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.category = domain.DedicatedAICluster
	m.scope = &domain.Scope{Category: domain.Tenant, Name: "tenant1"}
	m.inputMode = common.NormalInput

	cmds := m.handleNormalKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})

	require.NotEmpty(t, cmds, "pressing o should dispatch a command")
	require.Equal(t, domain.Tenant, m.category, "o should jump to the parent category")
}

func TestGPUWorkloadKeys(t *testing.T) {
	t.Parallel()
	km := keys.ResolveKeys(domain.GPUWorkload, common.ListView)
	wantDescs := map[string]bool{"Parent": false, keys.SortPrefix + common.TenantCol: false}
	for _, b := range km.Context {
		if _, ok := wantDescs[b.Help().Desc]; ok {
			wantDescs[b.Help().Desc] = true
		}
	}
	for d, found := range wantDescs {
		if !found {
			t.Errorf("GPUWorkload list view missing binding %q", d)
		}
	}
}

func TestParentShortcut_OfferedInSubCategoriesOnly(t *testing.T) {
	t.Parallel()

	// Every sub-category advertises the Parent shortcut in list view.
	subCategories := []domain.Category{
		domain.DedicatedAICluster, domain.ImportedModel, domain.GPUNode,
		domain.LimitTenancyOverride, domain.ConsolePropertyTenancyOverride,
		domain.PropertyTenancyOverride, domain.LimitRegionalOverride,
		domain.ConsolePropertyRegionalOverride, domain.PropertyRegionalOverride,
		domain.GPUWorkload,
	}
	for _, c := range subCategories {
		require.True(t, contextHasParent(keys.ResolveKeys(c, common.ListView)),
			"%v should offer the Parent shortcut", c)
		// ... but never in details view, where "o" is Copy Object.
		require.False(t, contextHasParent(keys.ResolveKeys(c, common.DetailsView)),
			"%v should not advertise Parent in details view", c)
	}

	// Parentless categories do not advertise it.
	for _, c := range []domain.Category{domain.Tenant, domain.BaseModel, domain.GPUPool} {
		require.False(t, contextHasParent(keys.ResolveKeys(c, common.ListView)),
			"%v has no parent and should not offer the shortcut", c)
	}
}
