//nolint:paralleltest // cobra uses global state → data race under t.Parallel
package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRootCmd_HelpOutput(t *testing.T) {
	cmd := NewRootCmd("vtest")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--help"})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, cmd.Use) {
		t.Errorf("help output missing command use: %q", cmd.Use)
	}
	if !strings.Contains(out, cmd.Short) {
		t.Errorf("help output missing short description: %q", cmd.Short)
	}
}

func TestRootCmd_UnknownFlag(t *testing.T) {
	cmd := NewRootCmd("vtest")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--unknownflag"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for unknown flag, got nil")
	}
	if !strings.Contains(buf.String(), "unknown flag") {
		t.Errorf("expected unknown flag error, got: %s", buf.String())
	}
}

func TestCompletion(t *testing.T) {
	shells := []string{"bash", "zsh", "fish"}
	for _, sh := range shells {
		cmd := NewRootCmd("vtest")
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"completion", sh})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("%s completion: %v", sh, err)
		}
		if buf.Len() == 0 {
			t.Fatalf("%s completion produced no output", sh)
		}
	}
}

func TestInitCreatesConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cmd := NewRootCmd("vtest")
	cmd.SetArgs([]string{"init"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("init: %v", err)
	}
	path := filepath.Join(home, ".config", "toolkit", "config.yaml")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("config.yaml not created: %v", err)
	}
}

func TestDefaultFlags(t *testing.T) {
	cmd := NewRootCmd("vtest")
	tests := []struct{ name, want string }{
		{"log_format", "console"},
		{"log_file", "toolkit.log"},
	}
	for _, tc := range tests {
		got, _ := cmd.PersistentFlags().GetString(tc.name)
		if got != tc.want {
			t.Errorf("%s default %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestVersionCommand(t *testing.T) {
	cmd := NewRootCmd("vtest")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"version"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("version: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "vtest") {
		t.Errorf("version output missing version: %q", out)
	}
}

func TestVersionCommandWithCheck(t *testing.T) {
	cmd := NewRootCmd("vtest")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"version", "--check"})
	// This will hit the real GitHub API, so in a real test, you'd want to monkey-patch fetchLatestRelease.
	// For now, just check that it doesn't panic and prints something.
	_ = cmd.Execute()
	out := buf.String()
	if !strings.Contains(out, "toolkit version: vtest") {
		t.Errorf("version --check output missing version: %q", out)
	}
}

// We cannot easily test Execute() since it calls os.Exit on error.
// Instead, test NewRootCmd and its RunE logic via the above tests.
