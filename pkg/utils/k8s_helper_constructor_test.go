package utils

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/rest"
)

func TestNewK8sHelper_ChangeContextError(t *testing.T) {
	// Create a temp file that is not a valid kubeconfig
	tmp := ""
	f, err := os.CreateTemp("", "badkubeconfig")
	assert.NoError(t, err)
	tmp = f.Name()
	f.Close()
	defer os.Remove(tmp)

	helper, err := NewK8sHelper(tmp, "nonexistent-context")
	assert.Error(t, err)
	assert.Nil(t, helper)
}

func TestNewK8sHelperWithClients_Error(t *testing.T) {
	badClient := func(_ *rest.Config) (KubernetesClient, error) {
		return nil, assert.AnError
	}
	badDyn := func(_ *rest.Config) (DynamicClient, error) {
		return nil, assert.AnError
	}
	helper, err := NewK8sHelperWithClients("bad", "bad", badClient, badDyn)
	assert.Error(t, err)
	assert.Nil(t, helper)
}
