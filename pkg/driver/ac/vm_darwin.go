// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package ac

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/store"
	"github.com/lima-vm/lima/v2/pkg/store/filenames"
	"github.com/lima-vm/lima/v2/pkg/textutil"
)

// init system (pid 1) in VM.
const initSystem = "openrc"

// registerVM calls AC to register a VM.
func registerVM(ctx context.Context, distroName string, cpus, memory int) error {
	imageName := distroName
	entrypoint := "/sbin/init"
	// /sbin/init is normally just a symlink to systemd
	// eventually we might want to look inside the image
	switch initSystem {
	case "systemd":
		entrypoint = "/lib/systemd/systemd"
	case "openrc":
		entrypoint = "/sbin/openrc-init"
	default:
		logrus.Infof("unknown init system, running only vminitd")
	}
	out, err := exec.CommandContext(ctx,
		"container",
		"create",
		"--name",
		distroName,
		"--cpus",
		fmt.Sprintf("%d", cpus),
		"--memory",
		fmt.Sprintf("%dM", memory),
		"--entrypoint",
		entrypoint,
		imageName).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to run `container create %s`: %w (out=%q)",
			distroName, err, out)
	}
	return nil
}

// startVM calls AC to start a VM.
func startVM(ctx context.Context, distroName string) error {
	out, err := exec.CommandContext(ctx,
		"container",
		"start",
		distroName).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to run `container start %s`: %w (out=%q)",
			distroName, err, out)
	}
	return nil
}

// initVM calls AC to import a new VM specifically for Lima.
func initVM(ctx context.Context, instanceDir, distroName string) error {
	imageName := distroName
	dockerFile := filepath.Join(instanceDir, "Dockerfile")
	fileContents := fmt.Sprintf("FROM scratch\nADD %s /\n", filenames.BaseDisk)
	err := os.WriteFile(dockerFile, []byte(fileContents), 0o644)
	if err != nil {
		return err
	}
	baseDisk := filepath.Join(instanceDir, filenames.BaseDisk)
	logrus.Infof("Importing distro from %q to %q", baseDisk, instanceDir)
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	err = os.Chdir(instanceDir)
	if err != nil {
		return err
	}
	defer func() { _ = os.Chdir(wd) }()
	out, err := exec.CommandContext(ctx,
		"container",
		"build",
		"-t",
		imageName,
		instanceDir).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to run `container build -t %s %s`: %w (out=%q)",
			imageName, instanceDir, err, out)
	}
	return nil
}

// stopVM calls AC to stop a running VM.
func stopVM(ctx context.Context, distroName string) error {
	out, err := exec.CommandContext(ctx,
		"container",
		"stop",
		distroName).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to run `container stop %s`: %w (out=%q)",
			distroName, err, out)
	}
	return nil
}

//go:embed lima-init.TEMPLATE
var limaBoot string

// copyDir copies a directory.
func copyDir(ctx context.Context, distroName, src, dst string) error {
	cmd := exec.CommandContext(ctx, "container", "exec", distroName, "mkdir", "-p", dst)
	if err := cmd.Run(); err != nil {
		if exiterr, ok := err.(*exec.ExitError); ok {
			logrus.Debugf("run stderr: %s", exiterr.Stderr)
		}
		return fmt.Errorf("failed to run %v: %w", cmd.Args, err)
	}
	logrus.Infof("Copying directory from %q to \"%s:%s\"", src, distroName, dst)
	tar1 := exec.CommandContext(ctx, "tar", "Cc", src, ".")
	tar2 := exec.CommandContext(ctx, "container", "exec", "-i", distroName, "tar", "Cx", dst)

	p, err := tar1.StdoutPipe()
	if err != nil {
		return err
	}
	tar2.Stdin = p
	if err := tar2.Start(); err != nil {
		return err
	}
	if err := tar1.Run(); err != nil {
		return err
	}
	if err := tar2.Wait(); err != nil {
		return err
	}
	return nil
}

// provisionVM starts Lima's boot process inside an already imported VM.
func provisionVM(ctx context.Context, instanceDir, instanceName, distroName string, errCh chan<- error) error {
	ciDataPath := filepath.Join(instanceDir, filenames.CIDataISODir)
	// can't mount the cidata, due to problems with virtiofs mounts
	if err := copyDir(ctx, distroName, ciDataPath, "/mnt/lima-cidata"); err != nil {
		return fmt.Errorf("failed to copy cidata directory: %w", err)
	}
	m := map[string]string{
		"CIDataPath": "/mnt/lima-cidata",
	}
	limaBootB, err := textutil.ExecuteTemplate(limaBoot, m)
	if err != nil {
		return fmt.Errorf("failed to construct ac boot.sh script: %w", err)
	}
	go func() {
		cmd := exec.CommandContext(
			ctx,
			"container",
			"exec",
			"-i",
			distroName,
			"/bin/bash",
		)
		cmd.Stdin = bytes.NewReader(limaBootB)
		out, err := cmd.CombinedOutput()
		logrus.Debugf("%v: %q", cmd.Args, string(out))
		if err != nil {
			errCh <- fmt.Errorf(
				"error running command that executes boot.sh (%v): %w, "+
					"check /var/log/lima-init.log for more details (out=%q)", cmd.Args, err, string(out))
		}

		for {
			<-ctx.Done()
			logrus.Info("Context closed, stopping vm")
			if status, err := store.GetAcStatus(instanceName); err == nil &&
				status == store.StatusRunning {
				_ = stopVM(ctx, distroName)
			}
		}
	}()

	return err
}

// unregisterVM calls AC to unregister a VM.
func unregisterVM(ctx context.Context, distroName string) error {
	imageName := distroName
	logrus.Info("Unregistering AC VM")
	out, err := exec.CommandContext(ctx,
		"container",
		"rm",
		"-f",
		distroName).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to run `container rm -f %s`: %w (out=%q)",
			distroName, err, out)
	}
	out, err = exec.CommandContext(ctx,
		"container",
		"image",
		"rm",
		imageName).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to run `container image rm %s`: %w (out=%q)",
			distroName, err, out)
	}
	return nil
}
