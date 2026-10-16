package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/cli/output"
	"github.com/jingle2008/toolkit/internal/config"
	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/pkg/models"
)

func TestParseRelatedTo(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in      string
		cat     domain.Category
		want    *domain.Scope
		wantErr string
	}{
		{in: "", cat: domain.GPUWorkload},
		{in: "dac/amaaaaaa-dac1", cat: domain.GPUWorkload, want: &domain.Scope{Category: domain.DedicatedAICluster, Name: "amaaaaaa-dac1"}},
		{in: "dedicatedaicluster/amaaaaaa-dac1", cat: domain.GPUWorkload, want: &domain.Scope{Category: domain.DedicatedAICluster, Name: "amaaaaaa-dac1"}},
		{in: "t/OABCS1", cat: domain.DedicatedAICluster, want: &domain.Scope{Category: domain.Tenant, Name: "OABCS1"}},
		// The name is verbatim: case, a trailing space, and further slashes survive.
		{in: "tenant/cldhorizonsqa ", cat: domain.InferenceService, want: &domain.Scope{Category: domain.Tenant, Name: "cldhorizonsqa "}},
		{in: "sr/a/b", cat: domain.InferenceService, want: &domain.Scope{Category: domain.ServingRuntime, Name: "a/b"}},
		{in: "dac", cat: domain.GPUWorkload, wantErr: "want <category>/<name>"},
		{in: "dac/", cat: domain.GPUWorkload, wantErr: "want <category>/<name>"},
		{in: "nope/x", cat: domain.GPUWorkload, wantErr: `unknown category "nope"`},
		{
			in: "gpupool/p", cat: domain.GPUWorkload,
			wantErr: "GPUPool is not related to GPUWorkload; use one of: tenant/, gpunode/, dedicatedaicluster/, servingruntime/",
		},
		{in: "tenant/x", cat: domain.Tenant, wantErr: "Tenant rows are not related to any other category"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			got, err := parseRelatedTo(tc.in, tc.cat)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestCompleteRelatedTo(t *testing.T) {
	t.Parallel()
	got, _ := completeRelatedTo(nil, []string{"is"}, "")
	assert.Equal(t, []string{"tenant/", "dedicatedaicluster/", "basemodel/", "importedmodel/", "servingruntime/"}, got)
	got, _ = completeRelatedTo(nil, []string{"tenant"}, "")
	assert.Empty(t, got)
	got, _ = completeRelatedTo(nil, nil, "")
	assert.Empty(t, got)
}

// relatedEmitLoader is emitLoader with workloads and endpoints spread
// over several groups, so --related-to has something to narrow.
type relatedEmitLoader struct{ emitLoader }

func (relatedEmitLoader) LoadGPUWorkloadsByNode(context.Context, string, models.Environment) (map[string][]models.GPUWorkload, error) {
	return map[string][]models.GPUWorkload{
		"node-a": {{Name: "pod-1", Node: "node-a", Namespace: "amaaaaaa-dac1", Runtime: "vllm"}, {Name: "pod-2", Node: "node-a", Namespace: "amaaaaaa-dac2"}},
		"node-b": {{Name: "pod-3", Node: "node-b", Namespace: "amaaaaaa-dac1", Runtime: "srt"}},
	}, nil
}

func (relatedEmitLoader) LoadInferenceServices(context.Context, string, models.Environment) (map[string][]models.InferenceService, error) {
	return map[string][]models.InferenceService{
		"amaaaaaa-dac1": {{Name: "isvc-1", Namespace: "amaaaaaa-dac1", Type: models.InferenceServiceDedicated, Model: "llama", Runtime: "vllm"}},
		"amaaaaaa-dac2": {{Name: "isvc-2", Namespace: "amaaaaaa-dac2", Type: models.InferenceServiceDedicated, Model: "cohere", Runtime: "vllm"}},
	}, nil
}

func emitNames(t *testing.T, cat domain.Category, related *domain.Scope, filter string) []string {
	t.Helper()
	var buf bytes.Buffer
	err := emitCategory(context.Background(), &buf, relatedEmitLoader{}, cat,
		config.Config{}, testEnv(), filter, related, 0, output.Options{Format: output.FormatJSON}, nil)
	require.NoError(t, err)
	var rows []map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &rows))
	names := make([]string, len(rows))
	for i, r := range rows {
		names[i], _ = r["name"].(string)
	}
	slices.Sort(names)
	return names
}

func TestEmitCategory_RelatedTo(t *testing.T) {
	t.Parallel()
	dac1 := &domain.Scope{Category: domain.DedicatedAICluster, Name: "amaaaaaa-dac1"}

	// A reference across groups: a DAC's pods on every node.
	assert.Equal(t, []string{"pod-1", "pod-3"}, emitNames(t, domain.GPUWorkload, dac1, ""))
	// Combined with --filter (which still matches the node group key).
	assert.Equal(t, []string{"pod-3"}, emitNames(t, domain.GPUWorkload, dac1, "node-b"))
	// A runtime's pods and endpoints.
	vllm := &domain.Scope{Category: domain.ServingRuntime, Name: "vllm"}
	assert.Equal(t, []string{"pod-1"}, emitNames(t, domain.GPUWorkload, vllm, ""))
	assert.Equal(t, []string{"isvc-1", "isvc-2"}, emitNames(t, domain.InferenceService, vllm, ""))
	// A storage-group parent: the DAC's endpoints.
	assert.Equal(t, []string{"isvc-1"}, emitNames(t, domain.InferenceService, dac1, ""))
	// No --related-to: unchanged.
	assert.Equal(t, []string{"pod-1", "pod-2", "pod-3"}, emitNames(t, domain.GPUWorkload, nil, ""))
	// Nothing related: an empty result, not an error.
	assert.Empty(t, emitNames(t, domain.GPUWorkload, &domain.Scope{Category: domain.DedicatedAICluster, Name: "missing"}, ""))
}

// The flat regional overrides and the tenancy-override group narrow too.
func TestEmitCategory_RelatedTo_FlatAndTenancyGroup(t *testing.T) {
	t.Parallel()
	assert.Equal(t, []string{"lro-a"},
		emitNames(t, domain.LimitRegionalOverride, &domain.Scope{Category: domain.LimitDefinition, Name: "lro-a"}, ""))
	assert.Empty(t,
		emitNames(t, domain.LimitRegionalOverride, &domain.Scope{Category: domain.LimitDefinition, Name: "other"}, ""))

	assert.Equal(t, []string{"lim-a"},
		emitNames(t, domain.LimitTenancyOverride, &domain.Scope{Category: domain.Tenant, Name: "tenant-a"}, ""))
	assert.Equal(t, []string{"lim-a"},
		emitNames(t, domain.LimitTenancyOverride, &domain.Scope{Category: domain.LimitDefinition, Name: "lim-a"}, ""))
	assert.Empty(t,
		emitNames(t, domain.LimitTenancyOverride, &domain.Scope{Category: domain.Tenant, Name: "tenant-b"}, ""))
}
