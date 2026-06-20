package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/infra/telemetry"
	"github.com/jingle2008/toolkit/pkg/models"
)

func TestMetricsURL_Build(t *testing.T) {
	t.Parallel()
	env := models.Environment{Realm: "oc1", Region: "me-abudhabi-1", Type: "prod"}
	got := metricsURL(env, telemetry.Filter{Key: telemetry.FilterDacId, Value: "ocid1.dac.oc1.me-abudhabi-1.x"}, telemetry.CapabilityChat, time.UnixMilli(1781832733444))
	require.True(t, strings.HasPrefix(got,
		"https://devops.oci.oraclecorp.com/telemetry/mql/explore?data="),
		"unexpected URL: %s", got)
	assert.Greater(t, len(got), 256)
}

func TestOpenMetrics_UnknownIsNoOp(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	assert.Nil(t, m.openMetrics("not a dac"))
	assert.Nil(t, m.openMetrics((*models.DedicatedAICluster)(nil)))
}

func TestOpenMetrics_DACReturnsCmd(t *testing.T) {
	t.Parallel()
	m := newTestModel(t)
	require.NotNil(t, m.openMetrics(&models.DedicatedAICluster{Name: "dac1"}))
}
