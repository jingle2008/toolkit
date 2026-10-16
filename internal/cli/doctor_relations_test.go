package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/config"
	"github.com/jingle2008/toolkit/internal/domain"
	loader "github.com/jingle2008/toolkit/internal/infra/loader"
	"github.com/jingle2008/toolkit/internal/infra/terraform"
	"github.com/jingle2008/toolkit/internal/relations"
	"github.com/jingle2008/toolkit/pkg/models"
)

func TestRelationResult(t *testing.T) {
	t.Parallel()
	pass := relationResult(relations.Check{Name: "A → B", Checked: 5, Severity: relations.SeverityFail})
	assert.Equal(t, checkResult{Name: "relations: A → B", Status: statusPass, Detail: "5 checked"}, pass)

	dangling := make([]relations.Dangling, 0, 4)
	for _, r := range []string{"r1", "r2", "r3", "r4"} {
		dangling = append(dangling, relations.Dangling{Row: r, Missing: "m-" + r})
	}
	fail := relationResult(relations.Check{
		Child: domain.InferenceService, Parent: domain.DedicatedAICluster, Name: "InferenceService → DedicatedAICluster",
		Checked: 9, Severity: relations.SeverityFail, Dangling: dangling, Note: "a note",
	})
	assert.Equal(t, statusFail, fail.Status)
	assert.Equal(t, "9 checked; 4 dangling: r1 (missing m-r1), r2 (missing m-r2), r3 (missing m-r3), +1 more; a note", fail.Detail)
	assert.Equal(t, []string{"r1 → m-r1", "r2 → m-r2", "r3 → m-r3", "r4 → m-r4"}, fail.Items)
	assert.Contains(t, fail.Hint, "InferenceService rows name a DedicatedAICluster that doesn't exist")

	warn := relationResult(relations.Check{
		Parent: domain.Tenant, Name: "tenancy resolution", Checked: 2,
		Severity: relations.SeverityWarn, Dangling: dangling[:1],
	})
	assert.Equal(t, statusWarn, warn.Status)
	assert.Contains(t, warn.Hint, "metadata file")
}

// relationsLoader is emitLoader with one dangling endpoint (its DAC is
// gone), one unresolved tenancy, and a node in an unknown pool.
type relationsLoader struct {
	emitLoader
	poolErr error
}

func (relationsLoader) LoadDataset(context.Context, string, models.Environment) (*models.Dataset, error) {
	return &models.Dataset{Tenants: []models.Tenant{{Name: "Acme", IDs: []string{"ocid1.tenancy.oc1..acme"}}}}, nil
}

func (l relationsLoader) LoadGPUPools(context.Context, string, models.Environment) ([]models.GPUPool, error) {
	return []models.GPUPool{{Name: "pool-a"}}, l.poolErr
}

func (relationsLoader) LoadGPUNodesByPool(context.Context, string, models.Environment) (map[string][]models.GPUNode, error) {
	return map[string][]models.GPUNode{"pool-a": {{Name: "node-a"}}, "pool-x": {{Name: "node-x"}}}, nil
}

func (relationsLoader) LoadGPUWorkloadsByNode(context.Context, string, models.Environment) (map[string][]models.GPUWorkload, error) {
	return map[string][]models.GPUWorkload{"node-a": {{Name: "pod-1", Namespace: "llama", TenantID: "nobody"}}}, nil
}

func (relationsLoader) LoadDedicatedAIClusters(context.Context, string, models.Environment) (map[string][]models.DedicatedAICluster, error) {
	return map[string][]models.DedicatedAICluster{"acme": {{Name: "amaaaaaa-dac", TenantID: "acme"}}}, nil
}

func (relationsLoader) LoadInferenceServices(context.Context, string, models.Environment) (map[string][]models.InferenceService, error) {
	return map[string][]models.InferenceService{
		"amaaaaaa-dac":  {{Name: "isvc-ok", Namespace: "amaaaaaa-dac", Type: models.InferenceServiceDedicated}},
		"amaaaaaa-gone": {{Name: "isvc-orphan", Namespace: "amaaaaaa-gone", Type: models.InferenceServiceDedicated}},
	}, nil
}

func relationsCfg() config.Config {
	return config.Config{RepoPath: "/repo", KubeConfig: "/kube", EnvType: "prod", EnvRegion: "us-chicago-1", EnvRealm: "oc1"}
}

func useDoctorLoader(t *testing.T, ld loader.Composite) {
	t.Helper()
	orig := doctorLoaderFn
	t.Cleanup(func() { doctorLoaderFn = orig })
	doctorLoaderFn = func(context.Context, string) loader.Composite { return ld }
}

