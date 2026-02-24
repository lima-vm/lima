// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package asifutil

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/lima-vm/lima/v2/pkg/plist"
)

// NewASIF creates a new ASIF image file at the specified path with the given size.
func NewASIF(path string, size int64) error {
	createArgs := []string{"image", "create", "blank", "--fs", "none", "--format", "ASIF", "--size", strconv.FormatInt(size, 10), path}
	if err := exec.CommandContext(context.Background(), "diskutil", createArgs...).Run(); err != nil {
		return fmt.Errorf("failed to create ASIF image %q: %w", path, err)
	}
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		if _, err2 := os.Stat(path + ".asif"); !errors.Is(err2, os.ErrNotExist) {
			// diskutil may create the file with .asif suffix
			if err3 := os.Rename(path+".asif", path); err3 != nil {
				return fmt.Errorf("failed to rename ASIF image from %q to %q: %w", path+".asif", path, err3)
			}
		}
	}
	return nil
}

// NewAttachedASIF creates a new ASIF image file at the specified path with the given size
// and attaches it, returning the attached device path and an open file handle.
// The caller is responsible for detaching the ASIF image device when done.
func NewAttachedASIF(path string, size int64) (string, *os.File, error) {
	if err := NewASIF(path, size); err != nil {
		return "", nil, err
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

type AttachedDisk struct {
	Data string // e.g. "disk7s5"
}

// parseDiskutilImageAttachOutput parses the output of `diskutil image attach -plist -nomount <disk>`
// and returns the attached disk information.
func parseDiskutilImageAttachOutput(xmlStr string) (*AttachedDisk, error) {
	var p plist.Plist
	if err := xml.Unmarshal([]byte(xmlStr), &p); err != nil {
		return nil, fmt.Errorf("failed to unmarshal xml: %w", err)
	}

	if p.Value.Dict == nil {
		return nil, errors.New("unexpected plist format: missing root dict")
	}

	seVal, ok := p.Value.Dict["system-entities"]
	if !ok || len(seVal.Array) == 0 {
		return nil, errors.New("unexpected plist format: missing system-entities array")
	}

	for _, devEnt := range seVal.Array {
		devDict := devEnt.Dict
		if devDict == nil {
			continue
		}
		if role, ok := devDict["role"]; ok && role.String != nil && *role.String == "Data" {
			if devEntry, ok := devDict["dev-entry"]; ok && devEntry.String != nil && *devEntry.String != "" {
				return &AttachedDisk{Data: *devEntry.String}, nil
			}
		}
	}

	return nil, errors.New("no data device found in diskutil output")
}

var ErrResourceTemporarilyUnavailable = errors.New("resource temporarily unavailable")

// DiskutilImageAttachNoMount executes `diskutil image attach -plist -nomount <disk>`.
func DiskutilImageAttachNoMount(ctx context.Context, disk string) (*AttachedDisk, error) {
	cmd := exec.CommandContext(ctx, "diskutil", "image", "attach", "-plist", "-nomount", disk)
	// Enforce English output for parsing stderr
	cmd.Env = append(os.Environ(), "LANG=C", "LC_ALL=C")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		errToWrap := err
		if strings.Contains(stderr.String(), "Resource temporarily unavailable") {
			errToWrap = ErrResourceTemporarilyUnavailable
		}
		return nil, fmt.Errorf("failed to execute %v: %w (stderr: %q)", cmd.Args, errToWrap, stderr.String())
	}
	return parseDiskutilImageAttachOutput(stdout.String())
}
