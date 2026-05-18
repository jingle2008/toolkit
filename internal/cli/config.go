package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/jingle2008/toolkit/internal/cli/output"
)

// addConfigCommand wires `toolkit config`, a read-only inspection of
// the effective merged settings (defaults + env + config file + flags).
// Counterpart to `toolkit init`, which scaffolds the file.
func addConfigCommand(rootCmd *cobra.Command, cfgFile *string) {
	var format string
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Print the effective merged config (defaults + env + file + flags)",
		Long: `Print toolkit's effective config — the same view every other
subcommand sees after merging defaults, environment (TOOLKIT_*),
the config file, and CLI flags.

Use this to inspect what's currently in effect without opening
the file manually. The output includes the resolved config-file
path and whether it exists on disk.

Examples:
  toolkit config
  toolkit config -o json
  toolkit --config /tmp/alt.yaml config`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := readConfigFile(cfgFile); err != nil {
				return err
			}
			return writeConfigView(cmd.OutOrStdout(), *cfgFile, format)
		},
	}
	cmd.Flags().StringVarP(&format, "output", "o", "yaml", "yaml|json")
	_ = cmd.RegisterFlagCompletionFunc("output", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return []string{"yaml", "json"}, cobra.ShellCompDirectiveNoFileComp
	})
	rootCmd.AddCommand(cmd)
}

// configView is the JSON/YAML shape printed by `toolkit config`.
// Keeping it a typed struct (rather than a free-form map) means
// consumers can rely on a stable schema.
type configView struct {
	ConfigFile string         `json:"config_file" yaml:"config_file"`
	Exists     bool           `json:"exists" yaml:"exists"`
	Settings   map[string]any `json:"settings" yaml:"settings"`
}

func writeConfigView(w io.Writer, cfgFile, format string) error {
	var fmtChoice output.Format
	switch strings.ToLower(format) {
	case "", "yaml":
		fmtChoice = output.FormatYAML
	case "json":
		fmtChoice = output.FormatJSON
	default:
		return fmt.Errorf("invalid output format %q (valid: yaml|json)", format)
	}

	exists := false
	if cfgFile != "" {
		if _, err := os.Stat(cfgFile); err == nil {
			exists = true
		}
	}
	view := configView{
		ConfigFile: cfgFile,
		Exists:     exists,
		Settings:   viper.AllSettings(),
	}

	opts := output.Options{Format: fmtChoice, Pretty: true}
	switch fmtChoice { //nolint:exhaustive
	case output.FormatYAML:
		return output.WriteYAML(w, view, opts)
	case output.FormatJSON:
		return output.WriteJSON(w, view, opts)
	}
	return nil
}
