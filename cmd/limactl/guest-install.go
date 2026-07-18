// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/lima-vm/lima/v2/pkg/cacheutil"
	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
	"github.com/lima-vm/lima/v2/pkg/store"
	"github.com/lima-vm/lima/v2/pkg/usrlocal"
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

func shell(ctx context.Context, name string, flags []string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, append(flags, args...)...)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	out = bytes.TrimSuffix(out, []byte{'\n'})
	return string(out), nil
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
	guestAgentBinary, err := usrlocal.GuestAgentBinary(*inst.Config.OS, *inst.Config.Arch)
	if err != nil {
		return err
	}
	guestAgentFilename := filepath.Base(guestAgentBinary)
	if filepath.Ext(guestAgentBinary) == ".gz" {
		compressedGuestAgent, err := os.Open(guestAgentBinary)
		if err != nil {
			return err
		}
		defer compressedGuestAgent.Close()
		tmpGuestAgent, err := os.CreateTemp("", "lima-guestagent-")
		if err != nil {
			return err
		}
		logrus.Debugf("Decompressing %s", guestAgentBinary)
		guestAgent, err := gzip.NewReader(compressedGuestAgent)
		if err != nil {
			return err
		}
		defer guestAgent.Close()
		_, err = io.Copy(tmpGuestAgent, guestAgent)
		if err != nil {
			return err
		}
		tmpGuestAgent.Close()
		guestAgentBinary = tmpGuestAgent.Name()
		defer os.RemoveAll(guestAgentBinary)
		guestAgentFilename = strings.TrimSuffix(guestAgentFilename, ".gz")
	}
	tmpname := "lima-guestagent"
	tmp, err := shell(ctx, sshExe, sshFlags, hostname, "mktemp", "-t", "lima-guestagent.XXXXXX")
	if err != nil {
		return err
	}
	bin := prefix + "/bin/lima-guestagent"
	logrus.Infof("Copying %q to %s:%s", guestAgentFilename, inst.Name, tmpname)
	scpArgs := []string{guestAgentBinary, hostname + ":" + tmp}
	if err := runCmd(ctx, scpExe, scpFlags, scpArgs...); err != nil {
		return nil
	}
	logrus.Infof("Installing %s to %s", tmpname, bin)
	sshArgs := []string{hostname, "sudo", "install", "-m", "755", tmp, bin}
	if err := runCmd(ctx, sshExe, sshFlags, sshArgs...); err != nil {
		return nil
	}
	_, _ = shell(ctx, sshExe, sshFlags, hostname, "rm", tmp)

	// nerdctl-full.tgz
	nerdctlFilename := cacheutil.NerdctlArchive(inst.Config)
	if nerdctlFilename != "" {
		nerdctlArchive, err := cacheutil.EnsureNerdctlArchiveCache(cmd.Context(), inst.Config, false)
		if err != nil {
			return err
		}
		tmpname := "nerdctl-full.tgz"
		tmp, err := shell(ctx, sshExe, sshFlags, hostname, "mktemp", "-t", "nerdctl-full.XXXXXX.tgz")
		if err != nil {
			return err
		}
		logrus.Infof("Copying %q to %s:%s", nerdctlFilename, inst.Name, tmpname)
		scpArgs := []string{nerdctlArchive, hostname + ":" + tmp}
		if err := runCmd(ctx, scpExe, scpFlags, scpArgs...); err != nil {
			return nil
		}
		logrus.Infof("Installing %s in %s", tmpname, prefix)
		sshArgs := []string{hostname, "sudo", "tar", "Cxzf", prefix, tmp}
		if err := runCmd(ctx, sshExe, sshFlags, sshArgs...); err != nil {
			return nil
		}
		_, _ = shell(ctx, sshExe, sshFlags, hostname, "rm", tmp)
	}

	return nil
}
