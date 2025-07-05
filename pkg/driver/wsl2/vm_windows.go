// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package wsl2

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/pkg/executil"
	"github.com/lima-vm/lima/pkg/store"
	"github.com/lima-vm/lima/pkg/store/filenames"
	"github.com/lima-vm/lima/pkg/textutil"
)

// startVM calls WSL to start a VM.
func startVM(ctx context.Context, distroName string) error {
	out, err := executil.RunUTF16leCommand([]string{
		"wsl.exe",
		"--distribution",
		distroName,
	}, executil.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("failed to run `wsl.exe --distribution %s`: %w (out=%q)",
			distroName, err, out)
	}
	return nil
}

// initVM calls WSL to import a new VM specifically for Lima.
func initVM(ctx context.Context, instanceDir, distroName string) error {
	baseDisk := filepath.Join(instanceDir, filenames.BaseDisk)
	logrus.Infof("Importing distro from %q to %q", baseDisk, instanceDir)
	out, err := executil.RunUTF16leCommand([]string{
		"wsl.exe",
		"--import",
		distroName,
		instanceDir,
		baseDisk,
	}, executil.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("failed to run `wsl.exe --import %s %s %s`: %w (out=%q)",
			distroName, instanceDir, baseDisk, err, out)
	}
	return nil
}

// stopVM calls WSL to stop a running VM.
func stopVM(ctx context.Context, distroName string) error {
	out, err := executil.RunUTF16leCommand([]string{
		"wsl.exe",
		"--terminate",
		distroName,
	}, executil.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("failed to run `wsl.exe --terminate %s`: %w (out=%q)",
			distroName, err, out)
	}
	return nil
}

//go:embed lima-init.TEMPLATE
var limaBoot string

// provisionVM starts Lima's boot process inside an already imported VM.
func provisionVM(ctx context.Context, instanceDir, instanceName, distroName string, errCh chan<- error) error {
	ciDataPath := filepath.Join(instanceDir, filenames.CIDataISODir)
	m := map[string]string{
		"CIDataPath": ciDataPath,
	}
	limaBootB, err := textutil.ExecuteTemplate(limaBoot, m)
	if err != nil {
		return fmt.Errorf("failed to construct wsl boot.sh script: %w", err)
	}
	limaBootFile, err := os.CreateTemp("", "lima-wsl2-boot-*.sh")
	if err != nil {
		return err
	}
	if _, err = limaBootFile.Write(limaBootB); err != nil {
		limaBootFile.Close()
		return err
	}
	limaBootFileWinPath := limaBootFile.Name()
	if err = limaBootFile.Close(); err != nil {
		return err
	}
	// path should be quoted and use \\ as separator
	bootFileWSLPath := strconv.Quote(limaBootFileWinPath)
	limaBootFilePathOnLinuxB, err := exec.Command(
		"wsl.exe",
		"-d",
		distroName,
		"bash",
		"-c",
		fmt.Sprintf("wslpath -u %s", bootFileWSLPath),
		bootFileWSLPath,
	).Output()
	if err != nil {
		os.RemoveAll(limaBootFileWinPath)
		// this can return an error with an exit code, which causes it not to be logged
		// because main.handleExitCoder() traps it, so wrap the error
		return fmt.Errorf("failed to run wslpath command: %w", err)
	}
	limaBootFileLinuxPath := strings.TrimSpace(string(limaBootFilePathOnLinuxB))
	go func() {
		cmd := exec.CommandContext(
			ctx,
			"wsl.exe",
			"-d",
			distroName,
			"bash",
			"-c",
			limaBootFileLinuxPath,
		)
		out, err := cmd.CombinedOutput()
		os.RemoveAll(limaBootFileWinPath)
		logrus.Debugf("%v: %q", cmd.Args, string(out))
		if err != nil {
			errCh <- fmt.Errorf(
				"error running wslCommand that executes boot.sh (%v): %w, "+
					"check /var/log/lima-init.log for more details (out=%q)", cmd.Args, err, string(out))
		}

		for {
			<-ctx.Done()
			logrus.Info("Context closed, stopping vm")
			if status, err := store.GetWslStatus(instanceName); err == nil &&
				status == store.StatusRunning {
				_ = stopVM(ctx, distroName)
			}
		}
	}()

	return err
}

// keepAlive runs a background process which in order to keep the WSL2 VM running in the background after launch.
func keepAlive(ctx context.Context, distroName string, errCh chan<- error) {
	keepAliveCmd := exec.CommandContext(
		ctx,
		"wsl.exe",
		"-d",
		distroName,
		"bash",
		"-c",
		"nohup sleep 2147483647d >/dev/null 2>&1",
	)

	go func() {
		if err := keepAliveCmd.Run(); err != nil {
			errCh <- fmt.Errorf(
				"error running wsl keepAlive command: %w", err)
		}
	}()
}

// unregisterVM calls WSL to unregister a VM.
func unregisterVM(ctx context.Context, distroName string) error {
	logrus.Info("Unregistering WSL2 VM")
	out, err := executil.RunUTF16leCommand([]string{
		"wsl.exe",
		"--unregister",
		distroName,
	}, executil.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("failed to run `wsl.exe --unregister %s`: %w (out=%q)",
			distroName, err, out)
	}
	return nil
}
