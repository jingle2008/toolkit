package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/config"
	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/cli/output"
	"github.com/jingle2008/toolkit/pkg/models"
)

// emitLoader is a loader.Composite whose every method returns scripted
// data (or a scripted error), so emitCategory's per-category switch can be
// driven without a repo, a cluster, or OCI auth.
type emitLoader struct {
	err error // when non-nil, every Load* fails with this
}

func (l emitLoader) LoadDataset(context.Context, string, models.Environment) (*models.Dataset, error) {
	if l.err != nil {
		return nil, l.err
	}
	return &models.Dataset{
		LimitDefinitionGroup:           models.LimitDefinitionGroup{Values: []models.LimitDefinition{{Name: "lim-a"}}},
		ConsolePropertyDefinitionGroup: models.ConsolePropertyDefinitionGroup{Values: []models.ConsolePropertyDefinition{{Name: "cpd-a"}}},
		PropertyDefinitionGroup:        models.PropertyDefinitionGroup{Values: []models.PropertyDefinition{{Name: "pd-a"}}},
		Environments:                   []models.Environment{{Type: "dev", Region: "us-ashburn-1", Realm: "oc1"}},
		ServiceTenancies:               []models.ServiceTenancy{{Name: "svc-a", Realm: "oc1"}},
		ModelArtifactMap:               map[string][]models.ModelArtifact{"m1": {{Name: "art-a", ModelName: "m1"}}},
	}, nil
}

func (l emitLoader) LoadBaseModels(context.Context, string, models.Environment) ([]models.BaseModel, error) {
	if l.err != nil {
		return nil, l.err
	}
	return []models.BaseModel{{Name: "bm-a", Status: "Ready"}}, nil
}

func (l emitLoader) LoadImportedModels(context.Context, string, models.Environment) (map[string][]models.ImportedModel, error) {
	if l.err != nil {
		return nil, l.err
	}
	return map[string][]models.ImportedModel{
		"t1": {{BaseModel: models.BaseModel{Name: "im-a"}, TenantID: "t1"}},
	}, nil
}

func (l emitLoader) LoadGPUPools(context.Context, string, models.Environment) ([]models.GPUPool, error) {
	if l.err != nil {
		return nil, l.err
	}
	return []models.GPUPool{{Name: "pool-a", Shape: "BM.GPU.8", Size: 1}}, nil
}

func (l emitLoader) LoadGPUNodesByPool(context.Context, string, models.Environment) (map[string][]models.GPUNode, error) {
	if l.err != nil {
		return nil, l.err
	}
	return map[string][]models.GPUNode{
		"pool-a": {{Name: "node-a", NodePool: "pool-a", InstanceType: "BM.GPU.8", Allocatable: 8, IsReady: true}},
	}, nil
}

func (l emitLoader) LoadGPUWorkloadsByNode(context.Context, string, models.Environment) (map[string][]models.GPUWorkload, error) {
	if l.err != nil {
		return nil, l.err
	}
	return map[string][]models.GPUWorkload{"node-a": {{Name: "wl-a"}}}, nil
}

func (l emitLoader) LoadDedicatedAIClusters(context.Context, string, models.Environment) (map[string][]models.DedicatedAICluster, error) {
	if l.err != nil {
		return nil, l.err
	}
	return map[string][]models.DedicatedAICluster{
		"t1": {{Name: "dac-a", TenantID: "t1", Status: "Ready"}},
	}, nil
}

func (l emitLoader) LoadTenancyOverrideGroup(context.Context, string, models.Environment) (models.TenancyOverrideGroup, error) {
	if l.err != nil {
		return models.TenancyOverrideGroup{}, l.err
	}
	return models.TenancyOverrideGroup{
		Tenants: []models.Tenant{{Name: "tenant-a", IDs: []string{"id1"}}},
		LimitTenancyOverrideMap: map[string][]models.LimitTenancyOverride{
			"tenant-a": {{LimitRegionalOverride: models.LimitRegionalOverride{Name: "lim-a"}, TenantID: "id1"}},
		},
		ConsolePropertyTenancyOverrideMap: map[string][]models.ConsolePropertyTenancyOverride{
			"tenant-a": {{ConsolePropertyRegionalOverride: models.ConsolePropertyRegionalOverride{Name: "cp-a"}, TenantID: "id1"}},
		},
		PropertyTenancyOverrideMap: map[string][]models.PropertyTenancyOverride{
			"tenant-a": {{PropertyRegionalOverride: models.PropertyRegionalOverride{Name: "p-a"}, TenantID: "id1"}},
		},
	}, nil
}

func (l emitLoader) LoadLimitRegionalOverrides(context.Context, string, models.Environment) ([]models.LimitRegionalOverride, error) {
	if l.err != nil {
		return nil, l.err
	}
	return []models.LimitRegionalOverride{{Name: "lro-a"}}, nil
}

func (l emitLoader) LoadConsolePropertyRegionalOverrides(context.Context, string, models.Environment) ([]models.ConsolePropertyRegionalOverride, error) {
	if l.err != nil {
		return nil, l.err
	}
	return []models.ConsolePropertyRegionalOverride{{Name: "cpro-a"}}, nil
}

