package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/pkg/models"
)

func TestDacMetricsURL(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	got := m.dacMetricsURL(&models.DedicatedAICluster{Name: "dac1"}, time.UnixMilli(1781832733444))
	require.True(t, strings.HasPrefix(got,
		"https://devops.oci.oraclecorp.com/telemetry/mql/explore?data="),
		"unexpected URL: %s", got)
	// The encoded dashboard payload makes the URL substantially longer than
	// the bare prefix; correctness of the payload itself is covered by the
	// telemetry package's tests.
	assert.Greater(t, len(got), 256)
}

func TestOpenDacMetrics_NonDACIsNoOp(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	assert.Nil(t, m.openDacMetrics("not a dac"))
	assert.Nil(t, m.openDacMetrics((*models.DedicatedAICluster)(nil)))
}

func TestOpenDacMetrics_DACReturnsCmd(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	// A DAC selection yields a launch command. The command is intentionally
	// not executed here — running it would call actions.OpenURL and spawn a
	// real browser.
	require.NotNil(t, m.openDacMetrics(&models.DedicatedAICluster{Name: "dac1"}))
}
