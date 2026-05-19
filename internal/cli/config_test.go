//nolint:paralleltest // NewRootCmd uses cobra/viper global state
package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

func TestConfigCmd_YAML_DefaultPath(t *testing.T) {
	// HOME → tempdir so the default config path resolves to a file that
	// doesn't exist. The command should still succeed (exists: false)
	// and print the resolved path so users know where to put it.
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	viper.Reset()
	t.Cleanup(viper.Reset)

	cmd := NewRootCmd("vtest")
	cmd.SetArgs([]string{"config"})
	stdout := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(new(bytes.Buffer))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("toolkit config: %v", err)
	}

	out := stdout.String()
	wantPath := filepath.Join(tmp, ".config", "toolkit", "config.yaml")
	if !strings.Contains(out, "config_file: "+wantPath) {
		t.Errorf("expected config_file %q in output:\n%s", wantPath, out)
	}
	if !strings.Contains(out, "exists: false") {
		t.Errorf("expected exists: false (no config file scaffolded), got:\n%s", out)
	}
	if !strings.Contains(out, "settings:") {
		t.Errorf("expected settings block in output:\n%s", out)
	}
	// At least one bound flag should appear as a key.
	if !strings.Contains(out, "repo_path:") {
		t.Errorf("expected repo_path key in settings, got:\n%s", out)
	}
}

func TestConfigCmd_JSON_ParsesCleanly(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	viper.Reset()
	t.Cleanup(viper.Reset)

	cmd := NewRootCmd("vtest")
	cmd.SetArgs([]string{"config", "-o", "json"})
	stdout := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(new(bytes.Buffer))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("toolkit config -o json: %v", err)
	}

	var view struct {
		ConfigFile string         `json:"config_file"`
		Exists     bool           `json:"exists"`
		Settings   map[string]any `json:"settings"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &view); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, stdout.String())
	}
	if view.ConfigFile == "" {
		t.Error("config_file should be set in JSON output")
	}
	if _, ok := view.Settings["repo_path"]; !ok {
		t.Errorf("expected repo_path in settings, got: %+v", view.Settings)
	}
}

func TestConfigCmd_ReadsExistingFile(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "cfg.yaml")
	contents := []byte("repo_path: /from/file\nenv_realm: oc-stage\n")
	if err := os.WriteFile(cfgPath, contents, 0o600); err != nil {
		t.Fatalf("seed config: %v", err)
	}
	viper.Reset()
	t.Cleanup(viper.Reset)

	cmd := NewRootCmd("vtest")
	cmd.SetArgs([]string{"--config", cfgPath, "config", "-o", "json"})
	stdout := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(new(bytes.Buffer))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("toolkit config: %v", err)
	}

	var view struct {
		ConfigFile string         `json:"config_file"`
		Exists     bool           `json:"exists"`
		Settings   map[string]any `json:"settings"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &view); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, stdout.String())
	}
	if view.ConfigFile != cfgPath {
		t.Errorf("config_file = %q, want %q", view.ConfigFile, cfgPath)
	}
	if !view.Exists {
		t.Error("exists should be true when --config points at a real file")
	}
	if got := view.Settings["repo_path"]; got != "/from/file" {
		t.Errorf("repo_path = %v, want /from/file", got)
	}
	if got := view.Settings["env_realm"]; got != "oc-stage" {
		t.Errorf("env_realm = %v, want oc-stage", got)
	}
}

func TestConfigCmd_InvalidFormat(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	viper.Reset()
	t.Cleanup(viper.Reset)

	cmd := NewRootCmd("vtest")
	cmd.SetArgs([]string{"config", "-o", "toml"})
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid format, got nil")
	}
	if !strings.Contains(err.Error(), "invalid output format") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestConfigCmd_YAMLDecodes(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	viper.Reset()
	t.Cleanup(viper.Reset)

	cmd := NewRootCmd("vtest")
	cmd.SetArgs([]string{"config"})
	stdout := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(new(bytes.Buffer))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("toolkit config: %v", err)
	}

	var view struct {
		ConfigFile string         `yaml:"config_file"`
		Exists     bool           `yaml:"exists"`
		Settings   map[string]any `yaml:"settings"`
	}
	if err := yaml.Unmarshal(stdout.Bytes(), &view); err != nil {
		t.Fatalf("stdout is not valid YAML: %v\n%s", err, stdout.String())
	}
	if view.Settings == nil {
		t.Error("expected non-nil settings map after YAML decode")
	}
}
