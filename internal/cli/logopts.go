package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
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
