package actions

import (
	"context"
	"errors"
	"testing"

	"github.com/jingle2008/toolkit/pkg/models"
	"github.com/oracle/oci-go-sdk/v65/generativeai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeleteDedicatedAICluster_ClientError(t *testing.T) {
	t.Parallel()
	origGetGenAIClient := getGenAIClient
	defer func() { getGenAIClient = origGetGenAIClient }()

	// Patch getGenAIClient to simulate client creation error
	getGenAIClient = func(_ models.Environment) (*generativeai.GenerativeAiClient, error) {
		return nil, errors.New("client creation failed")
	}

	dac := &models.DedicatedAICluster{}
	env := models.Environment{Type: "prod", Region: "us-phoenix-1", Realm: "oc1"}
	logger := &fakeLogger{}
	err := DeleteDedicatedAICluster(context.Background(), dac, env, logger)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create GenerativeAI client")
}
