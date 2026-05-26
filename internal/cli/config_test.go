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
	"sigs.k8s.io/yaml"
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
	if !strings.Contains(out, "repo-path:") {
		t.Errorf("expected repo-path key in settings, got:\n%s", out)
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
	if _, ok := view.Settings["repo-path"]; !ok {
		t.Errorf("expected repo-path in settings, got: %+v", view.Settings)
	}
	// The persistent --config flag is bound to viper, but writeConfigView
	// strips it from settings to avoid a redundant copy of ConfigFile.
	if _, ok := view.Settings["config"]; ok {
		t.Errorf("settings should not contain redundant 'config' key, got: %+v", view.Settings)
	}
}

func TestConfigCmd_EmptyConfigFlag(t *testing.T) {
	// `--config ""` disables the config-file read path. The command must
	// still produce a valid view (exists: false, empty config_file) rather
	// than panicking or returning an error.
	t.Setenv("HOME", t.TempDir())
	viper.Reset()
	t.Cleanup(viper.Reset)

	cmd := NewRootCmd("vtest")
	cmd.SetArgs([]string{"--config", "", "config", "-o", "json"})
	stdout := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(new(bytes.Buffer))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("toolkit --config '' config: %v", err)
	}

	var view struct {
		ConfigFile string         `json:"config_file"`
		Exists     bool           `json:"exists"`
		Settings   map[string]any `json:"settings"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &view); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, stdout.String())
	}
	if view.ConfigFile != "" {
		t.Errorf("config_file = %q, want empty", view.ConfigFile)
	}
	if view.Exists {
		t.Error("exists should be false when --config is empty")
	}
}

func TestConfigCmd_PrettyFalseProducesCompactJSON(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	viper.Reset()
	t.Cleanup(viper.Reset)

	cmd := NewRootCmd("vtest")
	cmd.SetArgs([]string{"config", "-o", "json", "--pretty=false"})
	stdout := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(new(bytes.Buffer))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("toolkit config -o json --pretty=false: %v", err)
	}

	out := stdout.String()
	// Pretty-printed JSON contains a newline-indent sequence. Compact
	// must not.
	if strings.Contains(out, "\n  ") {
		t.Errorf("expected compact JSON, got pretty:\n%s", out)
	}
	// Output should still parse as one JSON value.
	var view struct {
		ConfigFile string `json:"config_file"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &view); err != nil {
		t.Fatalf("compact JSON output not parseable: %v\n%s", err, out)
	}
}

func TestConfigCmd_ReadsExistingFile(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "cfg.yaml")
	contents := []byte("repo-path: /from/file\nenv-realm: oc-stage\n")
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
	if got := view.Settings["repo-path"]; got != "/from/file" {
		t.Errorf("repo-path = %v, want /from/file", got)
	}
	if got := view.Settings["env-realm"]; got != "oc-stage" {
		t.Errorf("env-realm = %v, want oc-stage", got)
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

func TestConfigCmd_ValidatePasses(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "cfg.yaml")
	contents := []byte("repo-path: /tmp/repo\n" +
		"env-type: dev\n" +
		"env-region: us-phoenix-1\n" +
		"env-realm: oc1\n" +
		"category: tenant\n")
	if err := os.WriteFile(cfgPath, contents, 0o600); err != nil {
		t.Fatalf("seed config: %v", err)
	}
	t.Setenv("HOME", tmp)
	viper.Reset()
	t.Cleanup(viper.Reset)

	cmd := NewRootCmd("vtest")
	cmd.SetArgs([]string{"--config", cfgPath, "config", "--validate", "-o", "json"})
	stdout := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(new(bytes.Buffer))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("validate should pass, got: %v\nstdout: %s", err, stdout.String())
	}

	var view struct {
		Valid      bool   `json:"valid"`
		ConfigFile string `json:"config_file"`
		Error      string `json:"error"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &view); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, stdout.String())
	}
	if !view.Valid {
		t.Errorf("valid = false, want true; error: %q", view.Error)
	}
	if view.ConfigFile != cfgPath {
		t.Errorf("config_file = %q, want %q", view.ConfigFile, cfgPath)
	}
	if view.Error != "" {
		t.Errorf("error should be empty on pass, got: %q", view.Error)
	}
}

func TestConfigCmd_ValidateFailsOnMissingFields(t *testing.T) {
	// Seed a config that's syntactically valid but missing required
	// fields. cfg.Validate() should reject it; the command should write
	// the structured failure on stdout AND return a non-nil error to set
	// exit code.
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "cfg.yaml")
	if err := os.WriteFile(cfgPath, []byte("env-type: dev\n"), 0o600); err != nil {
		t.Fatalf("seed config: %v", err)
	}
	t.Setenv("HOME", tmp)
	viper.Reset()
	t.Cleanup(viper.Reset)

	cmd := NewRootCmd("vtest")
	cmd.SetArgs([]string{"--config", cfgPath, "config", "--validate", "-o", "json"})
	stdout := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(new(bytes.Buffer))
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected validation failure to return non-nil error, got nil")
	}
	if !strings.Contains(err.Error(), "config validation failed") {
		t.Errorf("unexpected error wrapper: %v", err)
	}

	// Structured output is still emitted before the error path returns,
	// so consumers can parse `.valid` and `.error` from stdout.
	var view struct {
		Valid bool   `json:"valid"`
		Error string `json:"error"`
	}
	if jerr := json.Unmarshal(stdout.Bytes(), &view); jerr != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", jerr, stdout.String())
	}
	if view.Valid {
		t.Error("valid should be false")
	}
	if view.Error == "" {
		t.Error("error field should be populated on failure")
	}
}

func TestConfigCmd_ValidateYAMLDefault(t *testing.T) {
	// Default format is yaml; verify the shape is sane and the failure
	// message reaches the human-readable output.
	t.Setenv("HOME", t.TempDir())
	viper.Reset()
	t.Cleanup(viper.Reset)

	cmd := NewRootCmd("vtest")
	cmd.SetArgs([]string{"config", "--validate"})
	stdout := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(new(bytes.Buffer))
	// Empty HOME means no defaults from a real file → validation fails.
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected validation failure under empty HOME, got nil")
	}
	out := stdout.String()
	if !strings.Contains(out, "valid: false") {
		t.Errorf("expected `valid: false` in YAML output, got:\n%s", out)
	}
	if !strings.Contains(out, "error:") {
		t.Errorf("expected `error:` line in YAML output, got:\n%s", out)
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
		ConfigFile string         `json:"config_file"`
		Exists     bool           `json:"exists"`
		Settings   map[string]any `json:"settings"`
	}
	if err := yaml.Unmarshal(stdout.Bytes(), &view); err != nil {
		t.Fatalf("stdout is not valid YAML: %v\n%s", err, stdout.String())
	}
	if view.Settings == nil {
		t.Error("expected non-nil settings map after YAML decode")
	}
}
