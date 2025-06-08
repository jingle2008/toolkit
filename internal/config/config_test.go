package config

import (
	"reflect"
	"testing"
)

func TestConfig_Validate(t *testing.T) {
	t.Parallel()
	valid := Config{
		RepoPath:   "repo",
		KubeConfig: "kube",
		EnvType:    "type",
		EnvRegion:  "region",
		EnvRealm:   "realm",
		Category:   "Tenant",
	}
	if err := valid.Validate(); err != nil {
		t.Errorf("expected valid config, got error: %v", err)
	}

	// Each required field missing
	fields := []string{"RepoPath", "KubeConfig", "EnvType", "EnvRegion", "EnvRealm", "Category"}
	for _, f := range fields {
		cfg := valid
		reflect.ValueOf(&cfg).Elem().FieldByName(f).SetString("")
		err := cfg.Validate()
		if err == nil {
			t.Errorf("expected error for missing %s, got nil", f)
		}
	}

	// Invalid category
	cfg := valid
	cfg.Category = "notacategory"
	err := cfg.Validate()
	if err == nil || !contains(err.Error(), "unknown category") {
		t.Errorf("expected invalid category error, got: %v", err)
	}
}

// contains reports whether substr is in s.
func contains(s, substr string) bool {
	return len(substr) == 0 || (len(s) >= len(substr) && (s == substr || contains(s[1:], substr)))
}

func TestParseArgs_Priority(t *testing.T) {
	args := []string{
		"-repo", "flagrepo",
		"-kubeconfig", "flagkube",
		"-envtype", "flagtype",
		"-envregion", "flagregion",
		"-envrealm", "flagrealm",
		"-category", "Tenant",
	}

	t.Setenv("TOOLKIT_REPO_PATH", "envrepo")
	t.Setenv("KUBECONFIG", "envkube")
	t.Setenv("TOOLKIT_ENV_TYPE", "envtype")
	t.Setenv("TOOLKIT_ENV_REGION", "envregion")
	t.Setenv("TOOLKIT_ENV_REALM", "envrealm")
	t.Setenv("TOOLKIT_CATEGORY", "Tenant")

	cfg := Parse(args)
	if cfg.RepoPath != "flagrepo" {
		t.Errorf("flag should override env for RepoPath, got %q", cfg.RepoPath)
	}
	if cfg.KubeConfig != "flagkube" {
		t.Errorf("flag should override env for KubeConfig, got %q", cfg.KubeConfig)
	}
	if cfg.EnvType != "flagtype" {
		t.Errorf("flag should override env for EnvType, got %q", cfg.EnvType)
	}
	if cfg.EnvRegion != "flagregion" {
		t.Errorf("flag should override env for EnvRegion, got %q", cfg.EnvRegion)
	}
	if cfg.EnvRealm != "flagrealm" {
		t.Errorf("flag should override env for EnvRealm, got %q", cfg.EnvRealm)
	}
	if cfg.Category != "Tenant" {
		t.Errorf("flag should override env for Category, got %q", cfg.Category)
	}
}
