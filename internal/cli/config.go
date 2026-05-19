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
	var (
		format string
		pretty bool
	)
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Print the effective merged config (defaults + env + file + flags)",
		Long: `Print toolkit's effective config — the same view every other
subcommand sees after merging defaults, environment (TOOLKIT_*),
the config file, and CLI flags.

Use this to inspect what's currently in effect without opening
the file manually. The output includes the resolved config-file
path and whether it exists on disk.

Note: output may include local filesystem paths (repo_path,
kubeconfig, log_file, metadata_file). Strip those before sharing
the output in bug reports.

Examples:
  toolkit config
  toolkit config -o json
  toolkit config -o json --pretty=false
  toolkit --config /tmp/alt.yaml config`,
		Args: cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			if err := readConfigFile(cfgFile); err != nil {
				return err
			}
			return writeConfigView(c.OutOrStdout(), *cfgFile, format, pretty)
		},
	}
	cmd.Flags().StringVarP(&format, "output", "o", "yaml", "yaml|json")
	cmd.Flags().BoolVar(&pretty, "pretty", true, "pretty-print JSON/YAML output")
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

func writeConfigView(w io.Writer, cfgFile, format string, pretty bool) error {
	exists := false
	if cfgFile != "" {
		if _, err := os.Stat(cfgFile); err == nil {
			exists = true
		}
	}
	settings := viper.AllSettings()
	// The persistent `--config` flag is bound by viper, so it shows up
	// here as a redundant copy of ConfigFile. Drop it to keep one
	// authoritative source for the resolved path.
	delete(settings, "config")
	view := configView{
		ConfigFile: cfgFile,
		Exists:     exists,
		Settings:   settings,
	}

	opts := output.Options{Pretty: pretty}
	switch strings.ToLower(format) {
	case "", "yaml":
		opts.Format = output.FormatYAML
		return output.WriteYAML(w, view, opts)
	case "json":
		opts.Format = output.FormatJSON
		return output.WriteJSON(w, view, opts)
	default:
		return fmt.Errorf("invalid output format %q (valid: yaml|json)", format)
	}
}
