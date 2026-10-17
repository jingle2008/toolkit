package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/cli/output"
	"github.com/jingle2008/toolkit/internal/config"
	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/infra/loader"
	"github.com/jingle2008/toolkit/pkg/models"
)

// faultyLoader is relationsLoader with every endpoint ready, so the only
// faulty one is the orphan (its DAC is gone), and one node not OK.
type faultyLoader struct{ relationsLoader }

func (faultyLoader) LoadInferenceServices(context.Context, string, models.Environment) (map[string][]models.InferenceService, error) {
	return map[string][]models.InferenceService{
		"amaaaaaa-dac":  {{Name: "isvc-ok", Namespace: "amaaaaaa-dac", Type: models.InferenceServiceDedicated, Ready: "True"}},
		"amaaaaaa-gone": {{Name: "isvc-orphan", Namespace: "amaaaaaa-gone", Type: models.InferenceServiceDedicated, Ready: "True"}},
	}, nil
}

func emitFaultyNames(t *testing.T, ld loader.Composite, cat domain.Category, filter string) []string {
	t.Helper()
	var out bytes.Buffer
	err := emitFaulty(context.Background(), &out, ld, cat, relationsCfg(), models.Environment{Realm: "oc1"},
		filter, nil, 0, output.Options{Format: output.FormatJSON}, nil)
	require.NoError(t, err)
	var items []struct {
		Name string `json:"name"`
	}
	require.NoError(t, json.Unmarshal(out.Bytes(), &items), out.String())
	names := make([]string, len(items))
	for i, it := range items {
		names[i] = it.Name
	}
	return names
}

// --faulty lists the rows Ctrl+Z shows: a row naming a missing parent
// counts, though its own state is fine.
func TestEmitFaulty_DanglingRows(t *testing.T) {
	t.Parallel()
	assert.Equal(t, []string{"isvc-orphan"}, emitFaultyNames(t, faultyLoader{}, domain.InferenceService, ""))
	assert.Empty(t, emitFaultyNames(t, faultyLoader{}, domain.InferenceService, "isvc-ok"), "--filter still applies")
	// node-x is in an unknown pool; node-a isn't OK either (no status).
	assert.ElementsMatch(t, []string{"node-a", "node-x"}, emitFaultyNames(t, faultyLoader{}, domain.GPUNode, ""))
}

//nolint:paralleltest // swaps faultyDatasetFn
func TestEmitFaulty_PoolsAndLoadErrors(t *testing.T) {
	orig := faultyDatasetFn
	t.Cleanup(func() { faultyDatasetFn = orig })
	faultyDatasetFn = func(context.Context, loader.Composite, string, string, models.Environment, domain.Category) (*models.Dataset, []string, error) {
		return &models.Dataset{GPUPools: []models.GPUPool{
			{Name: "full", Size: 2, ActualSize: 2}, {Name: "short", Size: 2, ActualSize: 1},
		}}, []string{"some GPU pool sources failed to load"}, nil
	}
	assert.Equal(t, []string{"short"}, emitFaultyNames(t, emitLoader{}, domain.GPUPool, ""))

	faultyDatasetFn = func(context.Context, loader.Composite, string, string, models.Environment, domain.Category) (*models.Dataset, []string, error) {
		return nil, nil, errors.New("live GPU pool sizes unavailable")
	}
	err := emitFaulty(context.Background(), &bytes.Buffer{}, emitLoader{}, domain.GPUPool, relationsCfg(), models.Environment{},
		"", nil, 0, output.Options{Format: output.FormatJSON}, nil)
	require.ErrorContains(t, err, "--faulty: live GPU pool sizes unavailable")
}

func TestValidateFaulty(t *testing.T) {
	t.Parallel()
	kube := filepath.Join(t.TempDir(), "kubeconfig")
	require.NoError(t, os.WriteFile(kube, nil, 0o600))
	cfg := config.Config{KubeConfig: kube}
	require.NoError(t, validateFaulty(cfg, domain.Tenant, true))
	require.NoError(t, validateFaulty(config.Config{}, domain.LimitDefinition, false), "off: nothing to check")
	require.ErrorContains(t, validateFaulty(cfg, domain.LimitDefinition, true), "--faulty: LimitDefinition rows are never faulty; faulty applies to: basemodel")
	require.ErrorContains(t, validateFaulty(config.Config{}, domain.Tenant, true), "--faulty needs --kubeconfig")
	require.ErrorContains(t, validateFaulty(config.Config{KubeConfig: "/nonexistent"}, domain.Tenant, true), "not readable")
}
