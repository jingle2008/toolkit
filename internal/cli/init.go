package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

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
