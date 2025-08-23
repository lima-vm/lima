// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/lima-vm/lima/v2/pkg/freeport"
	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/sshutil"
	"github.com/lima-vm/lima/v2/pkg/store"
)

const tunnelHelp = `Create a tunnel for Lima

Create a SOCKS tunnel so that the host can join the guest network.
`

func newTunnelCommand() *cobra.Command {
	tunnelCmd := &cobra.Command{
		Use:   "tunnel [flags] INSTANCE",
		Short: "Create a tunnel for Lima",
		PersistentPreRun: func(*cobra.Command, []string) {
			logrus.Warn("`limactl tunnel` is experimental")
		},
		Long:              tunnelHelp,
		Args:              WrapArgsError(cobra.ExactArgs(1)),
		RunE:              tunnelAction,
		ValidArgsFunction: tunnelBashComplete,
		SilenceErrors:     true,
		GroupID:           advancedCommand,
	}

	tunnelCmd.Flags().SetInterspersed(false)
	// TODO: implement l2tp, ikev2, masque, ...
	tunnelCmd.Flags().String("type", "socks", "Tunnel type, currently only \"socks\" is implemented")
	tunnelCmd.Flags().Int("socks-port", 0, "SOCKS port, defaults to a random port")
	return tunnelCmd
}

func tunnelAction(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	flags := cmd.Flags()
	tunnelType, err := flags.GetString("type")
	if err != nil {
		return err
	}
	if tunnelType != "socks" {
		return fmt.Errorf("unknown tunnel type: %q", tunnelType)
	}
	port, err := flags.GetInt("socks-port")
	if err != nil {
		return err
	}
	if port != 0 && (port < 1024 || port > 65535) {
		return fmt.Errorf("invalid socks port %d", port)
	}
	stdout, stderr := cmd.OutOrStdout(), cmd.ErrOrStderr()
	instName := args[0]
	inst, err := store.Inspect(ctx, instName)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("instance %q does not exist, run `limactl create %s` to create a new instance", instName, instName)
		}
		return err
	}
	if inst.Status == limatype.StatusStopped {
		return fmt.Errorf("instance %q is stopped, run `limactl start %s` to start the instance", instName, instName)
	}

	if port == 0 {
		port, err = freeport.TCP()
		if err != nil {
			return err
		}
	}

	sshExe, err := sshutil.NewSSHExe()
	if err != nil {
		return err
	}

	sshOpts, err := sshutil.SSHOpts(
		ctx,
		sshExe,
		inst.Dir,
		*inst.Config.User.Name,
		*inst.Config.SSH.LoadDotSSHPubKeys,
		*inst.Config.SSH.ForwardAgent,
		*inst.Config.SSH.ForwardX11,
		*inst.Config.SSH.ForwardX11Trusted)
	if err != nil {
		return err
	}
	sshArgs := append([]string{}, sshExe.Args...)
	sshArgs = append(sshArgs, sshutil.SSHArgsFromOpts(sshOpts)...)
	sshArgs = append(sshArgs, []string{
		"-q", // quiet
		"-f", // background
		"-N", // no command
		"-D", fmt.Sprintf("127.0.0.1:%d", port),
		"-p", strconv.Itoa(inst.SSHLocalPort),
		inst.SSHAddress,
	}...)
	sshCmd := exec.CommandContext(ctx, sshExe.Exe, sshArgs...)
	sshCmd.Stdout = stderr
	sshCmd.Stderr = stderr
	logrus.Debugf("executing ssh (may take a long)): %+v", sshCmd.Args)

	if err := sshCmd.Run(); err != nil {
		return err
	}

	switch runtime.GOOS {
	case "darwin":
		fmt.Fprint(stdout, "Open <System Settings> → <Network> → <Wi-Fi> (or whatever) → <Details> → <Proxies> → <SOCKS proxy>,\n")
		fmt.Fprint(stdout, "and specify the following configuration:\n")
		fmt.Fprint(stdout, "- Server: 127.0.0.1\n")
		fmt.Fprintf(stdout, "- Port: %d\n", port)
	case "windows":
		fmt.Fprint(stdout, "Open <Settings> → <Network & Internet> → <Proxy>,\n")
		fmt.Fprint(stdout, "and specify the following configuration:\n")
		fmt.Fprint(stdout, "- Address: socks=127.0.0.1\n")
		fmt.Fprintf(stdout, "- Port: %d\n", port)
	default:
		fmt.Fprintf(stdout, "Set `ALL_PROXY=socks5h://127.0.0.1:%d`, etc.\n", port)
	}
	fmt.Fprintf(stdout, "The instance can be connected from the host as <http://%s.internal> via a web browser.\n", inst.Hostname)

	// TODO: show the port in `limactl list --json` ?
	// TODO: add `--stop` flag to shut down the tunnel
	return nil
}

func tunnelBashComplete(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return bashCompleteInstanceNames(cmd)
}
