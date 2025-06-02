package toolkit

import (
	"testing"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/glamour"
	"github.com/jingle2008/toolkit/pkg/models"
	"github.com/stretchr/testify/assert"
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
	opt := WithCategory(GpuPool)
	opt(m)
	assert.Equal(t, GpuPool, m.category)
}

func TestWithAppContext(t *testing.T) {
	t.Parallel()
	m := &Model{}
	ctx := &AppContext{}
	opt := WithAppContext(ctx)
	opt(m)
	assert.Equal(t, ctx, m.context)
}

func TestWithViewSize(t *testing.T) {
	t.Parallel()
	m := &Model{}
	opt := WithViewSize(80, 24)
	opt(m)
	assert.Equal(t, 80, m.viewWidth)
	assert.Equal(t, 24, m.viewHeight)
}

func TestWithHelp(t *testing.T) {
	t.Parallel()
	m := &Model{}
	h := &help.Model{}
	opt := WithHelp(h)
	opt(m)
	assert.Equal(t, h, m.help)
}

func TestWithTable(t *testing.T) {
	t.Parallel()
	m := &Model{}
	tbl := &table.Model{}
	opt := WithTable(tbl)
	opt(m)
	assert.Equal(t, tbl, m.table)
}

func TestWithTextInput(t *testing.T) {
	t.Parallel()
	m := &Model{}
	ti := &textinput.Model{}
	opt := WithTextInput(ti)
	opt(m)
	assert.Equal(t, ti, m.textInput)
}

func TestWithViewport(t *testing.T) {
	t.Parallel()
	m := &Model{}
	vp := &viewport.Model{}
	opt := WithViewport(vp)
	opt(m)
	assert.Equal(t, vp, m.viewport)
}

func TestWithRenderer(t *testing.T) {
	t.Parallel()
	m := &Model{}
	r := &glamour.TermRenderer{}
	opt := WithRenderer(r)
	opt(m)
	assert.Equal(t, r, m.renderer)
}
