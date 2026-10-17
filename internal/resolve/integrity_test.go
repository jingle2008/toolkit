//nolint:paralleltest // FaultyDataset tests swap the shared enrichGPUPoolsFn seam and must run sequentially
package resolve

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/infra/terraform"
	"github.com/jingle2008/toolkit/pkg/models"
)

func stubEnrich(t *testing.T, fn func([]models.GPUPool) error) *int {
	t.Helper()
	calls := 0
	orig := enrichGPUPoolsFn
	t.Cleanup(func() { enrichGPUPoolsFn = orig })
	enrichGPUPoolsFn = func(_ context.Context, pools []models.GPUPool, _ string, _ models.Environment) error {
		calls++
		return fn(pools)
	}
	return &calls
}

// GPU pools are enriched with their live sizes, which is what makes a
// pool faulty; other categories skip the OCI call.
func TestFaultyDataset_EnrichesPoolsOnlyForGPUPool(t *testing.T) {
	calls := stubEnrich(t, func(pools []models.GPUPool) error {
		pools[0].ActualSize = 1
		return nil
	})
	ld := stubLoader{pools: []models.GPUPool{{Name: "p", Size: 2}}}

	ds, warnings, err := FaultyDataset(context.Background(), ld, "/repo", "/kube", models.Environment{}, domain.GPUPool)
	require.NoError(t, err)
	assert.Empty(t, warnings)
	assert.Equal(t, 1, ds.GPUPools[0].ActualSize)

	_, _, err = FaultyDataset(context.Background(), ld, "/repo", "/kube", models.Environment{}, domain.GPUNode)
	require.NoError(t, err)
	assert.Equal(t, 1, *calls)
}

// Without live sizes every pool would look faulty: an error, not a
// misleading list.
func TestFaultyDataset_PoolEnrichmentFailureIsAnError(t *testing.T) {
	stubEnrich(t, func([]models.GPUPool) error { return errors.New("no OCI session") })
	_, _, err := FaultyDataset(context.Background(), stubLoader{pools: []models.GPUPool{{Name: "p"}}},
		"/repo", "/kube", models.Environment{}, domain.GPUPool)
	require.ErrorContains(t, err, "no OCI session")
}

func TestFaultyDataset_PartialPoolsWarn(t *testing.T) {
	stubEnrich(t, func([]models.GPUPool) error { return nil })
	ld := stubLoader{pools: []models.GPUPool{{Name: "p"}}, poolsErr: &terraform.PartialLoadError{Errs: []error{errors.New("oke unresolved")}}}
	_, warnings, err := FaultyDataset(context.Background(), ld, "/repo", "/kube", models.Environment{}, domain.GPUNode)
	require.NoError(t, err)
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "oke unresolved")
}

func TestFaultyDataset_LoadFailure(t *testing.T) {
	_, _, err := FaultyDataset(context.Background(), stubLoader{nodesErr: errors.New("cluster unreachable")},
		"/repo", "/kube", models.Environment{}, domain.GPUNode)
	require.ErrorContains(t, err, "load gpu nodes: cluster unreachable")
}
