// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package asifutil

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// NewAttachedASIF creates a new ASIF image file at the specified path with the given size
// and attaches it, returning the attached device path and an open file handle.
// The caller is responsible for detaching the ASIF image device when done.
func NewAttachedASIF(path string, size int64) (string, *os.File, error) {
	createArgs := []string{"image", "create", "blank", "--fs", "none", "--format", "ASIF", "--size", fmt.Sprintf("%d", size), path}
	if err := exec.CommandContext(context.Background(), "diskutil", createArgs...).Run(); err != nil {
		return "", nil, fmt.Errorf("failed to create ASIF image %q: %w", path, err)
	}
	attachArgs := []string{"image", "attach", "--noMount", path}
	out, err := exec.CommandContext(context.Background(), "diskutil", attachArgs...).Output()
	if err != nil {
		return "", nil, fmt.Errorf("failed to attach ASIF image %q: %w", path, err)
	}
	devicePath := strings.TrimSpace(string(out))
	f, err := os.OpenFile(devicePath, os.O_RDWR, 0o644)
	if err != nil {
		_ = DetachASIF(devicePath)
		return "", nil, fmt.Errorf("failed to open ASIF device %q: %w", devicePath, err)
	}
	return devicePath, f, err
}

// DetachASIF detaches the ASIF image device at the specified path.
func DetachASIF(devicePath string) error {
	if output, err := exec.CommandContext(context.Background(), "hdiutil", "detach", devicePath).CombinedOutput(); err != nil {
		return fmt.Errorf("failed to detach ASIF image %q: %w: %s", devicePath, err, output)
	}
	return nil
}

// ResizeASIF resizes the ASIF image at the specified path to the given size.
func ResizeASIF(path string, size int64) error {
	resizeArgs := []string{"image", "resize", "--size", fmt.Sprintf("%d", size), path}
	if output, err := exec.CommandContext(context.Background(), "diskutil", resizeArgs...).CombinedOutput(); err != nil {
		return fmt.Errorf("failed to resize ASIF image %q: %w: %s", path, err, output)
	}
	return nil
}
