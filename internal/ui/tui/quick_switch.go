package tui

import (
	"fmt"
	"slices"
	"strconv"
	"sync"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

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
switchEnvironment makes env current, drops the realm-scoped data loaded for
the previous one, records env as most recently used, and loads target in the
new context.

The scope always names a row of the old environment, so it's cleared
whatever the target: navigation keeps a scope across the categories it
scopes, so a DAC picked before the switch would otherwise still filter
:gw and :is after it. When target is the current category the switch
happens in place: the visible rows belong to the old context, and the old
k8s watch dies with the load context updateCategoryCore cancels, so both
are cleared here too (a category change clears them on its own).
Results still in flight for the old environment are dropped on arrival
(inCurrentEnv).
*/
func (m *Model) switchEnvironment(env models.Environment, target domain.Category) tea.Cmd {
	m.environment = env
	if m.dataset != nil {
		m.dataset.ResetRealmScopedFields()
	}
	m.k8sVersion = ""
	m.recentEnvs.Touch(env)
	m.scope = nil
	if target == m.category {
		m.watch.k8sActive = false
		m.watch.k8sTrigger = nil
		m.applyRows(nil, nil, false)
	}
	return tea.Batch(
		tea.Sequence(m.updateCategory(target)...),
		fetchKubeVersionCmd(m.kubeConfig, m.environment.KubeContext()),
		m.saveRecentEnvsCmd(),
	)
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
			key.WithHelp(fmt.Sprintf("<%d>", e.Slot), desc),
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
