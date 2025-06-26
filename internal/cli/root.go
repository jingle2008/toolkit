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
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jingle2008/toolkit/internal/config"
	"github.com/jingle2008/toolkit/internal/domain"
	interrors "github.com/jingle2008/toolkit/internal/errors"
	production "github.com/jingle2008/toolkit/internal/infra/loader/production"
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

	const exampleConfig = `repo_path: "/path/to/your/repo"
kubeconfig: "/path/to/your/.kube/config"
env_type: "dev"
env_region: "us-phoenix-1"
env_realm: "oc1"
category: "tenant"
log_file: "toolkit.log"
debug: false
filter: ""
`

	rootCmd := &cobra.Command{
		Use:   "toolkit",
		Short: "Toolkit CLI",
		Long:  "Toolkit CLI for managing and visualizing infrastructure and configuration.",
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			showVersion, _ := cmd.Flags().GetBool("version")
			if showVersion {
				fmt.Println(version)
				// Early exit for version, but allow testability
				cmd.SilenceUsage = true
				return nil
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			_ = viper.BindPFlags(cmd.Flags())
			_ = viper.BindPFlags(cmd.PersistentFlags())

			viper.SetEnvPrefix("toolkit")
			viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
			viper.AutomaticEnv()

			if cfgFile != "" {
				viper.SetConfigFile(cfgFile)
				if err := viper.ReadInConfig(); err != nil && !errors.Is(err, os.ErrNotExist) {
					return interrors.Wrap("read config file", err)
				}
			}

			var cfg config.Config
			if err := viper.Unmarshal(&cfg); err != nil {
				return interrors.Wrap("unmarshal config", err)
			}
			if err := cfg.Validate(); err != nil {
				return interrors.Wrap("validate config", err)
			}

			logFormat := viper.GetString("log_format")
			logger, err := logging.NewFileLogger(cfg.Debug, cfg.LogFile, logFormat)
			if err != nil {
				return interrors.Wrap("initialize logger", err)
			}
			defer func() {
				_ = logger.Sync()
			}()
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			if err := runToolkit(ctx, logger, cfg, version); err != nil {
				logger.Errorw("fatal error", "error", err)
				return err
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
	// Enable shell completion for --category flag using domain.Aliases()
	_ = rootCmd.RegisterFlagCompletionFunc("category", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return domain.Aliases(), cobra.ShellCompDirectiveNoFileComp
	})
	rootCmd.PersistentFlags().StringP("filter", "f", "", "Initial filter for current category")
	rootCmd.PersistentFlags().String("kubeconfig", defaultKube, "Path to kubeconfig file")
	rootCmd.PersistentFlags().String("log_file", "toolkit.log", "Path to log file")
	rootCmd.PersistentFlags().Bool("debug", false, "Enable debug logging")
	rootCmd.PersistentFlags().String("log_format", "console", "Log format: console or json")
	rootCmd.PersistentFlags().Int("refresh_interval", 10, "Refresh interval in seconds")

	rootCmd.Flags().BoolP("version", "v", false, "Print version and exit")

	// Add "init" sub-command to scaffold an example config file
	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Scaffold an example config file",
		RunE: func(_ *cobra.Command, _ []string) error {
			// Only write if file does not exist
			if _, err := os.Stat(defaultConfig); err == nil {
				return fmt.Errorf("config file already exists at %s", defaultConfig)
			}
			if err := os.MkdirAll(filepath.Dir(defaultConfig), 0o750); err != nil {
				return interrors.Wrap("failed to create config directory", err)
			}
			if err := os.WriteFile(defaultConfig, []byte(exampleConfig), 0o600); err != nil {
				return interrors.Wrap("failed to write config file", err)
			}
			fmt.Printf("Example config written to %s\n", defaultConfig)
			return nil
		},
	}
	rootCmd.AddCommand(initCmd)

	return rootCmd
}

// Execute runs the root command.
func Execute(version string) {
	cmd := NewRootCmd(version)
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

// runToolkit is moved from main.go for clarity.
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
		tui.WithLoader(production.NewLoader()),
		tui.WithFilter(cfg.Filter),
		tui.WithVersion(version),
		tui.WithRefreshInterval(
			time.Duration(cfg.RefreshInterval)*time.Second),
	)
	if err != nil {
		logger.Errorw("failed to create toolkit model", "error", err)
		return interrors.Wrap("create toolkit model", err)
	}
	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithContext(ctx))
	_, err = p.Run()
	if err != nil && !errors.Is(err, context.Canceled) {
		logger.Errorw("program error", "error", err)
		return interrors.Wrap("program error", err)
	}
	return nil
}
