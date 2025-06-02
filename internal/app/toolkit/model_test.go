package toolkit

import (
	"testing"

	"github.com/jingle2008/toolkit/internal/testutil"
	"github.com/jingle2008/toolkit/pkg/models"
)

func TestCenterTextReturnsCenteredText(t *testing.T) {
	t.Parallel()
	result := centerText("hello", 10, 3)
	testutil.Contains(t, result, "hello")
	testutil.GreaterOrEqual(t, len(result), 10)
}

func TestNewModelInitializesFields(t *testing.T) {
	t.Parallel()
	env := models.Environment{Type: "dev", Region: "us-phoenix-1", Realm: "realmA"}
	m := NewModel(
		WithRepoPath("/repo"),
		WithKubeConfig("/kube"),
		WithEnvironment(env),
		WithCategory(Tenant),
	)
	testutil.NotNil(t, m)
	testutil.Equal(t, "/repo", m.repoPath)
	testutil.Equal(t, "/kube", m.kubeConfig)
	testutil.Equal(t, env, m.environment)
	testutil.Equal(t, Tenant, m.category)
	testutil.NotNil(t, m.table)
	testutil.NotNil(t, m.textInput)
}

func TestModelContextStringAndInfoView(t *testing.T) {
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
	testutil.Contains(t, cs, "Limit Tenancy Override")
	testutil.Contains(t, cs, "scopeA")

	info := m.infoView()
	testutil.Contains(t, info, "Realm:")
	testutil.Contains(t, info, "Type:")
	testutil.Contains(t, info, "Region:")
}

func TestModelStatusViewRenders(t *testing.T) {
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
	testutil.Contains(t, status, "Tenant")
	testutil.Contains(t, status, "[1/")
}
