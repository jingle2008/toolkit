package tui

import (
	"slices"
	"strconv"
	"sync"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	"github.com/jingle2008/toolkit/pkg/models"
)

// currentSlotMarker tags the quick-switch entry for the active context.
const currentSlotMarker = " ●"

// quickSwitch jumps to the environment in the pressed digit's slot, staying
// in the current category. A no-op outside environment-scoped categories,
// for an empty slot, or for the slot already active.
func (m *Model) quickSwitch(msg tea.KeyMsg) tea.Cmd {
	if !m.category.IsEnvironmentScoped() {
		return nil
	}
	slot, err := strconv.Atoi(msg.String())
	if err != nil {
		return nil
	}
	env, ok := m.recentEnvs.Get(slot)
	if !ok || env.Equals(m.environment) {
		return nil
	}
	return m.switchEnvironment(env, m.category)
}

/*
switchEnvironment makes env current, drops the data loaded for the previous
one that doesn't carry over (tenants and overrides do, within a realm),
records env as most recently used, and loads target in the new context.

A scope naming a row of the old environment is cleared whatever the
target: navigation keeps a scope across the categories it scopes, so a DAC
picked before the switch would otherwise still filter :gw and :is after
it. A definition's scope, and a tenant's within the realm, still name a
row and are kept (scopeSurvives). env is remembered in a quick-switch slot
once its kube context is known to exist (handleKubeVersionMsg). When target is the current category the switch
happens in place: the visible rows belong to the old context, and the old
k8s watch dies with the load context updateCategoryCore cancels, so both
are cleared here too (a category change clears them on its own).
Results still in flight for the old environment are dropped on arrival
(inCurrentEnv).
*/
func (m *Model) switchEnvironment(env models.Environment, target domain.Category) tea.Cmd {
	sameRealm := env.Realm == m.environment.Realm
	m.environment = env
	switch {
	case m.dataset == nil:
	case sameRealm:
		m.dataset.ResetEnvironmentScopedFields()
	default:
		m.dataset.ResetRealmScopedFields()
	}
	m.k8sVersion = ""
	m.pendingRecent = &env // remembered once its context is known to exist
	if !scopeSurvives(m.scope, sameRealm) {
		m.scope = nil
	}
	m.landOn = nil
	m.dropStaleHistoryScopes(sameRealm)
	if target == m.category {
		m.watch.k8sActive = false
		m.watch.k8sTrigger = nil
		m.applyRows(nil, nil, false)
	}
	return tea.Batch(
		tea.Sequence(m.updateCategory(target)...),
		fetchKubeVersionCmd(m.kubeConfig, m.environment.KubeContext()),
	)
}

// scopeSurvives reports whether scope still names a row after an
// environment switch: a definition's rows come from the repo and hold in
// every environment, and a tenant's within its realm, whose tenants stay
// loaded. Any other scope (a DAC, a pool, a node) is a row of the old
// environment.
func scopeSurvives(scope *domain.Scope, sameRealm bool) bool {
	if scope == nil {
		return false
	}
	if !scope.Category.IsEnvironmentScoped() {
		return true
	}
	return sameRealm && scope.Category == domain.Tenant
}

// inCurrentEnv reports whether a result produced for env still applies.
// An action or reload can finish after the user switched environments,
// and row keys repeat across them (GPU pool names, node IPs), so applying
// it would mark or replace another environment's rows.
func (m *Model) inCurrentEnv(env models.Environment) bool {
	if env.Equals(m.environment) {
		return true
	}
	m.logger.Debugw("dropping result from a previous environment", "env", env.KubeContext(), "current", m.environment.KubeContext())
	return false
}

// quickSwitchBindings returns one help binding per occupied slot, in slot
// order, or nil when the quick-switch column should be hidden: outside list
// view, in environment-independent categories, or with nothing to switch to.
func (m *Model) quickSwitchBindings(mode common.ViewMode) []key.Binding {
	if mode != common.ListView || !m.category.IsEnvironmentScoped() || m.recentEnvs.Len() < 2 {
		return nil
	}
	entries := m.recentEnvs.BySlot()
	out := make([]key.Binding, 0, len(entries))
	for _, e := range entries {
		desc := e.Env.KubeContext()
		if e.Env.Equals(m.environment) {
			desc += currentSlotMarker
		}
		out = append(out, key.NewBinding(
			key.WithKeys(strconv.Itoa(e.Slot)),
			key.WithHelp(strconv.Itoa(e.Slot), desc),
		))
	}
	return out
}

// pruneRecentEnvs drops remembered environments the repo no longer declares.
// Skipped until environments are known, so a failed load can't wipe the
// list. The current environment is always kept.
func (m *Model) pruneRecentEnvs() tea.Cmd {
	if m.dataset == nil || len(m.dataset.Environments) == 0 {
		return nil
	}
	known := m.dataset.Environments
	changed := m.recentEnvs.Retain(func(e models.Environment) bool {
		if e.Equals(m.environment) || slices.ContainsFunc(known, e.Equals) {
			return true
		}
		m.logger.Debugw("dropping unknown recent environment", "realm", e.Realm, "env", e.GetName())
		return false
	})
	if !changed {
		return nil
	}
	return m.saveRecentEnvsCmd()
}

// recentSaver orders the async writes of the recent list: commands in a
// tea.Batch run concurrently, so without it an older snapshot could land
// after a newer one during rapid switching.
type recentSaver struct {
	mu      sync.Mutex
	issued  int // last sequence handed out (update loop only)
	written int // last sequence written (guarded by mu)
}

// saveRecentEnvsCmd persists a snapshot of the recent list off the update
// loop. Failures are logged only: losing quick-switch history is not worth
// interrupting the user.
func (m *Model) saveRecentEnvsCmd() tea.Cmd {
	if m.recentEnvsPath == "" {
		return nil
	}
	saver := &m.recentSaver
	saver.issued++
	seq := saver.issued
	snapshot := m.recentEnvs.Clone()
	path := m.recentEnvsPath
	logger := m.logger
	return func() tea.Msg {
		saver.mu.Lock()
		defer saver.mu.Unlock()
		if seq < saver.written {
			return nil
		}
		saver.written = seq
		if err := snapshot.Save(path); err != nil {
			logger.Warnw("failed to save recent contexts", "path", path, "error", err)
		}
		return nil
	}
}
