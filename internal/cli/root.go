/*
Package cli provides the root command and CLI entrypoint for the toolkit application.
*/
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jingle2008/toolkit/internal/config"
	"github.com/jingle2008/toolkit/internal/domain"
	production "github.com/jingle2008/toolkit/internal/infra/loader/production"
	"github.com/jingle2008/toolkit/internal/ui/tui"
	"github.com/jingle2008/toolkit/pkg/infra/logging"
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
metadata_file: "" # Optional path to a YAML or JSON file with additional metadata (e.g. tenants)
`

	home := homedir.HomeDir()
	defaultKube := filepath.Join(home, ".kube", "config")
	defaultConfig := filepath.Join(home, ".config", "toolkit", "config.yaml")
	defaultMetadata := filepath.Join(home, ".config", "toolkit", "metadata.yaml")

	rootCmd := &cobra.Command{
		Use:   "toolkit",
		Short: "Toolkit CLI",
		Long:  "Toolkit CLI for managing and visualizing infrastructure and configuration.",
		RunE:  runRootE(&cfgFile, version),
	}

	addPersistentFlags(rootCmd, &cfgFile, defaultKube, defaultConfig, defaultMetadata)
	addInitCommand(rootCmd, defaultConfig, exampleConfig)
	addCompletionCommand(rootCmd)
	addVersionCheckCommand(rootCmd, version)

	return rootCmd
}

// runRootE returns the RunE function for the root command.
func runRootE(cfgFile *string, version string) func(cmd *cobra.Command, _ []string) error {
	return func(cmd *cobra.Command, _ []string) error {
		_ = viper.BindPFlags(cmd.Flags())
		_ = viper.BindPFlags(cmd.PersistentFlags())

		viper.SetEnvPrefix("toolkit")
		viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
		viper.AutomaticEnv()

		if *cfgFile != "" {
			viper.SetConfigFile(*cfgFile)
			if err := viper.ReadInConfig(); err != nil && !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("read config file: %w", err)
			}
		}

		var cfg config.Config
		if err := viper.Unmarshal(&cfg); err != nil {
			return fmt.Errorf("unmarshal config: %w", err)
		}
		if err := cfg.Validate(); err != nil {
			return fmt.Errorf("validate config: %w", err)
		}

		logFormat := viper.GetString("log_format")
		logger, err := logging.NewFileLogger(cfg.Debug, cfg.LogFile, logFormat)
		if err != nil {
			return fmt.Errorf("initialize logger: %w", err)
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
	}
}

// addPersistentFlags adds persistent flags to the root command.
func addPersistentFlags(rootCmd *cobra.Command, cfgFile *string, defaultKube, defaultConfig, defaultMetadata string) {
	rootCmd.PersistentFlags().StringVar(cfgFile, "config", defaultConfig, "Path to config file (YAML or JSON)")
	rootCmd.PersistentFlags().String("repo_path", "", "Path to the repository")
	rootCmd.PersistentFlags().String("env_type", "", "Environment type (e.g. dev, prod)")
	rootCmd.PersistentFlags().String("env_region", "", "Environment region")
	rootCmd.PersistentFlags().String("env_realm", "", "Environment realm")
	rootCmd.PersistentFlags().StringP("category", "c", "", "Category to display")
	_ = rootCmd.RegisterFlagCompletionFunc("category", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return domain.Aliases, cobra.ShellCompDirectiveNoFileComp
	})
	rootCmd.PersistentFlags().StringP("filter", "f", "", "Initial filter for current category")
	rootCmd.PersistentFlags().String("metadata_file", defaultMetadata, "Optional path to a YAML or JSON file with additional metadata (e.g. tenants)")
	rootCmd.PersistentFlags().String("kubeconfig", defaultKube, "Path to kubeconfig file")
	rootCmd.PersistentFlags().String("log_file", "toolkit.log", "Path to log file")
	rootCmd.PersistentFlags().Bool("debug", false, "Enable debug logging")
	rootCmd.PersistentFlags().String("log_format", "console", "Log format: console or json")
}

func addCompletionCommand(rootCmd *cobra.Command) {
	completionCmd := &cobra.Command{
		Use:   "completion [bash|zsh|fish]",
		Short: "Generate shell completion scripts",
		Long: `To load completions:

Bash:
  $ source <(toolkit completion bash)
  # To load completions for each session, execute once:
  # Linux:
  $ toolkit completion bash > /etc/bash_completion.d/toolkit
  # macOS:
  $ toolkit completion bash > /usr/local/etc/bash_completion.d/toolkit

Zsh:
  $ echo "autoload -U compinit; compinit" >> ~/.zshrc
  $ toolkit completion zsh > "${fpath[1]}/_toolkit"

Fish:
  $ toolkit completion fish | source
  $ toolkit completion fish > ~/.config/fish/completions/toolkit.fish
`,
		Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		ValidArgs: []string{"bash", "zsh", "fish"},
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return rootCmd.GenBashCompletion(cmd.OutOrStdout())
			case "zsh":
				return rootCmd.GenZshCompletion(cmd.OutOrStdout())
			case "fish":
				return rootCmd.GenFishCompletion(cmd.OutOrStdout(), true)
			default:
				return fmt.Errorf("unsupported shell: %s", args[0])
			}
		},
	}
	rootCmd.AddCommand(completionCmd)
}

func addVersionCheckCommand(rootCmd *cobra.Command, currentVersion string) {
	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print the version number and check for updates",
		RunE: func(cmd *cobra.Command, _ []string) error {
			fmt.Printf("toolkit version: %s\n", currentVersion)
			check, _ := cmd.Flags().GetBool("check")
			if check {
				latest, err := fetchLatestRelease()
				if err != nil {
					return fmt.Errorf("failed to check latest version: %w", err)
				}
				if latest == currentVersion {
					fmt.Println("You are running the latest version.")
				} else {
					fmt.Printf("A newer version is available: %s\n", latest)
				}
			}
			return nil
		},
	}
	versionCmd.Flags().Bool("check", false, "Check for the latest release on GitHub")
	rootCmd.AddCommand(versionCmd)
}

func fetchLatestRelease() (string, error) {
	const url = "https://api.github.com/repos/jingle2008/toolkit/releases/latest"
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "toolkit")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			return
		}
	}()
	if resp.StatusCode != 200 {
		return "", errors.New("unexpected status: " + resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var result struct {
		Tag string `json:"tag_name"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}
	if result.Tag == "" {
		return "", errors.New("no tag_name in GitHub response")
	}
	return result.Tag, nil
}

// addInitCommand adds the "init" subcommand to the root command.
func addInitCommand(rootCmd *cobra.Command, defaultConfig, exampleConfig string) {
	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Scaffold an example config file",
		RunE: func(_ *cobra.Command, _ []string) error {
			if _, err := os.Stat(defaultConfig); err == nil {
				return fmt.Errorf("config file already exists at %s", defaultConfig)
			}
			if err := os.MkdirAll(filepath.Dir(defaultConfig), 0o750); err != nil {
				return fmt.Errorf("failed to create config directory: %w", err)
			}
			if err := os.WriteFile(defaultConfig, []byte(exampleConfig), 0o600); err != nil {
				return fmt.Errorf("failed to write config file: %w", err)
			}
			fmt.Printf("Example config written to %s\n", defaultConfig)
			return nil
		},
	}
	rootCmd.AddCommand(initCmd)
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