func (l emitLoader) LoadPropertyRegionalOverrides(context.Context, string, models.Environment) ([]models.PropertyRegionalOverride, error) {
	if l.err != nil {
		return nil, l.err
	}
	return []models.PropertyRegionalOverride{{Name: "pro-a"}}, nil
}

// emitCategories is every category `toolkit get` supports, paired with a
// string that must appear in the rendered output. GPUPool is excluded: its
// branch calls resolve.EnrichGPUPools, which reaches K8s/OCI.
var emitCategories = []struct {
	cat  domain.Category
	want string
}{
	{domain.Alias, "tenant"},
	{domain.BaseModel, "bm-a"},
	{domain.ImportedModel, "im-a"},
	{domain.GPUNode, "node-a"},
	{domain.GPUWorkload, "wl-a"},
	{domain.DedicatedAICluster, "dac-a"},
	{domain.Tenant, "tenant-a"},
	{domain.LimitTenancyOverride, "lim-a"},
	{domain.ConsolePropertyTenancyOverride, "cp-a"},
	{domain.PropertyTenancyOverride, "p-a"},
	{domain.LimitRegionalOverride, "lro-a"},
	{domain.ConsolePropertyRegionalOverride, "cpro-a"},
	{domain.PropertyRegionalOverride, "pro-a"},
	{domain.LimitDefinition, "lim-a"},
	{domain.ConsolePropertyDefinition, "cpd-a"},
	{domain.PropertyDefinition, "pd-a"},
	{domain.Environment, "dev"},
	{domain.ServiceTenancy, "svc-a"},
	{domain.ModelArtifact, "art-a"},
}

func testEnv() models.Environment {
	return models.Environment{Type: "dev", Region: "us-ashburn-1", Realm: "oc1"}
}

func TestEmitCategory_AllCategories_JSON(t *testing.T) {
	t.Parallel()
	for _, tc := range emitCategories {
		t.Run(tc.cat.String(), func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			err := emitCategory(context.Background(), &buf, emitLoader{}, tc.cat,
				config.Config{}, testEnv(), "", 0,
				output.Options{Format: output.FormatJSON}, nil)
			require.NoError(t, err)
			assert.Contains(t, buf.String(), tc.want)
		})
	}
}

func TestEmitCategory_AllCategories_Table(t *testing.T) {
	t.Parallel()
	// Table output goes through the column registry rather than encoding
	// the structs directly, so it exercises a different write path.
	for _, tc := range emitCategories {
		t.Run(tc.cat.String(), func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			err := emitCategory(context.Background(), &buf, emitLoader{}, tc.cat,
				config.Config{}, testEnv(), "", 0,
				output.Options{Format: output.FormatTable}, nil)
			require.NoError(t, err)
			assert.NotEmpty(t, buf.String())
		})
	}
}

func TestEmitCategory_LoaderErrorsAreWrapped(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("loader exploded")
	// Alias needs no loader, so it can't fail this way.
	for _, tc := range emitCategories {
		if tc.cat == domain.Alias {
			continue
		}
		t.Run(tc.cat.String(), func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			err := emitCategory(context.Background(), &buf, emitLoader{err: sentinel}, tc.cat,
				config.Config{}, testEnv(), "", 0,
				output.Options{Format: output.FormatJSON}, nil)
			require.ErrorIs(t, err, sentinel, "the underlying cause must survive wrapping")
			assert.NotEqual(t, sentinel.Error(), err.Error(),
				"error should be wrapped with a 'load ...' label, not returned bare")
		})
	}
}

func TestEmitCategory_UnsupportedCategory(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	err := emitCategory(context.Background(), &buf, emitLoader{}, domain.CategoryUnknown,
		config.Config{}, testEnv(), "", 0,
		output.Options{Format: output.FormatJSON}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not supported")
}

func TestEmitTenancyGroup_UnsupportedCategory(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	// A category that isn't part of the tenancy group must be rejected
	// rather than silently emitting nothing.
	err := emitTenancyGroup(&buf, domain.BaseModel, models.TenancyOverrideGroup{},
		"", 0, output.Options{Format: output.FormatJSON}, testEnv(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not in tenancy group")
}

func TestEmitCategory_FilterMatchingNothing(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	err := emitCategory(context.Background(), &buf, emitLoader{}, domain.BaseModel,
		config.Config{}, testEnv(), "no-such-model", 0,
		output.Options{Format: output.FormatJSON}, nil)
	require.NoError(t, err, "an empty result is not an error")

	// FilterSlice returns a typed nil slice here, which encoding/json
	// would render as "null" and break `| jq '.[]'`. WriteJSON normalizes
	// it to an empty array instead.
	assert.Equal(t, "[]", strings.TrimSpace(buf.String()))
}

func TestEmitCategory_LimitTruncates(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	err := emitCategory(context.Background(), &buf, emitLoader{}, domain.Alias,
		config.Config{}, testEnv(), "", 2,
		output.Options{Format: output.FormatJSON}, nil)
	require.NoError(t, err)

	var got []map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))
	assert.Len(t, got, 2, "limit should cap the emitted rows")
}

func TestEmitCategory_UnsupportedFormat(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	err := emitCategory(context.Background(), &buf, emitLoader{}, domain.BaseModel,
		config.Config{}, testEnv(), "", 0,
		output.Options{Format: output.Format("toml")}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported format")
}