func byName(results []checkResult) map[string]checkResult {
	m := make(map[string]checkResult, len(results))
	for _, r := range results {
		m[r.Name] = r
	}
	return m
}

//nolint:paralleltest // swaps doctorLoaderFn
func TestRelationChecks(t *testing.T) {
	useDoctorLoader(t, relationsLoader{})
	got := byName(relationChecks(context.Background(), relationsCfg()))

	isvc := got["relations: InferenceService → DedicatedAICluster"]
	assert.Equal(t, statusFail, isvc.Status)
	assert.Equal(t, []string{"amaaaaaa-gone/isvc-orphan → amaaaaaa-gone"}, isvc.Items)
	assert.Equal(t, statusFail, got["relations: GPUNode → GPUPool"].Status)
	assert.Equal(t, statusPass, got["relations: GPUWorkload → DedicatedAICluster"].Status, "llama isn't a DAC namespace")

	ten := got["relations: tenancy resolution"]
	assert.Equal(t, statusWarn, ten.Status, "the DAC resolves; the pod's and emitLoader's imported model's don't")
	assert.Equal(t, []string{"ImportedModel /im-a → t1", "GPUWorkload llama/pod-1 → nobody"}, ten.Items)
}

// A partial pool load makes a missing pool inconclusive: WARN, not FAIL.
//
//nolint:paralleltest // swaps doctorLoaderFn
func TestRelationChecks_PartialPoolsWarn(t *testing.T) {
	useDoctorLoader(t, relationsLoader{poolErr: &terraform.PartialLoadError{Errs: []error{errors.New("oke pools unresolved")}}})
	r := byName(relationChecks(context.Background(), relationsCfg()))["relations: GPUNode → GPUPool"]
	assert.Equal(t, statusWarn, r.Status)
	assert.Contains(t, r.Detail, "some GPU pool sources failed to load")
}

//nolint:paralleltest // swaps doctorLoaderFn
func TestRelationChecks_LoadFailureAndMissingConfig(t *testing.T) {
	useDoctorLoader(t, emitLoader{err: errors.New("cluster unreachable")})
	got := relationChecks(context.Background(), relationsCfg())
	require.Len(t, got, 1)
	assert.Equal(t, statusFail, got[0].Status)
	assert.Contains(t, got[0].Detail, "cluster unreachable")

	got = relationChecks(context.Background(), config.Config{RepoPath: "/repo"})
	require.Len(t, got, 1)
	assert.Equal(t, statusSkip, got[0].Status)
	assert.Contains(t, got[0].Detail, "--kubeconfig")
}

// End to end through the command: --relations appends the rows and a
// FAIL makes doctor exit non-zero; without it doctor stays local.
//
//nolint:paralleltest // NewRootCmd uses cobra/viper global state; swaps doctorLoaderFn
func TestDoctorCmd_Relations(t *testing.T) {
	useDoctorLoader(t, relationsLoader{})
	tmp := t.TempDir()
	repoDir := filepath.Join(tmp, "repo")
	require.NoError(t, os.Mkdir(repoDir, 0o750))
	kubePath := filepath.Join(tmp, "kube.yaml")
	require.NoError(t, os.WriteFile(kubePath, []byte("placeholder"), 0o600))
	cfgPath := filepath.Join(tmp, "cfg.yaml")
	require.NoError(t, os.WriteFile(cfgPath, fullCfgContents(repoDir, kubePath), 0o600))
	t.Setenv("HOME", tmp)

	run := func(args ...string) ([]checkResult, error) {
		viper.Reset()
		t.Cleanup(viper.Reset)
		cmd := NewRootCmd("vtest")
		cmd.SetArgs(append([]string{"--config", cfgPath, "doctor", "-o", "json"}, args...))
		stdout := new(bytes.Buffer)
		cmd.SetOut(stdout)
		cmd.SetErr(new(bytes.Buffer))
		err := cmd.Execute()
		var results []checkResult
		require.NoError(t, json.Unmarshal(stdout.Bytes(), &results), stdout.String())
		return results, err
	}

	local, err := run()
	require.NoError(t, err)
	assert.NotContains(t, byName(local), "relations: InferenceService → DedicatedAICluster")

	all, err := run("--relations")
	require.Error(t, err, "a FAIL row fails doctor")
	assert.Equal(t, statusFail, byName(all)["relations: InferenceService → DedicatedAICluster"].Status)
}
