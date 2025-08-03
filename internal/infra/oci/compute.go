package oci

import (
	"fmt"
	"strings"

	"github.com/jingle2008/toolkit/pkg/models"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
)

// GetComputeClient creates a new OCI ComputeClient for the given region.
func GetComputeClient(env models.Environment) (*core.ComputeClient, error) {
	profile := strings.ToUpper(env.Realm)
	provider, err := common.ConfigurationProviderForSessionTokenWithProfile(OciConfigPath, profile, "")
	if err != nil {
		return nil, fmt.Errorf("failed to get OCI config provider: %w", err)
	}
	client, err := core.NewComputeClientWithConfigurationProvider(provider)
	if err != nil {
		return nil, fmt.Errorf("failed to create compute client: %w", err)
	}
	client.SetRegion(env.Region)
	return &client, nil
}
