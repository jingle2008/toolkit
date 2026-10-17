package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/config"
	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/infra/loader"
	"github.com/jingle2008/toolkit/pkg/models"
)

func stubMetricsTarget(t *testing.T, item any) {
	t.Helper()
	orig := metricsTargetFn
	t.Cleanup(func() { metricsTargetFn = orig })
	metricsTargetFn = func(context.Context, loader.Composite, string, string, models.Environment, domain.Category, string) (any, *models.Dataset, []string, error) {
		return item, &models.Dataset{}, nil, nil
	}
}

func runMetrics(t *testing.T, cat domain.Category, dac string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	env := models.Environment{Realm: "oc1", Region: "me-abudhabi-1", Type: "prod"}
	err := emitMetrics(context.Background(), &out, emitLoader{}, config.Config{}, env, cat, "x", dac, time.UnixMilli(1781832733444))
	return out.String(), err
}

//nolint:paralleltest // swaps metricsTargetFn
func TestEmitMetrics_PrintsDashboardURL(t *testing.T) {
	stubMetricsTarget(t, &models.DedicatedAICluster{Name: "amaaaaaadac1"})
	out, err := runMetrics(t, domain.DedicatedAICluster, "")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(out, "https://devops.oci.oraclecorp.com/telemetry/mql/explore?data="), out)
}

// An imported model several DACs serve needs --dac, as the TUI needs a
// pick.
//
//nolint:paralleltest // swaps metricsTargetFn
func TestEmitMetrics_SeveralDACsNeedsDAC(t *testing.T) {
	stubMetricsTarget(t, &models.ImportedModel{
		BaseModel: models.BaseModel{Name: "amaaaaaaim"},
		Namespace: "default", Copies: []string{"amaaaaaadac1", "amaaaaaadac2"},
	})
	_, err := runMetrics(t, domain.ImportedModel, "")
	require.ErrorContains(t, err, "served by several DACs: amaaaaaadac1, amaaaaaadac2; pass --dac <namespace>")

	out, err := runMetrics(t, domain.ImportedModel, "amaaaaaadac2")
	require.NoError(t, err)
	assert.Contains(t, out, "https://")
}

//nolint:paralleltest // swaps metricsTargetFn
func TestEmitMetrics_NoDashboardReason(t *testing.T) {
	stubMetricsTarget(t, &models.GPUWorkload{Name: "pod"})
	_, err := runMetrics(t, domain.GPUWorkload, "")
	require.ErrorContains(t, err, "workload has no model")
}
