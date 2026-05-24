//nolint:paralleltest // GpuPool / CompartmentID tests mutate shared seam vars (fakeCompartmentResolver, fakePopulate, newClientsetFromKubeFn) and must run sequentially
package resolve

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/jingle2008/toolkit/internal/infra/terraform"
	"github.com/jingle2008/toolkit/pkg/models"
)

// stubLoader implements just enough of loader.Loader for GpuNode and
// GpuPool tests; every method is either inert or driven by the public
// fields. Keep narrow — the resolve package only calls LoadGpuNodes
// and LoadGpuPools.
type stubLoader struct {
	pools    []models.GpuPool
	poolsErr error
	nodes    map[string][]models.GpuNode
	nodesErr error
}

func (l stubLoader) LoadDataset(context.Context, string, models.Environment) (*models.Dataset, error) {
	return &models.Dataset{}, nil
}

func (l stubLoader) LoadImportedModels(context.Context, string, models.Environment) (map[string][]models.ImportedModel, error) {
	return nil, nil //nolint:nilnil // empty-map stub; resolve tests never read this
}

func (l stubLoader) LoadBaseModels(context.Context, string, models.Environment) ([]models.BaseModel, error) {
	return nil, nil
}

func (l stubLoader) LoadGpuPools(context.Context, string, models.Environment) ([]models.GpuPool, error) {
	return l.pools, l.poolsErr
}

func (l stubLoader) LoadGpuNodes(context.Context, string, models.Environment) (map[string][]models.GpuNode, error) {
	return l.nodes, l.nodesErr
}

func (l stubLoader) LoadDedicatedAIClusters(context.Context, string, models.Environment) (map[string][]models.DedicatedAICluster, error) {
	return nil, nil //nolint:nilnil // empty-map test stub; resolve tests never read this
}

func (l stubLoader) LoadTenancyOverrideGroup(context.Context, string, models.Environment) (models.TenancyOverrideGroup, error) {
	return models.TenancyOverrideGroup{}, nil
}

func (l stubLoader) LoadLimitRegionalOverrides(context.Context, string, models.Environment) ([]models.LimitRegionalOverride, error) {
	return nil, nil
}

func (l stubLoader) LoadConsolePropertyRegionalOverrides(context.Context, string, models.Environment) ([]models.ConsolePropertyRegionalOverride, error) {
	return nil, nil
}

func (l stubLoader) LoadPropertyRegionalOverrides(context.Context, string, models.Environment) ([]models.PropertyRegionalOverride, error) {
	return nil, nil
}

// -- GpuNode -------------------------------------------------------

func TestGpuNode_OcidBypassesLoader(t *testing.T) {
	t.Parallel()
	ld := stubLoader{nodesErr: errors.New("loader must not be called")}
	node, err := GpuNode(context.Background(), ld, "/dev/null", models.Environment{}, "n1", "ocid1.instance.fake")
	require.NoError(t, err)
	require.NotNil(t, node)
	assert.Equal(t, "n1", node.Name)
	assert.Equal(t, "ocid1.instance.fake", node.ID)
}

func TestGpuNode_FoundInCluster(t *testing.T) {
	t.Parallel()
	ld := stubLoader{nodes: map[string][]models.GpuNode{
		"pool-a": {{Name: "n1", ID: "ocid1.a"}, {Name: "n2", ID: "ocid1.b"}},
		"pool-b": {{Name: "n3", ID: "ocid1.c"}},
	}}
	node, err := GpuNode(context.Background(), ld, "/dev/null", models.Environment{}, "n2", "")
	require.NoError(t, err)
	assert.Equal(t, "ocid1.b", node.ID)
}

