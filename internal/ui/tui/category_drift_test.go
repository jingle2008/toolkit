package tui

import (
	"context"
	"slices"
	"testing"

	"charm.land/bubbles/v2/table"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/pkg/models"
)

// kubeBackedCategories returns every category that loads from a live cluster.
func kubeBackedCategories() []domain.Category {
	var out []domain.Category
	for _, c := range domain.Categories() {
		if c.NeedsKubeConfig() {
			out = append(out, c)
		}
	}
	return out
}

// Every kube-backed category must load on its own, watch, and be
// reloadable — what the GPUWorkload bug missed when that took edits in
// several places; now it is one lazySources entry.
func TestKubeBackedCategories_FullyWired(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	for _, c := range kubeBackedCategories() {
		src, ok := lazySources[c]
		require.Truef(t, ok, "%s must have a lazySources entry", c)
		assert.Falsef(t, src.withDataset, "%s loads from the cluster, not with the dataset", c)
		assert.NotNilf(t, src.watch, "%s must watch", c)
		assert.NotNilf(t, m.reloadCategoryCmd(c, 1), "%s must be reloadable", c)
	}
}

// reloadCategoryCmd returns a command only for kube-backed categories.
func TestReloadCategoryCmd_OnlyKubeBacked(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	for _, c := range domain.Categories() {
		got := m.reloadCategoryCmd(c, 1)
		if c.NeedsKubeConfig() {
			assert.NotNilf(t, got, "%s is kube-backed and must reload", c)
		} else {
			assert.Nilf(t, got, "%s is not kube-backed and must not reload", c)
		}
	}
}

// startK8sWatchCmd must start a watch for every kube-backed category.
func TestStartK8sWatchCmd_CoversKubeBacked(t *testing.T) {
	t.Parallel()
	ld := &watchableLoader{}
	for _, c := range kubeBackedCategories() {
		cmd := startK8sWatchCmd(context.Background(), ld, c, "kc", models.Environment{}, 1)
		require.NotNilf(t, cmd, "%s watch cmd must be built", c)
		_, started := cmd().(k8sWatchStartedMsg)
		assert.Truef(t, started, "%s must produce k8sWatchStartedMsg, got unavailable", c)
	}
}

/*
TestUpdate_RoutesEveryKubeBackedLoadedMsg guards the route from a load
to its rows.

A loaded message Update doesn't route is dropped: applyLoaded never
runs, endTask never fires, pendingTasks stays elevated, and the spinner
ticks forever. That's how ServingRuntime once shipped broken, when each
category had its own message type in a hand-kept list.

This drives each category's real load command, takes the message it
actually produces, and asserts Update drains the task.
*/
func TestUpdate_RoutesEveryKubeBackedLoadedMsg(t *testing.T) {
	t.Parallel()
	for _, cat := range kubeBackedCategories() {
		t.Run(cat.String(), func(t *testing.T) {
			t.Parallel()
			m := newTestModel(t)
			cmd := m.reloadCategoryCmd(cat, m.gens.msg)
			require.NotNilf(t, cmd, "%s must have a load command", cat)

			msg := cmd()
			require.NotNilf(t, msg, "%s load command must produce a message", cat)

			m.pendingTasks = 1
			m.Update(msg)
			assert.Zerof(t, m.pendingTasks,
				"%s: Update dropped %T",
				cat, msg)
		})
	}
}

// noStatsCategories are intentionally without aggregate stat columns. A new
// category must be added here OR to statsColumns — never silently neither.
// Keep in sync with statsColumns (table_utils.go).
var noStatsCategories = map[domain.Category]struct{}{
	domain.Tenant:                          {},
	domain.InferenceService:                {},
	domain.LimitDefinition:                 {},
	domain.ConsolePropertyDefinition:       {},
	domain.PropertyDefinition:              {},
	domain.LimitTenancyOverride:            {},
	domain.ConsolePropertyTenancyOverride:  {},
	domain.PropertyTenancyOverride:         {},
	domain.LimitRegionalOverride:           {},
	domain.ConsolePropertyRegionalOverride: {},
	domain.PropertyRegionalOverride:        {},
	domain.BaseModel:                       {},
	domain.ServingRuntime:                  {},
	domain.ImportedModel:                   {},
	domain.ModelArtifact:                   {},
	domain.Environment:                     {},
	domain.ServiceTenancy:                  {},
	domain.Alias:                           {},
}

// Every category either has stat columns or is explicitly listed as having
// none — a new category fails until someone decides.
func TestStatsColumns_EveryCategoryAccountedFor(t *testing.T) {
	t.Parallel()
	for _, c := range domain.Categories() {
		_, hasStats := statsColumns[c]
		_, excluded := noStatsCategories[c]
		assert.Truef(t, hasStats != excluded,
			"%s must be in exactly one of statsColumns / noStatsCategories (hasStats=%v excluded=%v)",
			c, hasStats, excluded)
	}
}

// itemKeyFrom must produce a non-nil key for every category so selection works.
func TestItemKeyFrom_NonNilForEveryCategory(t *testing.T) {
	t.Parallel()
	row := table.Row{"a", "b", "c", "d"}
	for _, c := range domain.Categories() {
		assert.NotNilf(t, itemKeyFrom(c, row), "itemKeyFrom returned nil for %s", c)
	}
}

// Environment-scoped categories are exactly those an environment switch
// reloads: the ones with a load handler or in the tenancy-override group.
func TestEnvironmentScopedCategories_MatchLoadHandlers(t *testing.T) {
	t.Parallel()
	for _, c := range domain.Categories() {
		_, handled := lazySources[c]
		_, tenancy := tenancyOverrideCategories[c]
		assert.Equalf(t, handled || tenancy, c.IsEnvironmentScoped(),
			"%s: IsEnvironmentScoped disagrees with lazySources/tenancyOverrideCategories", c)
	}
}

// Tab must visit every data category exactly once before wrapping, and
// Shift-Tab must walk the same cycle backwards. Alias is the only category
// outside the cycle. Guards against the cycle bound lagging the enum (it
// once stopped at DedicatedAICluster and skipped ServingRuntime).
func TestCategoryCycle_VisitsEveryDataCategory(t *testing.T) {
	t.Parallel()
	var want []domain.Category
	for _, c := range domain.Categories() {
		if c != domain.Alias {
			want = append(want, c)
		}
	}

	var forward []domain.Category
	c := domain.Tenant
	for range len(domain.Categories()) + 1 {
		forward = append(forward, c)
		c = nextCategory(c)
		if c == domain.Tenant {
			break
		}
	}
	assert.Equal(t, want, forward, "Tab cycle")

	var backward []domain.Category
	c = domain.LastDataCategory
	for range len(domain.Categories()) + 1 {
		backward = append(backward, c)
		c = prevCategory(c)
		if c == domain.LastDataCategory {
			break
		}
	}
	slices.Reverse(backward)
	assert.Equal(t, want, backward, "Shift-Tab cycle")

	assert.Equal(t, domain.Tenant, nextCategory(domain.Alias), "Tab from Alias re-enters the cycle")
	assert.Equal(t, domain.LastDataCategory, prevCategory(domain.Alias), "Shift-Tab from Alias re-enters the cycle")
}
