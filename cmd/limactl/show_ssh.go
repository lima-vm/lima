package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/lima-vm/lima/pkg/sshutil"
	"github.com/lima-vm/lima/pkg/store"
	"github.com/spf13/cobra"
)

const (
	showSSHFormatCmd     = "cmd"
	showSSHFormatArgs    = "args"
	showSSHFormatOptions = "options"
	showSSHFormatConfig  = "config"
	// TODO: consider supporting "url" format (ssh://USER@HOSTNAME:PORT)

	// TODO: consider supporting "json" format
	// It is unclear whether we can just map ssh "config" into JSON, as "config" has duplicated keys.
	// (JSON supports duplicated keys too, but not all JSON implementations expect JSON with duplicated keys)
)

var showSSHFormats = []string{showSSHFormatCmd, showSSHFormatArgs, showSSHFormatOptions, showSSHFormatConfig}

const showSSHExample = `
  "cmd" format (default): Full ssh command line.
    $ limactl show-ssh --format=cmd default
    ssh -o IdentityFile="/Users/example/.lima/_config/user" -o User=example -o Hostname=127.0.0.1 -o Port=60022 lima-default

  "args" format: Similar to the cmd format but omits "ssh" and the destination address
    $ limactl show-ssh --format=args default
    -o IdentityFile="/Users/example/.lima/_config/user" -o User=example -o Hostname=127.0.0.1 -o Port=60022

  "options" format: ssh option key value pairs
    $ limactl show-ssh --format=options default
    IdentityFile="/Users/example/.lima/_config/user"
    User=example
    Hostname=127.0.0.1
    Port=60022

  "config" format: ~/.ssh/config format
    $ limactl show-ssh --format=config default
    Host lima-default
      IdentityFile "/Users/example/.lima/_config/user "
      User example
      Hostname 127.0.0.1
      Port 60022
`

func newShowSSHCommand() *cobra.Command {
	var shellCmd = &cobra.Command{
		Use:               "show-ssh [flags] INSTANCE",
		Short:             "Show the ssh command line",
		Example:           showSSHExample,
		Args:              cobra.ExactArgs(1),
		RunE:              showSSHAction,
		ValidArgsFunction: showSSHBashComplete,
		SilenceErrors:     true,
	}

	shellCmd.Flags().StringP("format", "f", showSSHFormatCmd, "Format: "+strings.Join(showSSHFormats, ", "))
	_ = shellCmd.RegisterFlagCompletionFunc("format", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return showSSHFormats, cobra.ShellCompDirectiveNoFileComp
	})
	return shellCmd
}

func showSSHAction(cmd *cobra.Command, args []string) error {
	format, err := cmd.Flags().GetString("format")
	if err != nil {
		return err
	}
	instName := args[0]
	w := cmd.OutOrStdout()
	inst, err := store.Inspect(instName)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("instance %q does not exist, run `limactl start %s` to create a new instance", instName, instName)
		}
		return err
	}
	y, err := inst.LoadYAML()
	if err != nil {
		return err
	}
	opts, err := sshutil.SSHOpts(inst.Dir, *y.SSH.LoadDotSSHPubKeys, *y.SSH.ForwardAgent, *y.SSH.ForwardX11, *y.SSH.ForwardX11Trusted)
	if err != nil {
		return err
	}
	opts = append(opts, "Hostname=127.0.0.1")
	opts = append(opts, fmt.Sprintf("Port=%d", inst.SSHLocalPort))
	return formatSSH(w, instName, format, opts)
}

func quoteOption(o string) string {
	// make sure the shell doesn't swallow quotes in option values
	if strings.ContainsRune(o, '"') {
		o = "'" + o + "'"
	}
	return o
}

func formatSSH(w io.Writer, instName, format string, opts []string) error {
	fakeHostname := "lima-" + instName // corresponds to the default guest hostname
	switch format {
	case showSSHFormatCmd:
		args := []string{"ssh"}
		for _, o := range opts {
			args = append(args, "-o", quoteOption(o))
		}
		args = append(args, fakeHostname)
		// the args are similar to `limactl shell` but not exactly same. (e.g., lacks -t)
		fmt.Fprintln(w, strings.Join(args, " ")) // no need to use shellescape.QuoteCommand
	case showSSHFormatArgs:
		var args []string
		for _, o := range opts {
			args = append(args, "-o", quoteOption(o))
		}
		fmt.Fprintln(w, strings.Join(args, " ")) // no need to use shellescape.QuoteCommand
	case showSSHFormatOptions:
		for _, o := range opts {
			fmt.Fprintln(w, o)
		}
	case showSSHFormatConfig:
		fmt.Fprintf(w, "Host %s\n", fakeHostname)
		for _, o := range opts {
			kv := strings.SplitN(o, "=", 2)
			if len(kv) != 2 {
				return fmt.Errorf("unexpected option %q", o)
			}
			fmt.Fprintf(w, "  %s %s\n", kv[0], kv[1])
		}
	default:
		return fmt.Errorf("unknown format: %q", format)
	}
	return nil
}

func showSSHBashComplete(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return bashCompleteInstanceNames(cmd)
}
