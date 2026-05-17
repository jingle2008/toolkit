package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/jingle2008/toolkit/internal/config"
	production "github.com/jingle2008/toolkit/internal/infra/loader/production"
	"github.com/jingle2008/toolkit/internal/mcp"
	"github.com/jingle2008/toolkit/pkg/infra/logging"
)

// addMCPCommand wires the `toolkit mcp` subcommand that boots a
// stdio MCP server exposing the same loader surface as `toolkit get`.
func addMCPCommand(rootCmd *cobra.Command, cfgFile *string, version string) {
	mcpCmd := &cobra.Command{
		Use:   "mcp",
		Short: "Start the toolkit MCP server (stdio)",
		Long: `Run an MCP (Model Context Protocol) server over stdio so an
AI agent — Claude Code, Claude Desktop, or any MCP-aware client — can
list categories directly instead of shelling out to ` + "`toolkit get`" + `.

Startup env (env_type / env_region / env_realm / repo_path / kubeconfig)
comes from the same global flags and config file the TUI and ` + "`get`" + ` use;
each tool call may override env_type/region/realm per-invocation so one
running server can query multiple environments.

stdout is reserved for MCP JSON-RPC frames. Logs are written to
cfg.LogFile (default toolkit.log).`,
		Args: cobra.NoArgs,
		RunE: runMCP(cfgFile, version),
	}
	rootCmd.AddCommand(mcpCmd)
}

func runMCP(cfgFile *string, version string) func(cmd *cobra.Command, args []string) error {
	return func(_ *cobra.Command, _ []string) error {
		if err := readConfigFile(cfgFile); err != nil {
			return err
		}

		var cfg config.Config
		if err := viper.Unmarshal(&cfg); err != nil {
			return fmt.Errorf("unmarshal config: %w", err)
		}
		// MCP needs at minimum RepoPath + the env triple to load data.
		// KubeConfig is only required for cluster-derived tools; per-tool
		// failures there surface to the MCP client as tool errors.
		if missing := validateLoaderConfig(cfg); len(missing) > 0 {
			return fmt.Errorf(
				"missing required setting(s) for `toolkit mcp`: %s\n"+
					"  set them via flags, environment (TOOLKIT_*), or `toolkit init`",
				strings.Join(missing, ", "),
			)
		}

		// Stdout is reserved for MCP frames — logs go to cfg.LogFile.
		logFormat, logLevel, err := logOptionsFromViper()
		if err != nil {
			return err
		}
		logger, err := logging.NewFileLoggerWithLevel(cfg.Debug, cfg.LogFile, logFormat, logLevel)
		if err != nil {
			return fmt.Errorf("initialize logger: %w", err)
		}
		defer func() { _ = logger.Sync() }()

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		ctx = logging.WithContext(ctx, logger)

		ld := production.NewLoader(ctx, cfg.MetadataFile)
		srv := mcp.NewServer(cfg, ld, logger, version)
		logger.Infow("mcp server starting",
			"repo", cfg.RepoPath,
			"env_type", cfg.EnvType,
			"env_region", cfg.EnvRegion,
			"env_realm", cfg.EnvRealm,
		)
		return srv.Run(ctx)
	}
}
