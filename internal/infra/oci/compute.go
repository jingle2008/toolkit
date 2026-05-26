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

// newOCIClient is the shared scaffold for OCI client factories: build
// the realm-scoped session-token config provider, hand it to the
// typed factory, then call SetRegion on the freshly-built client.
// setRegion is the typed method expression (e.g.
// (*core.ComputeClient).SetRegion) so the helper stays generic over
// the concrete client type.
func newOCIClient[T any](
	env models.Environment,
	factory func(common.ConfigurationProvider) (T, error),
	setRegion func(*T, string),
	label string,
) (*T, error) {
	profile := strings.ToUpper(env.Realm)
	provider, err := computeConfigProviderFunc(OciConfigPath, profile, "")
	if err != nil {
		return nil, fmt.Errorf("failed to get OCI config provider: %w", err)
	}
	client, err := factory(provider)
	if err != nil {
		return nil, fmt.Errorf("failed to create %s: %w", label, err)
	}
	setRegion(&client, env.Region)
	return &client, nil
}

// NewComputeClient creates a new OCI ComputeClient for the given region.
func NewComputeClient(env models.Environment) (*core.ComputeClient, error) {
	return newOCIClient(env, computeClientFactory, (*core.ComputeClient).SetRegion, "compute client")
}

// NewComputeManagementClient creates a new OCI ComputeManagementClient for the given region.
func NewComputeManagementClient(env models.Environment) (*core.ComputeManagementClient, error) {
	return newOCIClient(env, computeMgmtClientFactory, (*core.ComputeManagementClient).SetRegion, "compute management client")
}
