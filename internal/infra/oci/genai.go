// Package oci provides helpers for OCI Generative AI SDK client creation.
package oci

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/jingle2008/toolkit/pkg/infra/logging"
	"github.com/jingle2008/toolkit/pkg/models"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/generativeai"
)

const OCI_CONFIG_PATH = "~/.oci/config"

type (
	configProviderFunc func(string, string, string) (common.ConfigurationProvider, error)
	genAIClientFunc    func(common.ConfigurationProvider) (generativeai.GenerativeAiClient, error)
)

// getGenAIClientWithDeps is like GetGenAIClient but allows dependency injection for testing.
func getGenAIClientWithDeps(
	env models.Environment,
	providerFn configProviderFunc,
	clientFn genAIClientFunc,
) (*generativeai.GenerativeAiClient, error) {
	profile := strings.ToUpper(env.Realm)
	provider, err := providerFn(OCI_CONFIG_PATH, profile, "")
	if err != nil {
		return nil, fmt.Errorf("failed to get OCI config provider: %w", err)
	}

	client, err := clientFn(provider)
	if err != nil {
		return nil, fmt.Errorf("failed to create GenerativeAI client: %w", err)
	}

	client.SetRegion(env.Region)
	if env.Type != "prod" {
		client.Host = getServiceEndpoint(client.Endpoint(), env.Type)
	}

	return &client, nil
}

// GetGenAIClient returns a ConfigurationProvider for the given profile (profile name is uppercased).
func GetGenAIClient(env models.Environment, logger logging.Logger) (*generativeai.GenerativeAiClient, error) {
	return getGenAIClientWithDeps(
		env,
		common.ConfigurationProviderForSessionTokenWithProfile,
		generativeai.NewGenerativeAiClientWithConfigurationProvider,
	)
}

// getServiceEndpoint returns the override endpoint for non-production regions.
func getServiceEndpoint(serviceEndpoint string, envType string) string {
	prefix := strings.ReplaceAll(envType, "preprod", "ppe")
	u, _ := url.Parse(serviceEndpoint)
	u.Host = fmt.Sprintf("%s.%s", prefix, u.Host)
	return u.String()
}
