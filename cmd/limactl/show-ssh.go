/*
Copyright The Lima Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lima-vm/lima/pkg/sshutil"
	"github.com/lima-vm/lima/pkg/store"
	"github.com/lima-vm/lima/pkg/store/dirnames"
	"github.com/lima-vm/lima/pkg/store/filenames"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

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

  To show the config file path:
    $ limactl ls --format='{{.SSHConfigFile}}' default
    /Users/example/.lima/default/ssh.config
`

func newShowSSHCommand() *cobra.Command {
	limaHome := "~/" + dirnames.DotLima
	if s, err := dirnames.LimaDir(); err == nil {
		limaHome = s
	}
	shellCmd := &cobra.Command{
		Use:   "show-ssh [flags] INSTANCE",
		Short: "Show the ssh command line (DEPRECATED; use `ssh -F` instead)",
		Long: fmt.Sprintf(`Show the ssh command line (DEPRECATED)

WARNING: 'limactl show-ssh' is deprecated.
Instead, use 'ssh -F %s/default/ssh.config lima-default' .
`, limaHome),
		Example:           showSSHExample,
		Args:              WrapArgsError(cobra.ExactArgs(1)),
		RunE:              showSSHAction,
		ValidArgsFunction: showSSHBashComplete,
		SilenceErrors:     true,
		GroupID:           advancedCommand,
	}

	shellCmd.Flags().StringP("format", "f", sshutil.FormatCmd, "Format: "+strings.Join(sshutil.Formats, ", "))
	_ = shellCmd.RegisterFlagCompletionFunc("format", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return sshutil.Formats, cobra.ShellCompDirectiveNoFileComp
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
			return fmt.Errorf("instance %q does not exist, run `limactl create %s` to create a new instance", instName, instName)
		}
		return err
	}
	logrus.Warnf("`limactl show-ssh` is deprecated. Instead, use `ssh -F %s %s`.",
		filepath.Join(inst.Dir, filenames.SSHConfig), inst.Hostname)
	opts, err := sshutil.SSHOpts(
		"ssh",
		inst.Dir,
		*inst.Config.User.Name,
		*inst.Config.SSH.LoadDotSSHPubKeys,
		*inst.Config.SSH.ForwardAgent,
		*inst.Config.SSH.ForwardX11,
		*inst.Config.SSH.ForwardX11Trusted)
	if err != nil {
		return err
	}
	opts = append(opts, "Hostname=127.0.0.1")
	opts = append(opts, fmt.Sprintf("Port=%d", inst.SSHLocalPort))
	return sshutil.Format(w, "ssh", instName, format, opts)
}

func showSSHBashComplete(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return bashCompleteInstanceNames(cmd)
}
