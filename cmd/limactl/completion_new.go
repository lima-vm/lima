package main

import (
	"github.com/spf13/cobra"
	"os"
)

func newCompletionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate completion script",
		Long: `To load completions:
Bash:
  $ source <(limactl completion bash)
  # To load completions for each session, execute once:
  # Linux:
  $ limactl completion bash > /etc/bash_completion.d/limactl
  # macOS:
  $ limactl completion bash > /usr/local/etc/bash_completion.d/limactl
Zsh:
  # If shell completion is not already enabled in your environment,
  # you will need to enable it.  You can execute the following once:
  $ echo "autoload -U compinit; compinit" >> ~/.zshrc
  # To load completions for each session, execute once:
  $ limactl completion zsh > "${fpath[1]}/_limactl"
  # You will need to start a new shell for this setup to take effect.
fish:
  $ limactl completion fish | source
  # To load completions for each session, execute once:
  $ limactl completion fish > ~/.config/fish/completions/limactl.fish
PowerShell:
  PS> limactl completion powershell | Out-String | Invoke-Expression
  # To load completions for every new session, run:
  PS> limactl completion powershell > limactl.ps1
  # and source this file from your PowerShell profile.
`,
		DisableFlagsInUseLine: true,
		ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
		Args:                  cobra.ExactValidArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			switch args[0] {
			case "bash":
				cmd.Root().GenBashCompletion(os.Stdout)
			case "zsh":
				cmd.Root().GenZshCompletion(os.Stdout)
			case "fish":
				cmd.Root().GenFishCompletion(os.Stdout, true)
			case "powershell":
				cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
			}
		},
	}
	return cmd
}
