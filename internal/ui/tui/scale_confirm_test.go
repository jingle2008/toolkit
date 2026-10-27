package tui

import (
	"context"
	"log/slog"
	"testing"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/infra/oci"
	"github.com/jingle2008/toolkit/internal/logging"
	"github.com/jingle2008/toolkit/internal/ui/tui/common"
	"github.com/jingle2008/toolkit/pkg/models"
)

// newScaleModel is a GPU pool list holding pool, sized so the modal
// renders whole.
func newScaleModel(t *testing.T, pool models.GPUPool) (*Model, models.ItemKey, *logging.RingSink) {
	t.Helper()
	m := newConfirmTestModel(t)
	sink := logging.NewRingSink(16)
	m.logger = slog.New(sink)
	m.updateLayout(120, 40)
	m.category = domain.GPUPool
	m.dataset = &models.Dataset{GPUPools: []models.GPUPool{pool}}
	return m, itemKeyFrom(domain.GPUPool, []string{pool.Name}), sink
}

// The count starts at the pool's default: its shortfall, or one past a
// full pool, which the modal warns terraform apply undoes.
func TestConfirmScale_DefaultCount(t *testing.T) {
	t.Parallel()
	short := models.GPUPool{Name: "pool-a", ID: "p", Size: 4, ActualSize: 2, Status: "RUNNING"}
	m, key, _ := newScaleModel(t, short)
	m.requestScale(key, &short)
	require.Equal(t, common.ConfirmView, m.viewMode)
	assert.Equal(t, "2", m.confirm.Text())
	out := ansi.Strip(m.confirmView())
	assert.Contains(t, out, "Grows the instance pool from 2 to 4 instances.")
	assert.NotContains(t, out, "Terraform declares")

	full := models.GPUPool{Name: "pool-a", ID: "p", Size: 4, ActualSize: 4, Status: "RUNNING"}
	m, key, _ = newScaleModel(t, full)
	m.requestScale(key, &full)
	assert.Equal(t, "1", m.confirm.Text())
	out = ansi.Strip(m.confirmView())
	assert.Contains(t, out, "from 4 to 5 instances")
	assert.Contains(t, out, "Terraform declares 4;")
}

// A count past the one-step limit says so and can't be confirmed.
func TestConfirmScale_CountOverLimit(t *testing.T) {
	t.Parallel()
	pool := models.GPUPool{Name: "pool-a", ID: "p", Size: 4, ActualSize: 4, Status: "RUNNING"}
	m, key, _ := newScaleModel(t, pool)
	m.requestScale(key, &pool)
	m.updateConfirmView(keyPress("backspace"))
	for _, d := range "101" {
		m.updateConfirmView(keyPress(string(d)))
	}
	assert.Contains(t, ansi.Strip(m.confirmView()), "add 101 is more than 100")
	_, cmd := m.updateConfirmView(keyPress("enter"))
	assert.Nil(t, cmd, "an over-limit count doesn't scale")
	assert.Equal(t, common.ConfirmView, m.viewMode)
}

// Digits and pastes edit the count, and the notice follows; an invalid
// count blocks confirming.
func TestConfirmScale_EditCount(t *testing.T) {
	t.Parallel()
	pool := models.GPUPool{Name: "pool-a", ID: "p", Size: 4, ActualSize: 4, Status: "RUNNING"}
	m, key, _ := newScaleModel(t, pool)
	m.requestScale(key, &pool)

	m.updateConfirmView(keyPress("backspace"))
	assert.Contains(t, ansi.Strip(m.confirmView()), "Enter a whole number of instances, at least 1.")
	_, cmd := m.updateConfirmView(keyPress("y"))
	assert.Nil(t, cmd, "no count, no scale")
	_, cmd = m.updateConfirmView(keyPress("enter"))
	assert.Nil(t, cmd)
	assert.Equal(t, common.ConfirmView, m.viewMode)

	m.updateConfirmView(keyPress("1"))
	m.updateConfirmView(keyPress("2"))
	n, problem := scaleCount(pool, m.confirm.Text())
	require.Empty(t, problem)
	assert.Equal(t, 12, n)
	assert.Contains(t, ansi.Strip(m.confirmView()), "from 4 to 16 instances")

	m.updateConfirmView(keyPress("backspace"))
	m.updateConfirmView(keyPress("backspace"))
	m.updateConfirmView(tea.PasteMsg{Content: "3"})
	n, _ = scaleCount(pool, m.confirm.Text())
	assert.Equal(t, 3, n, "a paste edits the count")

	m.updateConfirmView(keyPress("x"))
	n, _ = scaleCount(pool, m.confirm.Text())
	assert.Equal(t, 3, n, "other keys don't type into it")
}

