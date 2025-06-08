package main

import (
	"testing"

	"github.com/jingle2008/toolkit/internal/config"
	"github.com/jingle2008/toolkit/internal/domain"
)

func TestCategoryFromString_Valid(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input    string
		expected domain.Category
	}{
		{"tenant", domain.Tenant},
		{"LimitDefinition", domain.LimitDefinition},
		{"consolepropertydefinition", domain.ConsolePropertyDefinition},
		{"PropertyDefinition", domain.PropertyDefinition},
		{"limittenancyoverride", domain.LimitTenancyOverride},
		{"ConsolePropertyTenancyOverride", domain.ConsolePropertyTenancyOverride},
		{"propertytenancyoverride", domain.PropertyTenancyOverride},
		{"consolepropertyregionaloverride", domain.ConsolePropertyRegionalOverride},
		{"PropertyRegionalOverride", domain.PropertyRegionalOverride},
		{"basemodel", domain.BaseModel},
		{"ModelArtifact", domain.ModelArtifact},
		{"environment", domain.Environment},
		{"ServiceTenancy", domain.ServiceTenancy},
		{"gpupool", domain.GpuPool},
		{"GpuNode", domain.GpuNode},
		{"DedicatedAICluster", domain.DedicatedAICluster},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got, err := categoryFromString(tt.input)
			if err != nil {
				t.Errorf("categoryFromString(%q) returned error: %v", tt.input, err)
			}
			if got != tt.expected {
				t.Errorf("categoryFromString(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestCategoryFromString_Invalid(t *testing.T) {
	t.Parallel()
	invalidInputs := []string{"notacategory", "", "123", "fooBar"}
	for _, input := range invalidInputs {
		input := input
		t.Run(input, func(t *testing.T) {
			t.Parallel()
			_, err := categoryFromString(input)
			if err == nil {
				t.Errorf("expected error for invalid category %q, got nil", input)
			}
		})
	}
}

func TestParseConfig_Defaults(t *testing.T) {
	t.Parallel()
	args := []string{"toolkit"}

	envVars := []string{
		"TOOLKIT_REPO_PATH", "KUBECONFIG", "TOOLKIT_ENV_TYPE",
		"TOOLKIT_ENV_REGION", "TOOLKIT_ENV_REALM", "TOOLKIT_CATEGORY",
	}
	for _, v := range envVars {
		t.Setenv(v, "")
	}

	cfg := config.Parse(args)
	if cfg.RepoPath == "" || cfg.KubeConfig == "" || cfg.EnvType == "" || cfg.EnvRegion == "" || cfg.EnvRealm == "" || cfg.Category == "" {
		t.Errorf("config.Parse returned empty fields: %+v", cfg)
	}
}

func TestParseConfig_EnvOverride(t *testing.T) {
	args := []string{"toolkit"}

	t.Setenv("TOOLKIT_REPO_PATH", "/tmp/repo")
	t.Setenv("KUBECONFIG", "/tmp/kubeconfig")
	t.Setenv("TOOLKIT_ENV_TYPE", "prod")
	t.Setenv("TOOLKIT_ENV_REGION", "eu-west-1")
	t.Setenv("TOOLKIT_ENV_REALM", "oc2")
	t.Setenv("TOOLKIT_CATEGORY", "GpuNode")

	cfg := config.Parse(args)
	if cfg.RepoPath != "/tmp/repo" {
		t.Errorf("RepoPath = %q, want /tmp/repo", cfg.RepoPath)
	}
	if cfg.KubeConfig != "/tmp/kubeconfig" {
		t.Errorf("KubeConfig = %q, want /tmp/kubeconfig", cfg.KubeConfig)
	}
	if cfg.EnvType != "prod" {
		t.Errorf("EnvType = %q, want prod", cfg.EnvType)
	}
	if cfg.EnvRegion != "eu-west-1" {
		t.Errorf("EnvRegion = %q, want eu-west-1", cfg.EnvRegion)
	}
	if cfg.EnvRealm != "oc2" {
		t.Errorf("EnvRealm = %q, want oc2", cfg.EnvRealm)
	}
	if cfg.Category != "GpuNode" {
		t.Errorf("Category = %q, want GpuNode", cfg.Category)
	}
}
