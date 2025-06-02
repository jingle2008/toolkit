package toolkit

import (
	"testing"

	"github.com/jingle2008/toolkit/pkg/models"
	"github.com/stretchr/testify/assert"
)

func Test_centerText_returns_centered_text(t *testing.T) {
	t.Parallel()
	result := centerText("hello", 10, 3)
	assert.Contains(t, result, "hello")
	assert.GreaterOrEqual(t, len(result), 10)
}

func Test_NewModel_initializes_fields(t *testing.T) {
	t.Parallel()
	env := models.Environment{Type: "dev", Region: "us-phoenix-1", Realm: "realmA"}
	m := NewModel(
		WithRepoPath("/repo"),
		WithKubeConfig("/kube"),
		WithEnvironment(env),
		WithCategory(Tenant),
	)
	assert.NotNil(t, m)
	assert.Equal(t, "/repo", m.repoPath)
	assert.Equal(t, "/kube", m.kubeConfig)
	assert.Equal(t, env, m.environment)
	assert.Equal(t, Tenant, m.category)
	assert.NotNil(t, m.table)
	assert.NotNil(t, m.textInput)
}

func Test_Model_contextString_and_infoView(t *testing.T) {
	t.Parallel()
	env := models.Environment{Type: "dev", Region: "us-phoenix-1", Realm: "realmA"}
	m := NewModel(
		WithRepoPath("/repo"),
		WithKubeConfig("/kube"),
		WithEnvironment(env),
		WithCategory(LimitTenancyOverride),
	)
	// Set context.Category to Tenant, m.category to LimitTenancyOverride
	m.context = &AppContext{Name: "scopeA", Category: Tenant}
	m.chosen = false
	cs := m.contextString()
	assert.Contains(t, cs, "Limit Tenancy Override")
	assert.Contains(t, cs, "scopeA")

	info := m.infoView()
	assert.Contains(t, info, "Realm:")
	assert.Contains(t, info, "Type:")
	assert.Contains(t, info, "Region:")
}

func Test_Model_statusView_renders(t *testing.T) {
	t.Parallel()
	env := models.Environment{Type: "dev", Region: "us-phoenix-1", Realm: "realmA"}
	m := NewModel(
		WithRepoPath("/repo"),
		WithKubeConfig("/kube"),
		WithEnvironment(env),
		WithCategory(Tenant),
	)
	m.viewWidth = 40
	m.viewHeight = 10
	status := m.statusView()
	assert.Contains(t, status, "Tenant")
	assert.Contains(t, status, "[1/")
}
