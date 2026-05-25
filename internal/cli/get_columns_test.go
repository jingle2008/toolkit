package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

// Pattern mirrors TestGetCmd_UnknownCategory etc in get_test.go.
// Tests in this file mutate process-global state (viper.Reset, t.Setenv("HOME"))
// via runGetForColumnsTest and are intentionally serial.

//nolint:paralleltest // mutates viper global + t.Setenv
func TestGet_ColumnsFlag_Defaults_Alias(t *testing.T) {
	out, err := runGetForColumnsTest(t, "alias", "-o", "csv")
	if err != nil {
		t.Fatalf("unexpected error: %v\n%s", err, out)
	}
	header := strings.SplitN(out, "\n", 2)[0]
	parts := strings.Split(header, ",")
	if len(parts) != 2 || parts[0] != "NAME" || parts[1] != "ALIASES" {
		t.Errorf("default header = %q, want NAME,ALIASES", header)
	}
}

//nolint:paralleltest // mutates viper global + t.Setenv
func TestGet_ColumnsFlag_Explicit_Alias(t *testing.T) {
	out, err := runGetForColumnsTest(t, "alias", "-o", "csv", "--columns", "aliases,name")
	if err != nil {
		t.Fatalf("err: %v\n%s", err, out)
	}
	header := strings.SplitN(out, "\n", 2)[0]
	parts := strings.Split(header, ",")
	if len(parts) != 2 || parts[0] != "ALIASES" || parts[1] != "NAME" {
		t.Errorf("header = %q, want ALIASES,NAME", header)
	}
}

//nolint:paralleltest // mutates viper global + t.Setenv
func TestGet_ColumnsFlag_Unknown_Alias(t *testing.T) {
	_, err := runGetForColumnsTest(t, "alias", "-o", "csv", "--columns", "name,bogus")
	if err == nil {
		t.Fatal("expected error for unknown column, got nil")
	}
	if !strings.Contains(err.Error(), "unknown column key(s): bogus") {
		t.Errorf("error %q does not mention unknown key", err.Error())
	}
	if !strings.Contains(err.Error(), "valid keys:") {
		t.Errorf("error %q does not list valid keys", err.Error())
	}
}

//nolint:paralleltest // mutates viper global + t.Setenv
func TestGet_ColumnsFlag_EmptyToken_Alias(t *testing.T) {
	_, err := runGetForColumnsTest(t, "alias", "-o", "csv", "--columns", "name,,aliases")
	if err == nil {
		t.Fatal("expected error for empty token, got nil")
	}
	if !strings.Contains(err.Error(), "empty token") {
		t.Errorf("error %q does not mention empty token", err.Error())
	}
}

//nolint:paralleltest // mutates viper global + t.Setenv
func TestGet_ColumnsFlag_Help_Alias(t *testing.T) {
	out, err := runGetForColumnsTest(t, "alias", "--columns", "help")
	if err != nil {
		t.Fatalf("err: %v\n%s", err, out)
	}
	if !strings.Contains(out, "KEY") || !strings.Contains(out, "TITLE") {
		t.Errorf("help output missing expected headers: %s", out)
	}
	if !strings.Contains(out, "name") || !strings.Contains(out, "aliases") {
		t.Errorf("help output missing expected keys: %s", out)
	}
}

//nolint:paralleltest // mutates viper global + t.Setenv
func TestGet_ColumnsFlag_MutexWithJSON(t *testing.T) {
	_, err := runGetForColumnsTest(t, "alias", "-o", "json", "--columns", "name")
	if err == nil {
		t.Fatal("expected mutual-exclusion error")
	}
	if !strings.Contains(err.Error(), "--columns has no effect with -o json") {
		t.Errorf("error message: %q", err.Error())
	}
}

func runGetForColumnsTest(t *testing.T, args ...string) (string, error) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	viper.Reset()
	t.Cleanup(viper.Reset)
	cmd := NewRootCmd("vtest")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(append([]string{"get"}, args...))
	err := cmd.Execute()
	return out.String(), err
}
