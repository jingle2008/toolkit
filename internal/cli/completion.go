package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func addCompletionCommand(rootCmd *cobra.Command) {
	completionCmd := &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
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

PowerShell:
  PS> toolkit completion powershell | Out-String | Invoke-Expression
  # To load completions for every new session, run:
  PS> toolkit completion powershell > $PROFILE
`,
		Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return rootCmd.GenBashCompletion(cmd.OutOrStdout())
			case "zsh":
				return rootCmd.GenZshCompletion(cmd.OutOrStdout())
			case "fish":
				return rootCmd.GenFishCompletion(cmd.OutOrStdout(), true)
			case "powershell":
				return rootCmd.GenPowerShellCompletion(cmd.OutOrStdout())
			default:
				return fmt.Errorf("unsupported shell: %s", args[0])
			}
		},
	}
	rootCmd.AddCommand(completionCmd)
}
