//nolint:paralleltest // NewRootCmd uses cobra global state
package cli

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestGetCmd_UnknownCategory(t *testing.T) {
	cmd := NewRootCmd("vtest")
	cmd.SetArgs([]string{"get", "totally-not-a-thing"})
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for unknown category, got nil")
	}
	if !strings.Contains(err.Error(), "unknown category") {
		t.Errorf("expected unknown category error, got: %v", err)
	}
}

func TestGetCmd_InvalidOutput(t *testing.T) {
	cmd := NewRootCmd("vtest")
	cmd.SetArgs([]string{"get", "tenant", "-o", "toml"})
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid output format, got nil")
	}
	if !strings.Contains(err.Error(), "invalid output format") {
		t.Errorf("expected invalid output format error, got: %v", err)
	}
}

func TestGetCmd_MissingRequiredSettings(t *testing.T) {
	// Point HOME at a tempdir so the default ~/.config/toolkit/config.yaml
	// resolves to a path that doesn't exist, and clear any viper state
	// inherited from other tests in this package.
	t.Setenv("HOME", t.TempDir())
	viper.Reset()
	t.Cleanup(viper.Reset)

	cmd := NewRootCmd("vtest")
	cmd.SetArgs([]string{"get", "tenant"})
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when required settings are missing, got nil")
	}
	// Error should name the specific missing flags and NOT mention "Category"
	// (which the positional arg already supplies).
	msg := err.Error()
	if !strings.Contains(msg, "--repo_path") {
		t.Errorf("expected --repo_path in error, got: %v", err)
	}
	if strings.Contains(strings.ToLower(msg), "category is required") {
		t.Errorf("error should not mention Category, got: %v", err)
	}
}

func TestGetCmd_AliasJSON_HappyPath(t *testing.T) {
	// Scope viper away from the user's real ~/.config/toolkit so the test
	// doesn't pick up stray repo_path / env values. Alias is a static
	// enum dump — no loader call — so HOME is the only thing to isolate.
	t.Setenv("HOME", t.TempDir())
	viper.Reset()
	t.Cleanup(viper.Reset)

	cmd := NewRootCmd("vtest")
	cmd.SetArgs([]string{"get", "alias", "-o", "json"})
	stdout := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(new(bytes.Buffer))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("get alias: %v", err)
	}

	var items []struct {
		Alias    string `json:"alias"`
		Category string `json:"category"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &items); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, stdout.String())
	}
	if len(items) == 0 {
		t.Fatal("expected at least one alias in JSON output")
	}
	for _, it := range items {
		if it.Alias == "" || it.Category == "" {
			t.Errorf("entry missing alias or category: %+v", it)
		}
	}
}

func TestGetCmd_HelpListsExamples(t *testing.T) {
	cmd := NewRootCmd("vtest")
	cmd.SetArgs([]string{"get", "--help"})
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("get --help: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"json", "jsonl", "yaml", "table", "csv", "tsv", "-o"} {
		if !strings.Contains(out, want) {
			t.Errorf("get --help missing %q in output:\n%s", want, out)
		}
	}
}

func TestGetCmd_AliasCSV_HappyPath(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	viper.Reset()
	t.Cleanup(viper.Reset)

	cmd := NewRootCmd("vtest")
	cmd.SetArgs([]string{"get", "alias", "-o", "csv"})
	stdout := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(new(bytes.Buffer))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("get alias -o csv: %v", err)
	}

	records, err := csv.NewReader(stdout).ReadAll()
	if err != nil {
		t.Fatalf("stdout is not valid CSV: %v", err)
	}
	if len(records) < 2 {
		t.Fatalf("expected header + at least one alias row, got %d records", len(records))
	}
	if records[0][0] != "ALIAS" || records[0][1] != "CATEGORY" {
		t.Errorf("unexpected header row: %+v", records[0])
	}
}

func TestGetCmd_AliasTSV_HappyPath(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	viper.Reset()
	t.Cleanup(viper.Reset)

	cmd := NewRootCmd("vtest")
	cmd.SetArgs([]string{"get", "alias", "-o", "tsv"})
	stdout := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(new(bytes.Buffer))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("get alias -o tsv: %v", err)
	}

	lines := strings.Split(strings.TrimRight(stdout.String(), "\n"), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected header + at least one alias row, got %d lines", len(lines))
	}
	if lines[0] != "ALIAS\tCATEGORY" {
		t.Errorf("unexpected header line: %q", lines[0])
	}
	// Every data row must have exactly one tab (two columns).
	for _, line := range lines[1:] {
		if strings.Count(line, "\t") != 1 {
			t.Errorf("expected one tab per data row, got: %q", line)
		}
	}
}
