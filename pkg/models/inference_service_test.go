package models

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Replicas shows what the spec sets: nothing when it sets neither bound
// (not a misleading 0/0), and — for a bound it leaves out. JSON omits an
// unset bound rather than reporting 0.
func TestInferenceService_Replicas(t *testing.T) {
	t.Parallel()
	assert.Empty(t, InferenceService{}.Replicas())
	assert.Equal(t, "1/3", InferenceService{MinReplicas: new(1), MaxReplicas: new(3)}.Replicas())
	assert.Equal(t, "0/2", InferenceService{MinReplicas: new(0), MaxReplicas: new(2)}.Replicas())
	assert.Equal(t, "2/—", InferenceService{MinReplicas: new(2)}.Replicas())

	data, err := json.Marshal(InferenceService{Name: "x", MinReplicas: new(0)})
	require.NoError(t, err)
	assert.Contains(t, string(data), `"minReplicas":0`)
	assert.NotContains(t, string(data), "maxReplicas")
}
