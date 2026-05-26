package mcp

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/internal/config"
	"github.com/jingle2008/toolkit/internal/infra/terraform"
	"github.com/jingle2008/toolkit/pkg/models"
)

func TestEnvFor_DefaultsThenOverrides(t *testing.T) {
	t.Parallel()
	s := &Server{cfg: config.Config{
		EnvType:   "dev",
		EnvRegion: "us-ashburn-1",
		EnvRealm:  "oc1",
	}}

	// No overrides → return startup defaults.
	got := s.envFor(envOverride{})
	assert.Equal(t, models.Environment{Type: "dev", Region: "us-ashburn-1", Realm: "oc1"}, got)

	// All overrides supplied.
	got = s.envFor(envOverride{EnvType: "prod", EnvRegion: "us-phoenix-1", EnvRealm: "oc2"})
	assert.Equal(t, models.Environment{Type: "prod", Region: "us-phoenix-1", Realm: "oc2"}, got)

	// Partial overrides — only env_type changes.
	got = s.envFor(envOverride{EnvType: "preprod"})
	assert.Equal(t, models.Environment{Type: "preprod", Region: "us-ashburn-1", Realm: "oc1"}, got)
}

func TestJsonResult_EnvelopeShape(t *testing.T) {
	t.Parallel()
	// We now assert directly on the typed envelope (the SDK marshals it
	// into StructuredContent and auto-emits an equivalent TextContent
	// block at JSON-RPC time; that path is exercised end-to-end in
	// the integration test, not here).
	items := []map[string]string{{"name": "a"}, {"name": "b"}}
	res, env, err := jsonResult(items, nil)
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Empty(t, res.Content, "jsonResult returns empty Content so the SDK can auto-emit it")
	assert.Equal(t, 2, env.Count)
	assert.Equal(t, items, env.Items)
	assert.Empty(t, env.Warnings)
}

func TestJsonResult_NilEmitsEmptyArray(t *testing.T) {
	t.Parallel()
	var nilSlice []string
	res, env, err := jsonResult(nilSlice, nil)
	require.NoError(t, err)
	require.NotNil(t, res)
	// nil slice gets normalized to an empty slice so JSON-encodes
	// as `[]` rather than `null`.
	assert.Equal(t, 0, env.Count)
	require.NotNil(t, env.Items)
	assert.Empty(t, env.Items)
}

func TestJsonResult_WarningsPropagate(t *testing.T) {
	t.Parallel()
	res, env, err := jsonResult([]int{1, 2, 3}, []string{"partial source X failed"})
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, 3, env.Count)
	assert.Equal(t, []int{1, 2, 3}, env.Items)
	assert.Equal(t, []string{"partial source X failed"}, env.Warnings)
}

func TestMutationSuccess_Shape(t *testing.T) {
	t.Parallel()
	res, env, err := mutationSuccess("cordon", "node", "gpu-node-42")
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Empty(t, res.Content, "mutationSuccess returns empty Content so the SDK can auto-emit it")
	assert.Equal(t, mutationResult{Status: "OK", Action: "cordon", Kind: "node", Target: "gpu-node-42"}, env)
}

func TestWarningsFromPartial(t *testing.T) {
	t.Parallel()
	assert.Nil(t, warningsFromPartial(nil))
	assert.Nil(t, warningsFromPartial(errors.New("plain error")))

	p := &terraform.PartialLoadError{
		Source: "GPUPools",
		Errs: []error{
			errors.New("source-a: nope"),
			errors.New("source-b: also nope"),
		},
	}
	assert.Equal(t, []string{"source-a: nope", "source-b: also nope"}, warningsFromPartial(p))
}

func TestNormFilter(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "abc", normFilter("ABC"))
	assert.Equal(t, "abc", normFilter("  AbC  "))
	assert.Equal(t, "", normFilter(""))
	assert.Equal(t, "", normFilter("   "))
}
