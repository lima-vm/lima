// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"context"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/lima-vm/lima/v2/pkg/cacheutil"
	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
	"github.com/lima-vm/lima/v2/pkg/store"
	"github.com/lima-vm/lima/v2/pkg/usrlocalsharelima"
)

func newGuestInstallCommand() *cobra.Command {
	guestInstallCommand := &cobra.Command{
		Use:               "guest-install INSTANCE",
		Short:             "Install guest components",
		Args:              WrapArgsError(cobra.MaximumNArgs(1)),
		RunE:              guestInstallAction,
		ValidArgsFunction: cobra.NoFileCompletions,
		Hidden:            true,
	}
	return guestInstallCommand
}

func runCmd(ctx context.Context, name string, flags []string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, append(flags, args...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	logrus.Debugf("executing %v", cmd.Args)
	return cmd.Run()
}

func guestInstallAction(cmd *cobra.Command, args []string) error {
	instName := DefaultInstanceName
	if len(args) > 0 {
		instName = args[0]
	}

	inst, err := store.Inspect(cmd.Context(), instName)
	if err != nil {
		return err
	}
	if inst.Status == limatype.StatusStopped {
		return fmt.Errorf("instance %q is stopped, run `limactl start %s` to start the instance", instName, instName)
	}

	ctx := cmd.Context()

	sshExe := "ssh"
	sshConfig := filepath.Join(inst.Dir, filenames.SSHConfig)
	sshFlags := []string{"-F", sshConfig}

	scpExe := "scp"
	scpFlags := sshFlags

	hostname := fmt.Sprintf("lima-%s", inst.Name)
	prefix := *inst.Config.GuestInstallPrefix

	// lima-guestagent
	guestAgentBinary, err := usrlocalsharelima.GuestAgentBinary(*inst.Config.OS, *inst.Config.Arch)
	if err != nil {
		return err
	}
	tmp := "/tmp/lima-guestagent"
	bin := prefix + "/bin/lima-guestagent"
	logrus.Infof("Copying %q to %s", guestAgentBinary, hostname)
	scpArgs := []string{guestAgentBinary, hostname + ":" + tmp}
	if err := runCmd(ctx, scpExe, scpFlags, scpArgs...); err != nil {
		return nil
	}
	logrus.Infof("Installing %s to %s", tmp, bin)
	sshArgs := []string{hostname, "sudo", "install", "-m", "755", tmp, bin}
	if err := runCmd(ctx, sshExe, sshFlags, sshArgs...); err != nil {
		return nil
	}

	// nerdctl-full.tgz
	nerdctlFilename := cacheutil.NerdctlArchive(inst.Config)
	if nerdctlFilename != "" {
		nerdctlArchive, err := cacheutil.EnsureNerdctlArchiveCache(cmd.Context(), inst.Config, false)
		if err != nil {
			return err
		}
		tmp := "/tmp/nerdctl-full.tgz"
		logrus.Infof("Copying %q to %s", nerdctlFilename, hostname)
		scpArgs := []string{nerdctlArchive, hostname + ":" + tmp}
		if err := runCmd(ctx, scpExe, scpFlags, scpArgs...); err != nil {
			return nil
		}
		logrus.Infof("Installing %s in %s", tmp, prefix)
		sshArgs := []string{hostname, "sudo", "tar", "Cxzf", prefix, tmp}
		if err := runCmd(ctx, sshExe, sshFlags, sshArgs...); err != nil {
			return nil
		}
	}

	return nil
}
