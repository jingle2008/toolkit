package tui

import (
	"context"
	"testing"

	"github.com/charmbracelet/bubbles/table"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/pkg/models"
)

// kubeBackedCategories returns every category that loads from a live cluster.
func kubeBackedCategories() []domain.Category {
	var out []domain.Category
	for _, c := range domain.Categories {
		if c.NeedsKubeConfig() {
			out = append(out, c)
		}
	}
	return out
}

// Every kube-backed category must be lazy-loaded, have a load handler, and be
// reloadable/watchable — the cluster of planes the GPUWorkload bug missed.
func TestKubeBackedCategories_FullyWired(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	for _, c := range kubeBackedCategories() {
		_, lazy := lazyLoadedCategories[c]
		assert.Truef(t, lazy, "%s must be in lazyLoadedCategories", c)

		_, handled := categoryHandlers[c]
		assert.Truef(t, handled, "%s must have a categoryHandlers entry", c)

		assert.NotNilf(t, m.reloadCategoryCmd(c, 1), "%s must be reloadable", c)
	}
}

// reloadCategoryCmd returns a command only for kube-backed categories.
func TestReloadCategoryCmd_OnlyKubeBacked(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	for _, c := range domain.Categories {
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

// noStatsCategories are intentionally without aggregate stat columns. A new
// category must be added here OR to statsColumns — never silently neither.
// Keep in sync with statsColumns (table_utils.go).
var noStatsCategories = map[domain.Category]struct{}{
	domain.Tenant:                          {},
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
	for _, c := range domain.Categories {
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
	for _, c := range domain.Categories {
		assert.NotNilf(t, itemKeyFrom(c, row), "itemKeyFrom returned nil for %s", c)
	}
}

func TestParentScope_ResolvesForScopedCategories(t *testing.T) {
	t.Parallel()
	row := table.Row{"a", "b", "c", "d"}
	for _, c := range domain.Categories {
		// parentScope resolves a parent only for single-parent categories:
		// multi-parent categories (the tenancy overrides) can't disambiguate,
		// and categories with no parent have no scope. Every single-parent
		// category MUST resolve — a new one whose parent is not handled in
		// parentScope's switch fails here, surfacing the drift.
		if len(c.Parents()) != 1 {
			continue
		}
		_, ok := parentScope(c, row)
		assert.Truef(t, ok, "parentScope must resolve single-parent category %s (parent %s)", c, c.Parents()[0])
	}
}
