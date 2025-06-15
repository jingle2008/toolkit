package cli

import (
	"bytes"
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

// We cannot easily test Execute() since it calls os.Exit on error.
// Instead, test NewRootCmd and its RunE logic via the above tests.
