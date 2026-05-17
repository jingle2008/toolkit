package mcp

import (
	"encoding/json"
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
	items := []map[string]string{{"name": "a"}, {"name": "b"}}
	res, _, err := jsonResult(items, nil)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Len(t, res.Content, 1)

	var env struct {
		Items    []map[string]string `json:"items"`
		Count    int                 `json:"count"`
		Warnings []string            `json:"warnings"`
	}
	body := extractText(t, res)
	require.NoError(t, json.Unmarshal([]byte(body), &env))
	assert.Equal(t, 2, env.Count)
	assert.Equal(t, items, env.Items)
	assert.Empty(t, env.Warnings)
}

func TestJsonResult_NilEmitsEmptyArray(t *testing.T) {
	t.Parallel()
	var nilSlice []string
	res, _, err := jsonResult(nilSlice, nil)
	require.NoError(t, err)
	body := extractText(t, res)
	var env struct {
		Items []string `json:"items"`
		Count int      `json:"count"`
	}
	require.NoError(t, json.Unmarshal([]byte(body), &env))
	assert.Equal(t, 0, env.Count)
	assert.Empty(t, env.Items)
}

func TestJsonResult_WarningsPropagate(t *testing.T) {
	t.Parallel()
	res, _, err := jsonResult([]int{1, 2, 3}, []string{"partial source X failed"})
	require.NoError(t, err)
	body := extractText(t, res)
	var env struct {
		Count    int      `json:"count"`
		Warnings []string `json:"warnings"`
	}
	require.NoError(t, json.Unmarshal([]byte(body), &env))
	assert.Equal(t, 3, env.Count)
	assert.Equal(t, []string{"partial source X failed"}, env.Warnings)
}

func TestWarningsFromPartial(t *testing.T) {
	t.Parallel()
	assert.Nil(t, warningsFromPartial(nil))
	assert.Nil(t, warningsFromPartial(errors.New("plain error")))

	p := &terraform.PartialLoadError{
		Source: "GpuPools",
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

// extractText pulls the first text content block out of a CallToolResult
// for assertion-style parsing.
func extractText(t *testing.T, res any) string {
	t.Helper()
	raw, err := json.Marshal(res)
	require.NoError(t, err)
	var unmarshaled struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	require.NoError(t, json.Unmarshal(raw, &unmarshaled))
	require.NotEmpty(t, unmarshaled.Content)
	return unmarshaled.Content[0].Text
}