// The confirmed count is the one scaled by, and the audit records the
// count and the sizes OCI went from and to.
func TestConfirmScale_ConfirmedScalesByCount(t *testing.T) {
	t.Parallel()
	gotAdd := 0
	fakeIncreasePoolSize := func(_ context.Context, p *models.GPUPool, add int, _ models.Environment, _ *slog.Logger) (oci.PoolScale, error) {
		gotAdd = add
		return oci.PoolScale{Shown: p.ActualSize, Before: p.ActualSize, After: p.ActualSize + add}, nil
	}
	pool := models.GPUPool{Name: "pool-a", ID: "p", Size: 4, ActualSize: 4, Status: "RUNNING"}
	m, key, sink := newScaleModel(t, pool)
	m.deps.IncreasePoolSize = fakeIncreasePoolSize
	m.requestScale(key, &pool)
	m.updateConfirmView(keyPress("backspace"))
	m.updateConfirmView(keyPress("3"))

	_, cmd := m.updateConfirmView(keyPress("enter")) // the action button starts focused
	require.NotNil(t, cmd)
	msg, ok := actionResult(t, cmd).(gpuPoolScaleResultMsg)
	require.True(t, ok)
	require.NoError(t, msg.err)
	assert.Equal(t, 3, gotAdd)
	assert.Equal(t, oci.PoolScale{Shown: 4, Before: 4, After: 7}, msg.scale)
	assert.Equal(t, 4, msg.declared)

	entries := sink.Snapshot()
	require.NotEmpty(t, entries)
	done := entries[len(entries)-1].Fields
	assert.Subset(t, done, []any{"phase", "done", "add", int64(3), "from", int64(4), "to", int64(7)}) // slog normalizes ints
}

// A pool a reload removed while the modal was open is toasted as gone
// when the user confirms, not scaled or silently dropped.
func TestConfirmScale_PoolGoneByConfirm(t *testing.T) {
	t.Parallel()
	fakeIncreasePoolSize := func(context.Context, *models.GPUPool, int, models.Environment, *slog.Logger) (oci.PoolScale, error) {
		t.Error("must not scale")
		return oci.PoolScale{}, nil
	}
	pool := models.GPUPool{Name: "pool-a", ID: "p", Size: 4, ActualSize: 2, Status: "RUNNING"}
	m, key, _ := newScaleModel(t, pool)
	m.deps.IncreasePoolSize = fakeIncreasePoolSize
	m.requestScale(key, &pool)
	m.dataset = &models.Dataset{GPUPools: []models.GPUPool{{Name: "pool-b"}}}

	m.updateConfirmView(keyPress("enter"))
	assert.NotEqual(t, common.ConfirmView, m.viewMode)
	require.NotNil(t, m.toasts.Active())
	assert.Equal(t, toastWarn, m.toasts.Active().Sev)
	assert.Contains(t, m.toasts.Active().Msg, "pool-a is gone; nothing done")
}

// A pool with no instance pool ID can't be scaled: the modal doesn't
// open, and the toast says why.
func TestRequestScale_NoInstancePool(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		status, want string
	}{
		{models.GPUPoolStatusNonexistent, "pool-a has no OCI instance pool; nothing to scale"},
		{models.GPUPoolStatusPending, "pool-a's OCI state isn't loaded; refresh and retry"},
	} {
		pool := models.GPUPool{Name: "pool-a", Size: 4, Status: tc.status}
		m, key, _ := newScaleModel(t, pool)
		m.viewMode = common.ListView
		m.requestScale(key, &pool)
		assert.Equal(t, common.ListView, m.viewMode, tc.status)
		require.NotNil(t, m.toasts.Active(), tc.status)
		assert.Equal(t, tc.want, m.toasts.Active().Msg)
	}
}

// Actual Size is drawn in Warn only when past the Terraform size.
func TestToneOverSizePools(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	m.category = domain.GPUPool
	m.headers = []header{{text: "Name"}, {text: "Size"}, {text: "Actual Size"}}
	rows := []table.Row{{"over", "4", "6"}, {"at", "4", "4"}, {"short", "4", "2"}}
	m.toneOverSizePools(rows)
	warn := fgSeq(m.theme.Palette.Warn)
	assert.Equal(t, warn+"6\x1b[39m", rows[0][2])
	assert.Equal(t, "4", rows[1][2])
	assert.Equal(t, "2", rows[2][2])
}
