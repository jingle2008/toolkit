package oci

import (
	"fmt"
	"strings"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"

	"github.com/jingle2008/toolkit/pkg/models"
)

var (
	computeConfigProviderFunc = common.ConfigurationProviderForSessionTokenWithProfile
	computeClientFactory      = core.NewComputeClientWithConfigurationProvider
	computeMgmtClientFactory  = core.NewComputeManagementClientWithConfigurationProvider
)

// GetComputeClient creates a new OCI ComputeClient for the given region.
func GetComputeClient(env models.Environment) (*core.ComputeClient, error) {
	profile := strings.ToUpper(env.Realm)
	provider, err := computeConfigProviderFunc(OciConfigPath, profile, "")
	if err != nil {
		return nil, fmt.Errorf("failed to get OCI config provider: %w", err)
	}
	client, err := computeClientFactory(provider)
	if err != nil {
		return nil, fmt.Errorf("failed to create compute client: %w", err)
	}
	client.SetRegion(env.Region)
	return &client, nil
}

/*
GetComputeManagementClient creates a new OCI ComputeManagementClient for the given region.
*/
func GetComputeManagementClient(env models.Environment) (*core.ComputeManagementClient, error) {
	profile := strings.ToUpper(env.Realm)
	provider, err := computeConfigProviderFunc(OciConfigPath, profile, "")
	if err != nil {
		return nil, fmt.Errorf("failed to get OCI config provider: %w", err)
	}
	client, err := computeMgmtClientFactory(provider)
	if err != nil {
		return nil, fmt.Errorf("failed to create compute management client: %w", err)
	}
	client.SetRegion(env.Region)
	return &client, nil
}
