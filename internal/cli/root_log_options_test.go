package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

//nolint:paralleltest // uses global viper state.
func TestLogOptionsFromViper_Valid(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	viper.Set("log_format", "console")
	viper.Set("log_level", "warning")

	format, level, err := logOptionsFromViper()
	if err != nil {
		t.Fatalf("logOptionsFromViper error: %v", err)
	}
	if format != "console" {
		t.Fatalf("format = %q, want %q", format, "console")
	}
	if level != "warn" {
		t.Fatalf("level = %q, want %q", level, "warn")
	}
}

//nolint:paralleltest // uses global viper state.
func TestLogOptionsFromViper_InvalidFormat(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	viper.Set("log_format", "bad")
	viper.Set("log_level", "info")

	if _, _, err := logOptionsFromViper(); err == nil {
		t.Fatal("expected error for invalid log format")
	}
}

func TestNormalizeLogLevel_Invalid(t *testing.T) {
	t.Parallel()
	if _, err := normalizeLogLevel("nope"); err == nil {
		t.Fatal("expected error for invalid log level")
	}
}

//nolint:paralleltest // uses global viper state.
func TestReadConfigFile(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte("log_format: console\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if err := readConfigFile(&cfgPath); err != nil {
		t.Fatalf("readConfigFile error: %v", err)
	}
	if got := viper.GetString("log_format"); got != "console" {
		t.Fatalf("log_format = %q, want %q", got, "console")
	}
}
