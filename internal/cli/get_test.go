//nolint:paralleltest // NewRootCmd uses cobra global state
package cli

import (
	"bytes"
	"strings"
	"testing"
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
	for _, want := range []string{"json", "jsonl", "yaml", "table", "-o"} {
		if !strings.Contains(out, want) {
			t.Errorf("get --help missing %q in output:\n%s", want, out)
		}
	}
}
