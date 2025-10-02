// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package wsl2

import (
	"context"
	_ "embed"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/executil"
	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
	"github.com/lima-vm/lima/v2/pkg/textutil"
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
	limaBootFilePathOnLinuxB, err := exec.CommandContext(
		ctx,
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
			if status, err := getWslStatus(ctx, instanceName); err == nil &&
				status == limatype.StatusRunning {
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

// GetWslStatus runs `wsl --list --verbose` and parses its output.
// There are several possible outputs, all listed with their whitespace preserved output below.
//
// (1) Expected output if at least one distro is installed:
// PS > wsl --list --verbose
//
//	NAME      STATE           VERSION
//
// * Ubuntu    Stopped         2
//
// (2) Expected output when no distros are installed, but WSL is configured properly:
// PS > wsl --list --verbose
// Windows Subsystem for Linux has no installed distributions.
//
// Use 'wsl.exe --list --online' to list available distributions
// and 'wsl.exe --install <Distro>' to install.
//
// Distributions can also be installed by visiting the Microsoft Store:
// https://aka.ms/wslstore
// Error code: Wsl/WSL_E_DEFAULT_DISTRO_NOT_FOUND
//
// (3) Expected output when no distros are installed, and WSL2 has no kernel installed:
//
// PS > wsl --list --verbose
// Windows Subsystem for Linux has no installed distributions.
// Distributions can be installed by visiting the Microsoft Store:
// https://aka.ms/wslstore
func getWslStatus(ctx context.Context, instName string) (string, error) {
	distroName := "lima-" + instName
	out, err := executil.RunUTF16leCommand([]string{
		"wsl.exe",
		"--list",
		"--verbose",
	}, executil.WithContext(ctx))
	if err != nil {
		return "", fmt.Errorf("failed to run `wsl --list --verbose`, err: %w (out=%q)", err, out)
	}

	if out == "" {
		return limatype.StatusBroken, fmt.Errorf("failed to read instance state for instance %q, try running `wsl --list --verbose` to debug, err: %w", instName, err)
	}

	// Check for edge cases first
	if strings.Contains(out, "Windows Subsystem for Linux has no installed distributions.") {
		if strings.Contains(out, "Wsl/WSL_E_DEFAULT_DISTRO_NOT_FOUND") {
			return limatype.StatusBroken, fmt.Errorf(
				"failed to read instance state for instance %q because no distro is installed,"+
					"try running `wsl --install -d Ubuntu` and then re-running Lima", instName)
		}
		return limatype.StatusBroken, fmt.Errorf(
			"failed to read instance state for instance %q because there is no WSL kernel installed,"+
				"this usually happens when WSL was installed for another user, but never for your user."+
				"Try running `wsl --install -d Ubuntu` and `wsl --update`, and then re-running Lima", instName)
	}

	var instState string
	wslListColsRegex := regexp.MustCompile(`\s+`)
	// wsl --list --verbose may have different headers depending on localization, just split by line
	for _, rows := range strings.Split(strings.ReplaceAll(out, "\r\n", "\n"), "\n") {
		cols := wslListColsRegex.Split(strings.TrimSpace(rows), -1)
		nameIdx := 0
		// '*' indicates default instance
		if cols[0] == "*" {
			nameIdx = 1
		}
		if cols[nameIdx] == distroName {
			instState = cols[nameIdx+1]
			break
		}
	}

	if instState == "" {
		return limatype.StatusUninitialized, nil
	}

	return instState, nil
}

// GetSSHAddress runs a hostname command to get the IP from inside of a wsl2 VM.
//
// Expected output (whitespace preserved, [] for optional):
// PS > wsl -d <distroName> bash -c hostname -I | cut -d' ' -f1
// 168.1.1.1 [10.0.0.1]
// But busybox hostname does not implement --all-ip-addresses:
// hostname: unrecognized option: I
func getSSHAddress(ctx context.Context, instName string) (string, error) {
	distroName := "lima-" + instName
	// Ubuntu
	cmd := exec.CommandContext(ctx, "wsl.exe", "-d", distroName, "bash", "-c", `hostname -I | cut -d ' ' -f1`)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return strings.TrimSpace(string(out)), nil
	}
	// Alpine
	cmd = exec.CommandContext(ctx, "wsl.exe", "-d", distroName, "sh", "-c", `ip route get 1 | awk '{gsub("^.*src ",""); print $1; exit}'`)
	out, err = cmd.CombinedOutput()
	if err == nil {
		return strings.TrimSpace(string(out)), nil
	}
	// fallback
	cmd = exec.CommandContext(ctx, "wsl.exe", "-d", distroName, "hostname", "-i")
	out, err = cmd.CombinedOutput()
	if err == nil {
		ip := net.ParseIP(strings.TrimSpace(string(out)))
		// some distributions use "127.0.1.1" as the host IP, but we want something that we can route to here
		if ip != nil && !ip.IsLoopback() {
			return strings.TrimSpace(string(out)), nil
		}
	}
	return "", fmt.Errorf("failed to get hostname for instance %q, err: %w (out=%q)", instName, err, string(out))
}
