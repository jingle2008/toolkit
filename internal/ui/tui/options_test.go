package tui

import (
	"context"
	"fmt"
	"testing"

	"github.com/charmbracelet/bubbles/table"
	"github.com/stretchr/testify/assert"

	"github.com/jingle2008/toolkit/internal/domain"
	view "github.com/jingle2008/toolkit/internal/ui/tui/view"
	"github.com/jingle2008/toolkit/pkg/models"
)

func TestWithRepoPath(t *testing.T) {
	t.Parallel()
	m := &Model{}
	opt := WithRepoPath("foo/bar")
	opt(m)
	assert.Equal(t, "foo/bar", m.repoPath)
}

func TestWithKubeConfig(t *testing.T) {
	t.Parallel()
	m := &Model{}
	opt := WithKubeConfig("kube.yaml")
	opt(m)
	assert.Equal(t, "kube.yaml", m.kubeConfig)
}

func TestWithEnvironment(t *testing.T) {
	t.Parallel()
	m := &Model{}
	env := models.Environment{Region: "us-phoenix-1", Type: "dev", Realm: "realmA"}
	opt := WithEnvironment(env)
	opt(m)
	assert.Equal(t, env, m.environment)
}

func TestWithCategory(t *testing.T) {
	t.Parallel()
	m := &Model{}
	opt := WithCategory(domain.GpuPool)
	opt(m)
	assert.Equal(t, domain.GpuPool, m.category)
}

func TestWithTable(t *testing.T) {
	t.Parallel()
	m := &Model{}
	tbl := &table.Model{}
	opt := WithTable(tbl)
	opt(m)
	assert.Equal(t, tbl, m.table)
}

type mockRenderer struct{}

var _ view.Renderer = (*mockRenderer)(nil)

func (mockRenderer) RenderJSON(_ any, width int) (string, error) {
	return fmt.Sprintf("json: %d", width), nil
}

func TestWithContext(t *testing.T) {
	t.Parallel()
	m := &Model{}
	ctx := context.Background()
	opt := WithContext(ctx)
	opt(m)
	assert.Equal(t, ctx, m.parentCtx)
}

func TestWithFilter(t *testing.T) {
	t.Parallel()
	m := &Model{}
	opt := WithFilter("foo")
	opt(m)
	assert.Equal(t, "foo", m.newFilter)
}
