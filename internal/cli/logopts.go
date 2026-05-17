package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"

	"github.com/jingle2008/toolkit/internal/config"
	"github.com/jingle2008/toolkit/pkg/infra/logging"
)

func logOptionsFromViper() (string, string, error) {
	logFormat := viper.GetString("log_format")
	if err := validateLogFormat(logFormat); err != nil {
		return "", "", err
	}
	logLevel, err := normalizeLogLevel(viper.GetString("log_level"))
	if err != nil {
		return "", "", err
	}
	return logFormat, logLevel, nil
}

// initLogger reads log_format/log_level from viper and constructs a
// file-backed logger writing to cfg.LogFile. Stdout is reserved for
// command output (get) or MCP frames (mcp), so logs never bleed into
// the data stream.
func initLogger(cfg config.Config) (logging.Logger, error) {
	logFormat, logLevel, err := logOptionsFromViper()
	if err != nil {
		return nil, err
	}
	logger, err := logging.NewFileLoggerWithLevel(cfg.Debug, cfg.LogFile, logFormat, logLevel)
	if err != nil {
		return nil, fmt.Errorf("initialize logger: %w", err)
	}
	return logger, nil
}

func validateLogFormat(logFormat string) error {
	switch logFormat {
	case "console", "json", "slog":
		return nil
	default:
		return fmt.Errorf("invalid log_format %q (valid: console|json|slog)", logFormat)
	}
}

func normalizeLogLevel(level string) (string, error) {
	logLevel := strings.ToLower(level)
	switch logLevel {
	case "", "debug", "info", "warn", "warning", "error":
		if logLevel == "warning" {
			logLevel = "warn"
		}
		return logLevel, nil
	default:
		return "", fmt.Errorf("invalid log_level %q (valid: debug|info|warn|error or empty)", logLevel)
	}
}
