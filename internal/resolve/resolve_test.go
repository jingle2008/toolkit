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
	return nil, nil
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
func fakeCompartmentResolver(t *testing.T, compartmentID string) {
	t.Helper()
	origClient := newClientsetFromKubeFn
	origList := listGpuNodesForCompartFn
	t.Cleanup(func() {
		newClientsetFromKubeFn = origClient
		listGpuNodesForCompartFn = origList
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
	orig := newClientsetFromKubeFn
	t.Cleanup(func() { newClientsetFromKubeFn = orig })
	newClientsetFromKubeFn = func(string, string) (kubernetes.Interface, error) {
		return nil, errors.New("kubeconfig parse failed")
	}

	_, err := CompartmentID(context.Background(), "/kube", models.Environment{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "kubeconfig parse failed")
}
