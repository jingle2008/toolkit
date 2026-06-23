package tui

import (
	"context"
	"testing"

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
