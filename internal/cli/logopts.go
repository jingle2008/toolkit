package cli

import (
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/spf13/viper"
	"k8s.io/klog/v2"

	"github.com/jingle2008/toolkit/internal/config"
	"github.com/jingle2008/toolkit/internal/logging"
)

func logOptionsFromViper() (string, string, error) {
	logFormat := viper.GetString("log-format")
	if err := validateLogFormat(logFormat); err != nil {
		return "", "", err
	}
	logLevel, err := normalizeLogLevel(viper.GetString("log-level"))
	if err != nil {
		return "", "", err
	}
	return logFormat, logLevel, nil
}

// initLogger reads log-format/log-level from viper and constructs a
// file-backed logger: a per-session file in cfg.LogDir named for cmd
// (logging.NewSession), or cfg.LogFile when that is set. It returns the
// file's path too. Stdout is reserved for command output (get) or MCP
// frames (mcp), so logs never bleed into the data stream. It also
// routes client-go's klog output into the logger. Close the returned
// closer on shutdown.
func initLogger(cfg config.Config, cmd string) (*slog.Logger, io.Closer, string, error) {
	logFormat, logLevel, err := logOptionsFromViper()
	if err != nil {
		return nil, nil, "", err
	}
	var (
		logger *slog.Logger
		closer io.Closer
		path   = cfg.LogFile
	)
	if path != "" {
		logger, closer = logging.NewFile(path, logFormat, logLevel, cfg.Debug)
	} else {
		logger, closer, path, err = logging.NewSession(logDir(cfg), cmd, logFormat, logLevel, cfg.Debug)
		if err != nil {
			return nil, nil, "", err
		}
	}
	routeKlog(logger)
	return logger, closer, path, nil
}

// routeKlog sends klog (client-go's logger) through logger, so its lines
// land in the log file (and the TUI's log overlay) instead of on stderr.
func routeKlog(logger *slog.Logger) {
	klog.SetSlogLogger(logger)
}

func validateLogFormat(logFormat string) error {
	switch logFormat {
	case "console", "json", "slog":
		return nil
	default:
		return fmt.Errorf("invalid log-format %q (valid: console|json|slog)", logFormat)
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
		return "", fmt.Errorf("invalid log-level %q (valid: debug|info|warn|error or empty)", logLevel)
	}
}
