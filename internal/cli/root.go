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
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jingle2008/toolkit/internal/config"
	"github.com/jingle2008/toolkit/internal/domain"
	"github.com/jingle2008/toolkit/internal/infra/loader"
	"github.com/jingle2008/toolkit/internal/infra/logging"
	"github.com/jingle2008/toolkit/internal/ui/tui"
	"github.com/jingle2008/toolkit/pkg/models"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/client-go/util/homedir"
)

// NewRootCmd returns the root cobra command for the toolkit CLI.
func NewRootCmd(version string) *cobra.Command {
	var cfgFile string

	rootCmd := &cobra.Command{
		Use:   "toolkit",
		Short: "Toolkit CLI",
		Long:  "Toolkit CLI for managing and visualizing infrastructure and configuration.",
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			showVersion, _ := cmd.Flags().GetBool("version")
			if showVersion {
				fmt.Println(version)
				os.Exit(0)
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			_ = viper.BindPFlags(cmd.Flags())
			_ = viper.BindPFlags(cmd.PersistentFlags())

			viper.SetEnvPrefix("toolkit")
			viper.AutomaticEnv()

			if cfgFile != "" {
				viper.SetConfigFile(cfgFile)
				if err := viper.ReadInConfig(); err != nil && !errors.Is(err, os.ErrNotExist) {
					return fmt.Errorf("failed to read config file: %w", err)
				}
			}

			var cfg config.Config
			if err := viper.Unmarshal(&cfg); err != nil {
				return fmt.Errorf("failed to unmarshal config: %w", err)
			}
			if err := cfg.Validate(); err != nil {
				return fmt.Errorf("failed to validate config: %w", err)
			}

			logger, err := logging.NewFileLogger(cfg.Debug, cfg.LogFile)
			if err != nil {
				return fmt.Errorf("failed to initialize logger: %w", err)
			}
			defer func() {
				_ = logger.Sync()
			}()
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			if err := runToolkit(ctx, logger, cfg); err != nil {
				logger.Errorw("fatal error", "error", err)
				os.Exit(1)
			}
			return nil
		},
	}

	home := homedir.HomeDir()
	defaultKube := filepath.Join(home, ".kube", "config")
	defaultConfig := filepath.Join(home, ".config", "toolkit", "config.yaml")
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", defaultConfig, "Path to config file (YAML or JSON)")
	rootCmd.PersistentFlags().String("repo_path", "", "Path to the repository")
	rootCmd.PersistentFlags().String("env_type", "", "Environment type (e.g. dev, prod)")
	rootCmd.PersistentFlags().String("env_region", "", "Environment region")
	rootCmd.PersistentFlags().String("env_realm", "", "Environment realm")
	rootCmd.PersistentFlags().StringP("category", "c", "", "Category to display")
	rootCmd.PersistentFlags().StringP("filter", "f", "", "Initial filter for current category")
	rootCmd.PersistentFlags().String("kubeconfig", defaultKube, "Path to kubeconfig file")
	rootCmd.PersistentFlags().String("log_file", "toolkit.log", "Path to log file")
	rootCmd.PersistentFlags().Bool("debug", false, "Enable debug logging")

	rootCmd.Flags().BoolP("version", "v", false, "Print version and exit")

	return rootCmd
}

// Execute runs the root command.
func Execute(version string) {
	cmd := NewRootCmd(version)
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// runToolkit is moved from main.go for clarity.
func runToolkit(ctx context.Context, logger logging.Logger, cfg config.Config) error {
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
		tui.WithLoader(loader.ProductionLoader{}),
		tui.WithFilter(cfg.Filter),
	)
	if err != nil {
		logger.Errorw("failed to create toolkit model", "error", err)
		return fmt.Errorf("failed to create toolkit model: %w", err)
	}
	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithContext(ctx))
	// Run the program with context cancellation
	done := make(chan error, 1)
	go func() {
		_, err := p.Run()
		done <- err
	}()
	select {
	case <-ctx.Done():
		logger.Errorw("context cancelled", "error", ctx.Err())
		return ctx.Err()
	case err := <-done:
		if err != nil {
			logger.Errorw("program error", "error", err)
			return fmt.Errorf("alas, there's been an error: %w", err)
		}
	}
	return nil
}
