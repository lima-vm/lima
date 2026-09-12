// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package instance

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/lima-vm/lima/v2/pkg/driver/external/server"
	"github.com/lima-vm/lima/v2/pkg/driverutil"
	"github.com/lima-vm/lima/v2/pkg/limatype"
)

func Delete(ctx context.Context, inst *limatype.Instance, force bool) error {
	if inst.Protected {
		return errors.New("instance is protected to prohibit accidental removal (Hint: use `limactl unprotect`)")
	}
	if !force && inst.Status != limatype.StatusStopped {
		return fmt.Errorf("expected status %#q, got %#q", limatype.StatusStopped, inst.Status)
	}

	StopForcibly(inst)

	if len(inst.Errors) == 0 {
		if err := unregister(ctx, inst); err != nil {
			return fmt.Errorf("failed to unregister %#q: %w", inst.Dir, err)
		}
	}
	if err := removeDir(inst.Dir); err != nil {
		return fmt.Errorf("failed to remove %#q: %w", inst.Dir, err)
	}

	return nil
}

func removeDir(path string) error {
	fileInfo, err := os.Lstat(path) // Use Lstat to not follow symlinks
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Path doesn't exist, treat as success
		}
		return fmt.Errorf("failed to get path info: %w", err)
	}

	// Check if it's a symlink
	if (fileInfo.Mode() & os.ModeSymlink) != 0 {
		target, err := os.Readlink(path)
		if err != nil {
			return fmt.Errorf("failed to read symlink: %w", err)
		}

		// Remove the target content first
		if err := os.RemoveAll(target); err != nil {
			return fmt.Errorf("failed to remove symlink target: %w", err)
		}

		// Remove the symlink itself
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("failed to remove symlink: %w", err)
		}
		return nil
	}

	// Remove regular directory
	if fileInfo.IsDir() {
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("failed to remove directory: %w", err)
		}
		return nil
	}
	return fmt.Errorf("path is neither a directory nor a symlink: %s", path)
}

func unregister(ctx context.Context, inst *limatype.Instance) error {
	limaDriver, err := driverutil.CreateConfiguredDriver(ctx, inst, 0)
	if err != nil {
		return fmt.Errorf("failed to create driver instance: %w", err)
	}

	if err := limaDriver.Delete(ctx); err != nil {
		return fmt.Errorf("failed to delete driver instance: %w", err)
	}
	server.Stop(inst.Dir, true)

	return nil
}
