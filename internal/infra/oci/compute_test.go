//nolint:paralleltest // tests use global state
package oci

import (
	"crypto/rsa"
	"errors"
	"testing"

	"github.com/jingle2008/toolkit/pkg/models"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/stretchr/testify/require"
)

func makeEnv() models.Environment {
	return models.Environment{
		Realm:  "oc1",
		Region: "us-phoenix-1",
	}
}

type fakeProvider struct{}

var errNoKey = errors.New("no RSA key in fake provider")

func (f *fakeProvider) TenancyOCID() (string, error)            { return "", nil }
func (f *fakeProvider) UserOCID() (string, error)               { return "", nil }
func (f *fakeProvider) KeyFingerprint() (string, error)         { return "", nil }
func (f *fakeProvider) KeyID() (string, error)                  { return "", nil }
func (f *fakeProvider) PrivateKey() ([]byte, error)             { return nil, nil }
func (f *fakeProvider) PrivateKeyPassphrase() (string, error)   { return "", nil }
func (f *fakeProvider) Region() (string, error)                 { return "", nil }
func (f *fakeProvider) AuthType() (common.AuthConfig, error)    { return common.AuthConfig{}, nil }
func (f *fakeProvider) PrivateRSAKey() (*rsa.PrivateKey, error) { return nil, errNoKey }

func TestGetComputeClient_Success(t *testing.T) {
	origProvider := computeConfigProviderFunc
	origFactory := computeClientFactory
	defer func() {
		computeConfigProviderFunc = origProvider
		computeClientFactory = origFactory
	}()

	called := false
	computeConfigProviderFunc = func(path, profile, _ string) (common.ConfigurationProvider, error) {
		require.Equal(t, OciConfigPath, path)
		require.Equal(t, "OC1", profile)
		called = true
		return &fakeProvider{}, nil
	}
	computeClientFactory = func(_ common.ConfigurationProvider) (core.ComputeClient, error) {
		return core.ComputeClient{}, nil
	}

	env := makeEnv()
	client, err := GetComputeClient(env)
	require.NoError(t, err)
	require.NotNil(t, client)
	require.True(t, called)
}

func TestGetComputeClient_ConfigProviderError(t *testing.T) {
	origProvider := computeConfigProviderFunc
	defer func() { computeConfigProviderFunc = origProvider }()

	computeConfigProviderFunc = func(_, _, _ string) (common.ConfigurationProvider, error) {
		return nil, errors.New("provider error")
	}
	env := makeEnv()
	client, err := GetComputeClient(env)
	require.Error(t, err)
	require.Nil(t, client)
	require.Contains(t, err.Error(), "provider error")
}

func TestGetComputeClient_ClientFactoryError(t *testing.T) {
	origProvider := computeConfigProviderFunc
	origFactory := computeClientFactory
	defer func() {
		computeConfigProviderFunc = origProvider
		computeClientFactory = origFactory
	}()

	computeConfigProviderFunc = func(_, _, _ string) (common.ConfigurationProvider, error) {
		return &fakeProvider{}, nil
	}
	computeClientFactory = func(_ common.ConfigurationProvider) (core.ComputeClient, error) {
		return core.ComputeClient{}, errors.New("client error")
	}
	env := makeEnv()
	client, err := GetComputeClient(env)
	require.Error(t, err)
	require.Nil(t, client)
	require.Contains(t, err.Error(), "client error")
}

func TestGetComputeManagementClient_Success(t *testing.T) {
	origProvider := computeConfigProviderFunc
	origFactory := computeMgmtClientFactory
	defer func() {
		computeConfigProviderFunc = origProvider
		computeMgmtClientFactory = origFactory
	}()

	computeConfigProviderFunc = func(path, profile, _ string) (common.ConfigurationProvider, error) {
		require.Equal(t, OciConfigPath, path)
		require.Equal(t, "OC1", profile)
		return &fakeProvider{}, nil
	}
	computeMgmtClientFactory = func(_ common.ConfigurationProvider) (core.ComputeManagementClient, error) {
		return core.ComputeManagementClient{}, nil
	}

	env := makeEnv()
	client, err := GetComputeManagementClient(env)
	require.NoError(t, err)
	require.NotNil(t, client)
}

func TestGetComputeManagementClient_ConfigProviderError(t *testing.T) {
	origProvider := computeConfigProviderFunc
	defer func() { computeConfigProviderFunc = origProvider }()

	computeConfigProviderFunc = func(_, _, _ string) (common.ConfigurationProvider, error) {
		return nil, errors.New("provider error")
	}
	env := makeEnv()
	client, err := GetComputeManagementClient(env)
	require.Error(t, err)
	require.Nil(t, client)
	require.Contains(t, err.Error(), "provider error")
}

func TestGetComputeManagementClient_ClientFactoryError(t *testing.T) {
	origProvider := computeConfigProviderFunc
	origFactory := computeMgmtClientFactory
	defer func() {
		computeConfigProviderFunc = origProvider
		computeMgmtClientFactory = origFactory
	}()

	computeConfigProviderFunc = func(_, _, _ string) (common.ConfigurationProvider, error) {
		return &fakeProvider{}, nil
	}
	computeMgmtClientFactory = func(_ common.ConfigurationProvider) (core.ComputeManagementClient, error) {
		return core.ComputeManagementClient{}, errors.New("mgmt client error")
	}
	env := makeEnv()
	client, err := GetComputeManagementClient(env)
	require.Error(t, err)
	require.Nil(t, client)
	require.Contains(t, err.Error(), "mgmt client error")
}
