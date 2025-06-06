package main

import (
	"flag"
	"os"
	"testing"

	"github.com/jingle2008/toolkit/internal/domain"
)

func TestCategoryFromString_Valid(t *testing.T) {
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
		got, err := categoryFromString(tt.input)
		if err != nil {
			t.Errorf("categoryFromString(%q) returned error: %v", tt.input, err)
		}
		if got != tt.expected {
			t.Errorf("categoryFromString(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestCategoryFromString_Invalid(t *testing.T) {
	_, err := categoryFromString("notacategory")
	if err == nil {
		t.Errorf("expected error for invalid category, got nil")
	}
}

func TestGetEnvOrDefault(t *testing.T) {
	const key = "TOOLKIT_TEST_ENV"
	// Ensure variable is unset
	_ = unsetEnv(key)
	got := getEnvOrDefault(key, "default")
	if got != "default" {
		t.Errorf("getEnvOrDefault (unset) = %q, want %q", got, "default")
	}
	// Set variable
	t.Setenv(key, "value")
	got = getEnvOrDefault(key, "default")
	if got != "value" {
		t.Errorf("getEnvOrDefault (set) = %q, want %q", got, "value")
	}
}

func unsetEnv(key string) error {
	return nil // t.Setenv will handle unsetting in Go 1.17+
}

func TestParseConfig_Defaults(t *testing.T) {
	// Save and restore os.Args and flag.CommandLine
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = []string{"cmd"}

	// Unset all relevant env vars
	envVars := []string{
		"TOOLKIT_REPO_PATH", "KUBECONFIG", "TOOLKIT_ENV_TYPE",
		"TOOLKIT_ENV_REGION", "TOOLKIT_ENV_REALM", "TOOLKIT_CATEGORY",
	}
	for _, v := range envVars {
		t.Setenv(v, "")
	}

	// Reset flags
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	cfg := parseConfig()
	if cfg.RepoPath == "" || cfg.KubeConfig == "" || cfg.EnvType == "" || cfg.EnvRegion == "" || cfg.EnvRealm == "" || cfg.Category == "" {
		t.Errorf("parseConfig returned empty fields: %+v", cfg)
	}
}

func TestParseConfig_EnvOverride(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = []string{"cmd"}

	// Set env vars
	t.Setenv("TOOLKIT_REPO_PATH", "/tmp/repo")
	t.Setenv("KUBECONFIG", "/tmp/kubeconfig")
	t.Setenv("TOOLKIT_ENV_TYPE", "prod")
	t.Setenv("TOOLKIT_ENV_REGION", "eu-west-1")
	t.Setenv("TOOLKIT_ENV_REALM", "oc2")
	t.Setenv("TOOLKIT_CATEGORY", "GpuNode")

	// Reset flags
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	cfg := parseConfig()
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
