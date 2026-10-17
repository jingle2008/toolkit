package mcp

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/infra/loader"
	"github.com/jingle2008/toolkit/pkg/models"
)

func stubMCPMetricsTarget(t *testing.T, item any) {
	t.Helper()
	orig := mcpMetricsTargetFn
	t.Cleanup(func() { mcpMetricsTargetFn = orig })
	mcpMetricsTargetFn = func(context.Context, loader.Composite, string, string, models.Environment, domain.Category, string) (any, *models.Dataset, []string, error) {
		return item, &models.Dataset{}, nil, nil
	}
}

//nolint:paralleltest // replaces mcpMetricsTargetFn
func TestMetricsURL_DAC(t *testing.T) {
	stubMCPMetricsTarget(t, &models.DedicatedAICluster{Name: "amaaaaaadac1"})
	res, sc := callStructured(t, "metrics_url", map[string]any{"category": "dac", "name": "amaaaaaadac1"})
	require.False(t, res.IsError, resultText(t, res))
	assert.True(t, strings.HasPrefix(sc["url"].(string), "https://devops.oci.oraclecorp.com/telemetry/mql/explore?data=")) //nolint:forcetypeassert // url is a string
	assert.Equal(t, "chat", sc["capability"])
	assert.Equal(t, "DacId", sc["filterKey"])
	assert.Equal(t, "ocid1.generativeaidedicatedaicluster.oc1.iad.amaaaaaadac1", sc["filterValue"])
}

// As `toolkit metrics`: an imported model several DACs serve needs dac.
//
//nolint:paralleltest // replaces mcpMetricsTargetFn
func TestMetricsURL_SeveralDACs(t *testing.T) {
	stubMCPMetricsTarget(t, &models.ImportedModel{
		BaseModel: models.BaseModel{Name: "amaaaaaaim"},
		Namespace: "default", Copies: []string{"amaaaaaadac1", "amaaaaaadac2"},
	})
	res, _ := callStructured(t, "metrics_url", map[string]any{"category": "importedmodel", "name": "amaaaaaaim"})
	require.True(t, res.IsError)
	assert.Contains(t, resultText(t, res), "served by several DACs: amaaaaaadac1, amaaaaaadac2; pass dac")

	res, sc := callStructured(t, "metrics_url", map[string]any{"category": "importedmodel", "name": "amaaaaaaim", "dac": "amaaaaaadac2"})
	require.False(t, res.IsError, resultText(t, res))
	assert.Contains(t, sc["filterValue"], "amaaaaaadac2")
}
