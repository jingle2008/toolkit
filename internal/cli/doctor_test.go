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
)

// fullCfgContents returns a YAML config that satisfies cfg.Validate(),
// pointing repo_path/kubeconfig at the provided real paths.
func fullCfgContents(repoPath, kubeconfig string) []byte {
	return []byte(
		"repo_path: " + repoPath + "\n" +
			"kubeconfig: " + kubeconfig + "\n" +
			"env_type: dev\n" +
			"env_region: us-phoenix-1\n" +
			"env_realm: oc1\n" +
			"category: tenant\n",
	)
}

func TestDoctorCmd_AllPass(t *testing.T) {
	tmp := t.TempDir()
	repoDir := filepath.Join(tmp, "repo")
	if err := os.Mkdir(repoDir, 0o750); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}
	kubePath := filepath.Join(tmp, "kube.yaml")
	if err := os.WriteFile(kubePath, []byte("placeholder"), 0o600); err != nil {
		t.Fatalf("seed kubeconfig: %v", err)
	}
	cfgPath := filepath.Join(tmp, "cfg.yaml")
	if err := os.WriteFile(cfgPath, fullCfgContents(repoDir, kubePath), 0o600); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	t.Setenv("HOME", tmp)
	viper.Reset()
	t.Cleanup(viper.Reset)

	cmd := NewRootCmd("vtest")
	cmd.SetArgs([]string{"--config", cfgPath, "doctor", "-o", "json"})
	stdout := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(new(bytes.Buffer))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("doctor should pass, got: %v\nstdout: %s", err, stdout.String())
	}

	var results []checkResult
	if err := json.Unmarshal(stdout.Bytes(), &results); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, stdout.String())
	}
	if len(results) == 0 {
		t.Fatal("expected at least one check, got zero")
	}
	for _, r := range results {
		if r.Status == statusFail {
			t.Errorf("check %s failed in happy path: %+v", r.Name, r)
		}
	}
}

func TestDoctorCmd_FailsOnMissingRepoPath(t *testing.T) {
	// Config schema passes (repo_path is non-empty) but the path doesn't
	// exist on disk → repo_path check should FAIL and command exit
	// non-zero.
	tmp := t.TempDir()
	missingRepo := filepath.Join(tmp, "does-not-exist")
	kubePath := filepath.Join(tmp, "kube.yaml")
	if err := os.WriteFile(kubePath, []byte("placeholder"), 0o600); err != nil {
		t.Fatalf("seed kubeconfig: %v", err)
	}
	cfgPath := filepath.Join(tmp, "cfg.yaml")
	if err := os.WriteFile(cfgPath, fullCfgContents(missingRepo, kubePath), 0o600); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	t.Setenv("HOME", tmp)
	viper.Reset()
	t.Cleanup(viper.Reset)

	cmd := NewRootCmd("vtest")
	cmd.SetArgs([]string{"--config", cfgPath, "doctor", "-o", "json"})
	stdout := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(new(bytes.Buffer))
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected non-zero exit on FAIL, got nil")
	}
	if !strings.Contains(err.Error(), "doctor") {
		t.Errorf("unexpected error: %v", err)
	}

	var results []checkResult
	if jerr := json.Unmarshal(stdout.Bytes(), &results); jerr != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", jerr, stdout.String())
	}
	foundFail := false
	for _, r := range results {
		if r.Name == "repo_path" && r.Status == statusFail {
			foundFail = true
		}
	}
	if !foundFail {
		t.Errorf("expected repo_path FAIL row, got: %+v", results)
	}
}

func TestDoctorCmd_TableDefault(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	viper.Reset()
	t.Cleanup(viper.Reset)

	cmd := NewRootCmd("vtest")
	cmd.SetArgs([]string{"doctor"})
	stdout := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(new(bytes.Buffer))
	// Empty HOME guarantees at least the config_schema check fails, so
	// the command exits non-zero. We only care that the *table* renders.
	_ = cmd.Execute()

	out := stdout.String()
	for _, want := range []string{"CHECK", "STATUS", "DETAIL", "HINT", "config_schema"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in table output, got:\n%s", want, out)
		}
	}
}

func TestDoctorCmd_InvalidFormat(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	viper.Reset()
	t.Cleanup(viper.Reset)

	cmd := NewRootCmd("vtest")
	cmd.SetArgs([]string{"doctor", "-o", "csv"})
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid format")
	}
	if !strings.Contains(err.Error(), "invalid output format") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestCheckPath_TableDriven covers checkPath in isolation so we can
// assert each {required, value} permutation directly without dragging
// cobra/viper state into the picture.
func TestCheckPath_TableDriven(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "f"), []byte("x"), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	cases := []struct {
		name     string
		value    string
		required bool
		want     checkStatus
	}{
		{"required+empty", "", true, statusFail},
		{"required+missing", filepath.Join(tmp, "missing"), true, statusFail},
		{"required+present", filepath.Join(tmp, "f"), true, statusPass},
		{"optional+empty", "", false, statusSkip},
		{"optional+missing", filepath.Join(tmp, "missing"), false, statusFail},
		{"optional+present", filepath.Join(tmp, "f"), false, statusPass},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := checkPath("x", tc.value, tc.required, "hint")
			if got.Status != tc.want {
				t.Errorf("status = %s, want %s (detail=%q)", got.Status, tc.want, got.Detail)
			}
		})
	}
}
