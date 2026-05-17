/*
Package cli provides the root command and CLI entrypoint for the toolkit application.
*/
package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/jingle2008/toolkit/internal/config"
	"github.com/jingle2008/toolkit/internal/domain"
	production "github.com/jingle2008/toolkit/internal/infra/loader/production"
	"github.com/jingle2008/toolkit/internal/ui/tui"
	"github.com/jingle2008/toolkit/pkg/infra/logging"
	"github.com/jingle2008/toolkit/pkg/models"
)

// NewRootCmd returns the root cobra command for the toolkit CLI.
func NewRootCmd(version string) *cobra.Command {
	var cfgFile string

	const exampleConfig = `repo_path: "/path/to/your/repo"
kubeconfig: "/path/to/your/.kube/config"
env_type: "dev"
env_region: "us-phoenix-1"
env_realm: "oc1"
category: "tenant"
log_file: "toolkit.log"
log_format: "console" # console|json|slog
log_level: "" # debug|info|warn|error (empty uses debug flag)
debug: false
filter: ""
metadata_file: "" # Optional path to a YAML or JSON file with additional metadata (e.g. tenants)
`

	home, _ := os.UserHomeDir()
	cfgDir := filepath.Join(home, ".config")
	defaultKube := filepath.Join(home, ".kube", "config")
	defaultConfig := filepath.Join(cfgDir, "toolkit", "config.yaml")
	defaultMetadata := filepath.Join(cfgDir, "toolkit", "metadata.yaml")

	rootCmd := &cobra.Command{
		Use:           "toolkit",
		Short:         "Toolkit CLI",
		Long:          "Toolkit CLI for managing and visualizing infrastructure and configuration.",
		SilenceUsage:  true,
		SilenceErrors: false,
		RunE:          runRootE(&cfgFile, version),
	}

	addPersistentFlags(rootCmd, &cfgFile, defaultKube, defaultConfig, defaultMetadata)
	addInitCommand(rootCmd, defaultConfig, exampleConfig)
	addCompletionCommand(rootCmd)
	addVersionCheckCommand(rootCmd, version)
	addGetCommand(rootCmd)

	// Bind persistent flags once so Viper can read them.
	_ = viper.BindPFlags(rootCmd.PersistentFlags())

	// Initialize Viper: env settings and optional config file read before command execution.
	cobra.OnInitialize(func() {
		viper.SetEnvPrefix("toolkit")
		viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
		viper.AutomaticEnv()
	})

	return rootCmd
}

// runRootE returns the RunE function for the root command.
func runRootE(cfgFile *string, version string) func(cmd *cobra.Command, _ []string) error {
	return func(_ *cobra.Command, _ []string) error {
		// Parse config file with proper error handling (kept out of OnInitialize to preserve error semantics and tests).
		if err := readConfigFile(cfgFile); err != nil {
			return err
		}

		var cfg config.Config
		if err := viper.Unmarshal(&cfg); err != nil {
			return fmt.Errorf("unmarshal config: %w", err)
		}

		logFormat, logLevel, err := logOptionsFromViper()
		if err != nil {
			return err
		}

		logger, err := logging.NewFileLoggerWithLevel(cfg.Debug, cfg.LogFile, logFormat, logLevel)
		if err != nil {
			return fmt.Errorf("initialize logger: %w", err)
		}
		defer func() {
			_ = logger.Sync()
		}()

		// Validate config after log options so flag errors surface first.
		if err := cfg.Validate(); err != nil {
			return fmt.Errorf("validate config: %w (hint: run `toolkit init` to scaffold an example config)", err)
		}

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		if err := runToolkit(ctx, logger, cfg, version); err != nil {
			logger.Errorw("fatal error", "error", err)
			return err
		}
		return nil
	}
}

func readConfigFile(cfgFile *string) error {
	if cfgFile == nil || *cfgFile == "" {
		return nil
	}
	viper.SetConfigFile(*cfgFile)
	if err := viper.ReadInConfig(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read config file: %w", err)
	}
	return nil
}

// Execute runs the root command.
func Execute(version string) {
	cmd := NewRootCmd(version)
	if err := cmd.Execute(); err != nil {
		// Let Cobra print the error once; just exit with non-zero status.
		os.Exit(1)
	}
}

// runToolkit wires the loaded config into the TUI and runs the Bubble Tea program.
func runToolkit(ctx context.Context, logger logging.Logger, cfg config.Config, version string) error {
	category, _ := domain.ParseCategory(cfg.Category)
	env := models.Environment{
		Type:   cfg.EnvType,
		Region: cfg.EnvRegion,
		Realm:  cfg.EnvRealm,
	}
	repoPath := cfg.RepoPath
	kubeConfig := cfg.KubeConfig

	ctx = logging.WithContext(ctx, logger)
	logger.Infow("starting toolkit",
		"repo", repoPath,
		"env", env,
		"category", category,
	)

	model, err := tui.NewModel(
		tui.WithRepoPath(repoPath),
		tui.WithKubeConfig(kubeConfig),
		tui.WithEnvironment(env),
		tui.WithCategory(category),
		tui.WithLogger(logger),
		tui.WithContext(ctx),
		tui.WithLoader(production.NewLoader(ctx, cfg.MetadataFile)),
		tui.WithFilter(cfg.Filter),
		tui.WithVersion(version),
	)
	if err != nil {
		logger.Errorw("failed to create toolkit model", "error", err)
		return fmt.Errorf("create toolkit model: %w", err)
	}
	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithContext(ctx))
	_, err = p.Run()
	if err != nil && !errors.Is(err, context.Canceled) {
		logger.Errorw("program error", "error", err)
		return fmt.Errorf("program error: %w", err)
	}
	return nil
}
