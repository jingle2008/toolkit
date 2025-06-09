package config

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
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

func writeTempFile(t *testing.T, ext, content string) string {
	t.Helper()
	f, err := os.CreateTemp("", "toolkit_config_*"+ext)
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	f.Close() //nolint:errcheck,gosec // ignore error on close in test cleanup
	return f.Name()
}

func TestLoadConfigFile_YAML(t *testing.T) {
	t.Parallel()
	cfg := Config{
		RepoPath:   "repo",
		KubeConfig: "kube",
		EnvType:    "type",
		EnvRegion:  "region",
		EnvRealm:   "realm",
		Category:   "Tenant",
	}
	data, _ := yaml.Marshal(cfg)
	path := writeTempFile(t, ".yaml", string(data))
	defer os.Remove(path) //nolint:errcheck,gosec // ignore error on remove in test cleanup

	got, err := loadConfigFile(path)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.RepoPath != "repo" || got.KubeConfig != "kube" {
		t.Errorf("unexpected config: %+v", got)
	}
}

func TestLoadConfigFile_JSON(t *testing.T) {
	t.Parallel()
	cfg := Config{
		RepoPath:   "repo",
		KubeConfig: "kube",
		EnvType:    "type",
		EnvRegion:  "region",
		EnvRealm:   "realm",
		Category:   "Tenant",
	}
	data, _ := json.Marshal(cfg)
	path := writeTempFile(t, ".json", string(data))
	defer os.Remove(path) //nolint:errcheck,gosec // ignore error on remove in test cleanup

	got, err := loadConfigFile(path)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.RepoPath != "repo" || got.KubeConfig != "kube" {
		t.Errorf("unexpected config: %+v", got)
	}
}

func TestLoadConfigFile_UnsupportedExt(t *testing.T) {
	t.Parallel()
	path := writeTempFile(t, ".txt", "irrelevant")
	defer os.Remove(path) //nolint:errcheck,gosec // ignore error on remove in test cleanup
	_, err := loadConfigFile(path)
	if err == nil || !strings.Contains(err.Error(), "unsupported config file extension") {
		t.Errorf("expected unsupported extension error, got %v", err)
	}
}

func TestLoadConfigFile_FileNotFound(t *testing.T) {
	t.Parallel()
	_, err := loadConfigFile("/nonexistent/path.yaml")
	if err == nil {
		t.Errorf("expected error for missing file, got nil")
	}
}

func TestLoadConfigFile_DecodeError(t *testing.T) {
	t.Parallel()
	path := writeTempFile(t, ".yaml", "not: valid: yaml: [")
	defer os.Remove(path) //nolint:errcheck,gosec // ignore error on remove in test cleanup
	_, err := loadConfigFile(path)
	if err == nil {
		t.Errorf("expected decode error, got nil")
	}
}

func TestParse_WithConfigFile(t *testing.T) { //nolint:paralleltest
	cfg := Config{
		RepoPath:   "repo",
		KubeConfig: "kube",
		EnvType:    "type",
		EnvRegion:  "region",
		EnvRealm:   "realm",
		Category:   "Tenant",
	}
	data, _ := yaml.Marshal(cfg)
	path := writeTempFile(t, ".yaml", string(data))
	defer os.Remove(path) //nolint:errcheck,gosec // ignore error on remove in test cleanup

	// Unset env vars to avoid overlaying config file values
	t.Setenv("TOOLKIT_REPO_PATH", "")
	t.Setenv("KUBECONFIG", "")
	t.Setenv("TOOLKIT_ENV_TYPE", "")
	t.Setenv("TOOLKIT_ENV_REGION", "")
	t.Setenv("TOOLKIT_ENV_REALM", "")
	t.Setenv("TOOLKIT_CATEGORY", "")

	args := []string{
		"-config", path,
		"-repo", "",
		"-kubeconfig", "",
		"-envtype", "",
		"-envregion", "",
		"-envrealm", "",
		"-category", "",
	}
	got := Parse(args)
	if got.RepoPath != "repo" || got.KubeConfig != "kube" || got.EnvType != "type" ||
		got.EnvRegion != "region" || got.EnvRealm != "realm" || got.Category != "Tenant" {
		t.Errorf("expected config from file, got %+v", got)
	}
	if got.ConfigFile != path {
		t.Errorf("expected ConfigFile to be set, got %q", got.ConfigFile)
	}
}

func TestParse_ConfigFileNotFound(t *testing.T) {
	t.Parallel()
	args := []string{"-config", "/nonexistent/path.yaml"}
	got := Parse(args)
	if got.ConfigFile != "/nonexistent/path.yaml" {
		t.Errorf("expected ConfigFile to be set even if file not found, got %q", got.ConfigFile)
	}
}

func TestParse_ConfigFileOverlay(t *testing.T) {
	t.Parallel()
	cfg := Config{
		RepoPath:   "repo",
		KubeConfig: "kube",
		EnvType:    "type",
		EnvRegion:  "region",
		EnvRealm:   "realm",
		Category:   "Tenant",
	}
	data, _ := yaml.Marshal(cfg)
	path := writeTempFile(t, ".yaml", string(data))
	defer os.Remove(path) //nolint:errcheck // ignore error on remove in test cleanup

	args := []string{"-config", path, "-repo", "flagrepo"}
	got := Parse(args)
	if got.RepoPath != "flagrepo" {
		t.Errorf("flag should override config file, got %q", got.RepoPath)
	}
}

func TestMergeConfig_AllFields(t *testing.T) {
	t.Parallel()
	dst := Config{}
	src := Config{
		ConfigFile: "f", RepoPath: "r", KubeConfig: "k", EnvType: "t", EnvRegion: "g", EnvRealm: "m", Category: "c",
	}
	got := mergeConfig(dst, src)
	if got != src {
		t.Errorf("expected all fields to be merged, got %+v", got)
	}
}
