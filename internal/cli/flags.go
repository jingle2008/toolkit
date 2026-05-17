package cli

import (
	"github.com/spf13/cobra"

	"github.com/jingle2008/toolkit/internal/domain"
)

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
	rootCmd.PersistentFlags().String("log_format", "console", "Log format: console|json|slog")
	rootCmd.PersistentFlags().String("log_level", "", "Minimum log level: debug|info|warn|error (empty uses debug flag)")

	// Hint shells that these flags take filenames (improves completion UX).
	_ = rootCmd.MarkFlagFilename("config")
	_ = rootCmd.MarkFlagFilename("metadata_file")
	_ = rootCmd.MarkFlagFilename("kubeconfig")
	_ = rootCmd.MarkFlagFilename("log_file")
	// Shell completion for enumerated flags.
	_ = rootCmd.RegisterFlagCompletionFunc("log_format", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return []string{"console", "json", "slog"}, cobra.ShellCompDirectiveNoFileComp
	})
	_ = rootCmd.RegisterFlagCompletionFunc("log_level", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return []string{"debug", "info", "warn", "error"}, cobra.ShellCompDirectiveNoFileComp
	})
}