func TestGpuNode_NotFound(t *testing.T) {
	t.Parallel()
	ld := stubLoader{nodes: map[string][]models.GpuNode{
		"pool-a": {{Name: "n1"}},
	}}
	_, err := GpuNode(context.Background(), ld, "/dev/null", models.Environment{}, "n-missing", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestGpuNode_LoaderError(t *testing.T) {
	t.Parallel()
	ld := stubLoader{nodesErr: errors.New("kube unreachable")}
	_, err := GpuNode(context.Background(), ld, "/dev/null", models.Environment{}, "n1", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "load gpu nodes")
	assert.Contains(t, err.Error(), "kube unreachable")
}

// -- GpuPool -------------------------------------------------------

// fakeCompartmentResolver wires the package-level seams so GpuPool's
// compartment-ID call returns a fixed value without touching k8s.
// Also clears the compartment cache so tests don't pollute each other
// — the cache is package-level, so a previous test's successful
// lookup would otherwise be returned in place of the seam's response.
func fakeCompartmentResolver(t *testing.T, compartmentID string) {
	t.Helper()
	clearCompartmentCache()
	origClient := newClientsetFromKubeFn
	origList := listGpuNodesForCompartFn
	t.Cleanup(func() {
		newClientsetFromKubeFn = origClient
		listGpuNodesForCompartFn = origList
		clearCompartmentCache()
	})
	newClientsetFromKubeFn = func(string, string) (kubernetes.Interface, error) {
		return fake.NewSimpleClientset(), nil
	}
	listGpuNodesForCompartFn = func(context.Context, kubernetes.Interface, int) ([]models.GpuNode, error) {
		if compartmentID == "" {
			return nil, nil // simulate empty cluster
		}
		return []models.GpuNode{{CompartmentID: compartmentID}}, nil
	}
}

// fakePopulate swaps the action that fills in ID/ActualSize so the
// test controls the outcome without calling OCI.
func fakePopulate(t *testing.T, ocid string, perr error) {
	t.Helper()
	orig := populateGpuPoolsFn
	t.Cleanup(func() { populateGpuPoolsFn = orig })
	populateGpuPoolsFn = func(_ context.Context, pools []models.GpuPool, _ models.Environment, _ string) error {
		if perr != nil {
			return perr
		}
		// Populate the slice in place, mirroring actions.PopulateGpuPools.
		for i := range pools {
			pools[i].ID = ocid
			pools[i].ActualSize = 4
		}
		return nil
	}
}

func TestGpuPool_HappyPath(t *testing.T) {
	// Sequential — seam vars are global.
	fakeCompartmentResolver(t, "ocid1.compartment")
	fakePopulate(t, "ocid1.instancepool.fake", nil)

	ld := stubLoader{pools: []models.GpuPool{
		{Name: "p1", Size: 8},
		{Name: "p2", Size: 12},
	}}
	pool, err := GpuPool(context.Background(), ld, "/repo", "/kube", models.Environment{}, "p2")
	require.NoError(t, err)
	require.NotNil(t, pool)
	assert.Equal(t, "p2", pool.Name)
	assert.Equal(t, "ocid1.instancepool.fake", pool.ID)
	assert.Equal(t, 12, pool.Size)
}

func TestGpuPool_NotFound(t *testing.T) {
	fakeCompartmentResolver(t, "ocid1.compartment")
	fakePopulate(t, "", nil)

	ld := stubLoader{pools: []models.GpuPool{{Name: "p1"}}}
	_, err := GpuPool(context.Background(), ld, "/repo", "/kube", models.Environment{}, "p-missing")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found in repo")
}

func TestGpuPool_TolerantOfPartialLoad(t *testing.T) {
	// PartialLoadError is informational — proceed when the named pool
	// is still in the returned slice.
	fakeCompartmentResolver(t, "ocid1.compartment")
	fakePopulate(t, "ocid1.instancepool.fake", nil)

	partial := &terraform.PartialLoadError{
		Source: "GpuPools",
		Errs:   []error{errors.New("oke source missing")},
	}
	ld := stubLoader{
		pools:    []models.GpuPool{{Name: "p1", Size: 8}},
		poolsErr: partial,
	}
	pool, err := GpuPool(context.Background(), ld, "/repo", "/kube", models.Environment{}, "p1")
	require.NoError(t, err, "partial-load should not be fatal")
	assert.Equal(t, "ocid1.instancepool.fake", pool.ID)
}

func TestGpuPool_HardLoaderErrorFatal(t *testing.T) {
	// Non-partial errors must NOT be tolerated.
	ld := stubLoader{poolsErr: errors.New("repo path invalid")}
	_, err := GpuPool(context.Background(), ld, "/repo", "/kube", models.Environment{}, "p1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "load gpu pools")
}

func TestGpuPool_PopulateError(t *testing.T) {
	fakeCompartmentResolver(t, "ocid1.compartment")
	fakePopulate(t, "", errors.New("OCI throttled"))

	ld := stubLoader{pools: []models.GpuPool{{Name: "p1", Size: 8}}}
	_, err := GpuPool(context.Background(), ld, "/repo", "/kube", models.Environment{}, "p1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "populate gpu pool")
}

func TestGpuPool_NoOcidAfterPopulate(t *testing.T) {
	// Pool exists in Terraform but PopulateGpuPools didn't find it on
	// OCI side (NONEXIST) — guard against handing UpdateInstancePool an
	// empty OCID.
	fakeCompartmentResolver(t, "ocid1.compartment")
	fakePopulate(t, "", nil) // no ID assigned

	ld := stubLoader{pools: []models.GpuPool{{Name: "p1", Size: 8}}}
	_, err := GpuPool(context.Background(), ld, "/repo", "/kube", models.Environment{}, "p1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no OCID")
}

// -- CompartmentID -------------------------------------------------

func TestCompartmentID_HappyPath(t *testing.T) {
	fakeCompartmentResolver(t, "ocid1.compartment.real")

	got, err := CompartmentID(context.Background(), "/kube", models.Environment{Type: "dev", Region: "us-ashburn-1"})
	require.NoError(t, err)
	assert.Equal(t, "ocid1.compartment.real", got)
}

func TestCompartmentID_EmptyCluster(t *testing.T) {
	// listGpuNodesForCompartFn returns no nodes — we can't infer.
	fakeCompartmentResolver(t, "")

	_, err := CompartmentID(context.Background(), "/kube", models.Environment{Type: "dev"})
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "no GPU nodes"),
		"expected error about empty cluster, got: %v", err)
}

func TestCompartmentID_KubeUnreachable(t *testing.T) {
	clearCompartmentCache()
	t.Cleanup(clearCompartmentCache)
	orig := newClientsetFromKubeFn
	t.Cleanup(func() { newClientsetFromKubeFn = orig })
	newClientsetFromKubeFn = func(string, string) (kubernetes.Interface, error) {
		return nil, errors.New("kubeconfig parse failed")
	}

	_, err := CompartmentID(context.Background(), "/kube", models.Environment{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "kubeconfig parse failed")
}

// TestCompartmentID_Caches pins the new memoization: a successful
// lookup is reused on subsequent calls with the same (kubeConfig,
// kubeContext) so the MCP server doesn't burn a K8s lookup on every
// list_gpu_pools / scale_gpu_pool invocation. A different env (which
// produces a different kubeContext) bypasses the cache.
func TestCompartmentID_Caches(t *testing.T) {
	fakeCompartmentResolver(t, "ocid1.compartment.first")

	env := models.Environment{Type: "dev", Region: "us-ashburn-1"}
	got1, err := CompartmentID(context.Background(), "/kube", env)
	require.NoError(t, err)
	assert.Equal(t, "ocid1.compartment.first", got1)

	// Swap the seam to a different value. Cache should win.
	listGpuNodesForCompartFn = func(context.Context, kubernetes.Interface, int) ([]models.GpuNode, error) {
		return []models.GpuNode{{CompartmentID: "ocid1.compartment.SHOULD_NOT_SEE"}}, nil
	}
	got2, err := CompartmentID(context.Background(), "/kube", env)
	require.NoError(t, err)
	assert.Equal(t, "ocid1.compartment.first", got2, "cache should win against the changed seam")

	// Different env → different kubeContext → cache miss → seam fires.
	got3, err := CompartmentID(context.Background(), "/kube", models.Environment{Type: "dev", Region: "us-phoenix-1"})
	require.NoError(t, err)
	assert.Equal(t, "ocid1.compartment.SHOULD_NOT_SEE", got3, "different env key must miss the cache and call the seam")
}

// TestCompartmentID_DoesNotCacheErrors pins that failed lookups stay
// retry-able. An empty cluster on first call (or any K8s error) must
// not poison subsequent calls — otherwise a transient cluster
// hiccup at startup would brick the read path for the rest of the
// process.
func TestCompartmentID_DoesNotCacheErrors(t *testing.T) {
	fakeCompartmentResolver(t, "") // empty cluster → "no GPU nodes" error
	env := models.Environment{Type: "dev", Region: "us-ashburn-1"}

	_, err := CompartmentID(context.Background(), "/kube", env)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no GPU nodes")

	// Cluster recovers. Cache shouldn't have stored the prior error.
	listGpuNodesForCompartFn = func(context.Context, kubernetes.Interface, int) ([]models.GpuNode, error) {
		return []models.GpuNode{{CompartmentID: "ocid1.compartment.recovered"}}, nil
	}
	got, err := CompartmentID(context.Background(), "/kube", env)
	require.NoError(t, err, "second call must not hit a cached error")
	assert.Equal(t, "ocid1.compartment.recovered", got)
}

// -- EnrichGpuPools ------------------------------------------------

func TestEnrichGpuPools_HappyPath(t *testing.T) {
	fakeCompartmentResolver(t, "ocid1.compartment")
	fakePopulate(t, "ocid1.instancepool.fake", nil)

	pools := []models.GpuPool{{Name: "p1"}, {Name: "p2"}}
	msg := EnrichGpuPools(context.Background(), pools, "/kube", models.Environment{})
	assert.Empty(t, msg, "happy path should not return a warning")
	assert.Equal(t, 4, pools[0].ActualSize, "fakePopulate fills ActualSize=4")
	assert.Equal(t, 4, pools[1].ActualSize)
}

func TestEnrichGpuPools_EmptySlice_NoOp(t *testing.T) {
	// Don't wire seams: the empty-slice early-return must avoid them.
	msg := EnrichGpuPools(context.Background(), nil, "/kube", models.Environment{})
	assert.Empty(t, msg, "empty input should be a no-op")
}

func TestEnrichGpuPools_CompartmentLookupFailure_Warns(t *testing.T) {
	// listGpuNodesForCompartFn returns no nodes → CompartmentID errors;
	// EnrichGpuPools must surface that as a warning, not a panic.
	fakeCompartmentResolver(t, "")

	pools := []models.GpuPool{{Name: "p1", Status: "..."}}
	msg := EnrichGpuPools(context.Background(), pools, "/kube", models.Environment{})
	require.NotEmpty(t, msg)
	assert.Contains(t, msg, "compartment lookup failed")
	assert.Equal(t, "...", pools[0].Status, "placeholder must survive enrichment failure")
}

func TestEnrichGpuPools_PopulateFailure_Warns(t *testing.T) {
	fakeCompartmentResolver(t, "ocid1.compartment")
	fakePopulate(t, "", errors.New("OCI 500"))

	pools := []models.GpuPool{{Name: "p1"}}
	msg := EnrichGpuPools(context.Background(), pools, "/kube", models.Environment{})
	require.NotEmpty(t, msg)
	assert.Contains(t, msg, "OCI populate failed")
	assert.Contains(t, msg, "OCI 500")
}
