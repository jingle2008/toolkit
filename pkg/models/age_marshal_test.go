package models

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// age is written from CreatedAt as of now; a stored Age is ignored, so a
// row serialized long after its load still says how old it is.
func TestMarshalJSON_AgeFromCreatedAt(t *testing.T) {
	t.Parallel()
	created := daysAgo(3)
	for name, v := range map[string]any{
		"GPUNode":            GPUNode{Name: "n", CreatedAt: created, Age: "stale"},
		"GPUWorkload":        GPUWorkload{Name: "w", CreatedAt: created, Age: "stale"},
		"DedicatedAICluster": DedicatedAICluster{Name: "d", CreatedAt: created, Age: "stale"},
		"InferenceService":   InferenceService{Name: "s", CreatedAt: created, Age: "stale"},
	} {
		b, err := json.Marshal(v)
		require.NoError(t, err, name)
		var got map[string]any
		require.NoError(t, json.Unmarshal(b, &got), name)
		assert.Equal(t, "3d", got["age"], name)
		assert.Equal(t, created.Format(time.RFC3339), got["createdAt"], name)
	}

	b, err := json.Marshal(GPUNode{Name: "n"})
	require.NoError(t, err)
	assert.Contains(t, string(b), `"age":""`, "no creation time, no age")
	assert.NotContains(t, string(b), "createdAt")
}
