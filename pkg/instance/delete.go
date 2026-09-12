// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package instance

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lima-vm/lima/v2/pkg/driver/external/server"
	"github.com/lima-vm/lima/v2/pkg/driverutil"
	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
	"github.com/lima-vm/lima/v2/pkg/osutil"
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
	if err := removeDiskTarget(inst.Dir); err != nil {
		return fmt.Errorf("failed to remove the disk of %#q: %w", inst.Dir, err)
	}
	if err := os.RemoveAll(inst.Dir); err != nil {
		return fmt.Errorf("failed to remove %#q: %w", inst.Dir, err)
	}

	return nil
}

// removeDiskTarget removes the disk file when `disk` is a symlink to a file.
func removeDiskTarget(instDir string) error {
	diskPath := filepath.Join(instDir, filenames.Disk)
	if !osutil.IsSymlink(diskPath) {
		return nil
	}
	target, err := os.Readlink(diskPath)
	if err != nil {
		return fmt.Errorf("failed to read symlink %#q: %w", diskPath, err)
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(instDir, target)
	}
	// Targets inside the instance directory (e.g. the legacy `diffdisk`) are
	// removed together with the instance directory.
	if rel, err := filepath.Rel(instDir, target); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return nil
	}

	if err := os.Remove(target); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to remove %#q: %w", target, err)
	}
	return nil
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
