package tui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
)

var escPress = tea.KeyPressMsg{Code: tea.KeyEscape}

func TestErrorToast_EscDismissesOnlyErrors(t *testing.T) {
	t.Parallel()
	m := newGoldenModel(t, 120, 40)
	showCategory(t, m, domain.GPUNode)
	m.showToast("drain failed", toastError)
	m.showToast("copied", toastInfo)

	m.Update(escPress)

	assert.False(t, m.hasErrorToast())
	require.NotNil(t, m.toasts.active)
	assert.Equal(t, "copied", m.toasts.active.msg, "the info toast stays until it expires")
	assert.Equal(t, common.ListView, m.viewMode)
	assert.Equal(t, domain.GPUNode, m.category, "esc dismissed the error instead of backing out")
}

func TestErrorToast_EscPassesThroughWhileTyping(t *testing.T) {
	t.Parallel()
	m := newGoldenModel(t, 120, 40)
	showCategory(t, m, domain.GPUNode)
	m.showToast("drain failed", toastError)
	m.inputMode = common.EditInput

	assert.False(t, m.interceptToastDismiss(escPress), "esc closes the input first")
	assert.True(t, m.hasErrorToast())
}

func TestErrorToast_EscWithoutErrorIsNotIntercepted(t *testing.T) {
	t.Parallel()
	m := newGoldenModel(t, 120, 40)
	showCategory(t, m, domain.GPUNode)
	m.showToast("copied", toastInfo)

	assert.False(t, m.interceptToastDismiss(escPress))
}

func TestErrorToast_WrapsAndSaysHowToDismiss(t *testing.T) {
	t.Parallel()
	m := newGoldenModel(t, 120, 40)
	m.showToast(strings.Repeat("word ", 60), toastError)

	box := ansi.Strip(m.renderToasts())
	lines := strings.Split(box, "\n")
	assert.Len(t, lines, maxErrorLines+3, "borders, %d message lines and the hint", maxErrorLines)
	assert.Contains(t, box, "…", "an over-long error is cut on its last line")
	assert.Contains(t, box, "esc dismiss · ` log")
}

// A failed load names r as the next step where the category can refresh.
func TestLoadError_NamesRetryWhereRefreshable(t *testing.T) {
	t.Parallel()
	for cat, want := range map[domain.Category]string{domain.GPUNode: "r retry", domain.Tenant: ""} {
		m := newGoldenModel(t, 120, 40)
		showCategory(t, m, cat)
		m.handleErrMsg(errMsg{err: errors.New("boom")})
		require.NotNil(t, m.toasts.active, cat)
		assert.Equal(t, want, m.toasts.active.next, cat)
		if want != "" {
			assert.Contains(t, ansi.Strip(m.renderToasts()), "r retry · esc dismiss")
		}
	}
}
