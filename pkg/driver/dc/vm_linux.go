// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package dc

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
	"github.com/lima-vm/lima/v2/pkg/textutil"
)

// init system (pid 1) in VM.
const initSystem = "openrc"

// OCI runtime for container.
const ociRuntime = "runc"

// createVM calls DC to create a VM.
func createVM(ctx context.Context, distroName string, cpus, memory int) error {
	imageName := distroName
	entrypoint := "--init"
	// /sbin/init is normally just a symlink to systemd
	// eventually we might want to look inside the image
	// note: docker does not seem to resolve symlinks here
	// so need to give canonical path, after resolving root
	switch initSystem {
	case "systemd":
		entrypoint = "--entrypoint=/usr/lib/systemd/systemd"
	case "openrc":
		entrypoint = "--entrypoint=/usr/sbin/openrc-init"
	default:
		logrus.Infof("unknown init system, running only --init")
	}
	out, err := exec.CommandContext(ctx,
		"docker",
		"create",
		"--name",
		distroName,
		"--cpus",
		fmt.Sprintf("%d", cpus),
		"--memory",
		fmt.Sprintf("%d", int64(memory)<<20),
		"--runtime",
		ociRuntime,
		"--tmpfs",
		"/run", // for systemd (and openrc)
		entrypoint,
		imageName,
		"sleep",
		"infinity").CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to run `docker create %s`: %w (out=%q)",
			distroName, err, out)
	}
	return nil
}

// startVM calls DC to start a VM.
func startVM(ctx context.Context, distroName string) error {
	out, err := exec.CommandContext(ctx,
		"docker",
		"start",
		distroName).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to run `docker start %s`: %w (out=%q)",
			distroName, err, out)
	}
	return nil
}

// initVM calls DC to import a new VM specifically for Lima.
func initVM(ctx context.Context, instanceDir, distroName string) error {
	imageName := distroName
	baseDisk := filepath.Join(instanceDir, filenames.BaseDisk)
	logrus.Infof("Importing distro from %q to %q", baseDisk, instanceDir)
	out, err := exec.CommandContext(ctx,
		"docker",
		"import",
		baseDisk,
		imageName,
	).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to run `docker import %s %s`: %w (out=%q)",
			baseDisk, imageName, err, out)
	}
	return nil
}

// stopVM calls DC to stop a running VM.
func stopVM(ctx context.Context, distroName string) error {
	out, err := exec.CommandContext(ctx,
		"docker",
		"stop",
		distroName).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to run `docker stop %s`: %w (out=%q)",
			distroName, err, out)
	}
	return nil
}

//go:embed lima-init.TEMPLATE
var limaBoot string

// copyDir copies a directory.
func copyDir(ctx context.Context, distroName, src, dst string) error {
	cmd := exec.CommandContext(ctx, "docker", "exec", distroName, "mkdir", "-p", dst)
	tar1 := exec.CommandContext(ctx, "tar", "Cc", src, ".")
	tar2 := exec.CommandContext(ctx, "docker", "exec", "-i", distroName, "tar", "Cx", dst)

	// "RunQ does not support 'docker exec'. Use 'runq-exec' instead."
	if ociRuntime == "runq" {
		runqExec := "/var/lib/runq/runq-exec"
		time.Sleep(10 * time.Millisecond)
		cmd = exec.CommandContext(ctx, "sudo", runqExec, distroName, "mkdir", "-p", dst)
		cmd.Stderr = os.Stderr
		tar2 = exec.CommandContext(ctx, "sudo", runqExec, "-i", distroName, "tar", "Cx", dst)
	}

	if err := cmd.Run(); err != nil {
		if exiterr, ok := err.(*exec.ExitError); ok {
			logrus.Debugf("run stderr: %s", exiterr.Stderr)
		}
		return fmt.Errorf("failed to run %v: %w", cmd.Args, err)
	}
	logrus.Infof("Copying directory from %q to \"%s:%s\"", src, distroName, dst)

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
		return fmt.Errorf("failed to construct dc boot.sh script: %w", err)
	}
	go func() {
		cmd := exec.CommandContext(
			ctx,
			"docker",
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
			if status, err := getDcStatus(ctx, instanceName); err == nil &&
				status == limatype.StatusRunning {
				_ = stopVM(ctx, distroName)
			}
		}
	}()

	return err
}

// deleteVM calls DC to delete a VM.
func deleteVM(ctx context.Context, distroName string) error {
	imageName := distroName
	logrus.Info("Deleting DC VM")
	out, err := exec.CommandContext(ctx,
		"docker",
		"rm",
		"-f",
		distroName).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to run `docker rm -f %s`: %w (out=%q)",
			distroName, err, out)
	}
	out, err = exec.CommandContext(ctx,
		"docker",
		"image",
		"rm",
		imageName).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to run `docker image rm %s`: %w (out=%q)",
			distroName, err, out)
	}
	return nil
}

func inspectContainer(ctx context.Context, distroName, format string) (string, error) {
	out, err := exec.CommandContext(
		ctx,
		"docker",
		"inspect",
		"--format="+format,
		distroName,
	).CombinedOutput()
	if err != nil {
		if strings.Contains(string(out), fmt.Sprintf("No such object: %s", distroName)) {
			return "", nil
		}
		if strings.Contains(string(out), "map has no entry for key") {
			return "", nil
		}
		return "", fmt.Errorf("failed to run `docker inspect`, err: %w (out=%q)", err, out)
	}
	return strings.TrimSuffix(string(out), "\n"), nil
}

func getDcStatus(ctx context.Context, instName string) (string, error) {
	distroName := "lima-" + instName
	out, err := inspectContainer(ctx, distroName, "{{ .State.Status }}")
	if err != nil {
		return "", err
	}
	if out == "" {
		return limatype.StatusUninitialized, nil
	}

	var instState string
	switch out {
	case "exited":
		instState = limatype.StatusStopped
	case "running":
		instState = limatype.StatusRunning
	default:
		instState = limatype.StatusUnknown
	}

	return instState, nil
}

func getSSHAddress(ctx context.Context, instName string) (string, error) {
	distroName := "lima-" + instName
	out, err := inspectContainer(ctx, distroName, "{{ .NetworkSettings.IPAddress }}")
	if err != nil {
		return "", err
	}
	if out == "" {
		return "127.0.0.1", nil
	}

	instAddress := out

	return instAddress, nil
}
