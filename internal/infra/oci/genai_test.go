package oci

import (
	"errors"
	"net/url"
	"testing"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/generativeai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jingle2008/toolkit/pkg/models"
)

func Test_getServiceEndpoint(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		serviceURL   string
		envType      string
		wantHost     string
		wantScheme   string
		wantPath     string
		wantRawQuery string
	}{
		{
			name:         "prod - no prefix",
			serviceURL:   "https://genai.us-phoenix-1.oci.oraclecloud.com/v1/resource?foo=bar",
			envType:      "prod",
			wantHost:     "prod.genai.us-phoenix-1.oci.oraclecloud.com",
			wantScheme:   "https",
			wantPath:     "/v1/resource",
			wantRawQuery: "foo=bar",
		},
		{
			name:       "preprod - ppe prefix",
			serviceURL: "https://genai.us-ashburn-1.oci.oraclecloud.com/v1",
			envType:    "preprod",
			wantHost:   "ppe.genai.us-ashburn-1.oci.oraclecloud.com",
			wantScheme: "https",
			wantPath:   "/v1",
		},
		{
			name:       "stage - custom prefix",
			serviceURL: "https://genai.eu-frankfurt-1.oci.oraclecloud.com/",
			envType:    "stage",
			wantHost:   "stage.genai.eu-frankfurt-1.oci.oraclecloud.com",
			wantScheme: "https",
			wantPath:   "/",
		},
		{
			name:         "path and query preserved",
			serviceURL:   "https://genai.us-sanjose-1.oci.oraclecloud.com/api?x=1&y=2",
			envType:      "dev",
			wantHost:     "dev.genai.us-sanjose-1.oci.oraclecloud.com",
			wantScheme:   "https",
			wantPath:     "/api",
			wantRawQuery: "x=1&y=2",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := getServiceEndpoint(tt.serviceURL, tt.envType)
			u, err := url.Parse(got)
			require.NoError(t, err)
			assert.Equal(t, tt.wantHost, u.Host)
			assert.Equal(t, tt.wantScheme, u.Scheme)
			assert.Equal(t, tt.wantPath, u.Path)
			if tt.wantRawQuery != "" {
				assert.Equal(t, tt.wantRawQuery, u.RawQuery)
			}
		})
	}
}

func Test_getGenAIClientWithDeps(t *testing.T) {
	t.Parallel()
	makeProvider := func(err error) configProviderFunc {
		return func(_ /*OciConfigPath*/, _ /*profile*/, _ string) (common.ConfigurationProvider, error) {
			if err != nil {
				return nil, err
			}
			return &struct{ common.ConfigurationProvider }{}, nil
		}
	}
	makeClient := func(err error, _ *string, _ *string) genAIClientFunc {
		return func(common.ConfigurationProvider) (generativeai.GenerativeAiClient, error) {
			if err != nil {
				return generativeai.GenerativeAiClient{}, err
			}
			// Return a struct that allows SetRegion and Host assignment
			return generativeai.GenerativeAiClient{}, nil
		}
	}

	t.Run("provider error", func(t *testing.T) {
		t.Parallel()
		env := models.Environment{Type: "prod", Region: "us-phoenix-1", Realm: "oc1"}
		client, err := getGenAIClientWithDeps(env, makeProvider(errors.New("fail provider")), makeClient(nil, nil, nil))
		assert.Nil(t, client)
		assert.ErrorContains(t, err, "failed to get OCI config provider")
	})

	t.Run("client error", func(t *testing.T) {
		t.Parallel()
		env := models.Environment{Type: "prod", Region: "us-phoenix-1", Realm: "oc1"}
		client, err := getGenAIClientWithDeps(env, makeProvider(nil), makeClient(errors.New("fail client"), nil, nil))
		assert.Nil(t, client)
		assert.ErrorContains(t, err, "failed to create GenerativeAI client")
	})

	t.Run("success prod region/host", func(t *testing.T) {
		t.Parallel()
		env := models.Environment{Type: "prod", Region: "us-phoenix-1", Realm: "oc1"}
		clientFn := func(common.ConfigurationProvider) (generativeai.GenerativeAiClient, error) {
			c := generativeai.GenerativeAiClient{}
			c.SetRegion(env.Region)
			return c, nil
		}
		client, err := getGenAIClientWithDeps(env, makeProvider(nil), clientFn)
		require.NoError(t, err)
		assert.NotNil(t, client)
	})

	t.Run("success non-prod sets host", func(t *testing.T) {
		t.Parallel()
		env := models.Environment{Type: "dev", Region: "us-ashburn-1", Realm: "oc1"}
		clientFn := func(common.ConfigurationProvider) (generativeai.GenerativeAiClient, error) {
			c := generativeai.GenerativeAiClient{}
			c.SetRegion(env.Region)
			// Simulate Endpoint() returning a known value
			c.Host = "generativeai.us-ashburn-1.oci.oraclecloud.com"
			return c, nil
		}
		client, err := getGenAIClientWithDeps(env, makeProvider(nil), clientFn)
		require.NoError(t, err)
		assert.NotNil(t, client)
		// Host should be overridden with prefix
		assert.Contains(t, client.Host, "dev.generativeai.us-ashburn-1.oci.oraclecloud.com")
	})
}
