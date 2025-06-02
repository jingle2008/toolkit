package utils

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewK8sHelper_ChangeContextError(t *testing.T) {
	// Create a temp file that is not a valid kubeconfig
	tmp := ""
	f, err := os.CreateTemp("", "badkubeconfig")
	require.NoError(t, err)
	tmp = f.Name()
	_ = f.Close()
	defer func() { _ = os.Remove(tmp) }()

	helper, err := NewK8sHelper(tmp, "nonexistent-context")
	require.Error(t, err)
	assert.Nil(t, helper)
}
