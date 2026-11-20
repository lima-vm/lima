// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package driverutil

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"

	"github.com/lima-vm/lima/v2/pkg/driver"
	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
	"github.com/lima-vm/lima/v2/pkg/osutil"
)

// snapshotsDirName is the directory in the instance directory that holds
// one directory per snapshot, such as snapshots/<tag>/disk.
const snapshotsDirName = "snapshots"

// CreateDiskSnapshot snapshots a stopped instance with clonefile(2). It clones the boot disk,
// and each of extraFiles that exists, into snapshots/<tag>/ in the instance directory.
// extraFiles are other files in the instance directory that have to match the disk,
// such as an EFI variable store.
// The clones have to be on the same APFS volume as the instance directory; if clonefile
// fails, the snapshot fails instead of falling back to a full copy.
func CreateDiskSnapshot(inst *limatype.Instance, tag string, extraFiles ...string) error {
	if err := requireStopped(inst); err != nil {
		return err
	}
	if err := validateSnapshotTag(tag); err != nil {
		return err
	}
	destDir := filepath.Join(inst.Dir, snapshotsDirName, tag)
	if _, err := os.Stat(destDir); err == nil {
		return fmt.Errorf("snapshot %q already exists", tag)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Join(inst.Dir, snapshotsDirName), 0o755); err != nil {
		return err
	}
	// Clone into a directory owned by this call and rename it into place, so a
	// failure only cleans up that directory and a concurrent create of the same
	// tag fails on the rename.
	tmpDir, err := os.MkdirTemp(inst.Dir, "."+tag+"-")
	if err != nil {
		return err
	}
	for i, name := range append([]string{filenames.Disk}, extraFiles...) {
		src := filepath.Join(inst.Dir, name)
		if i > 0 && !osutil.FileExists(src) {
			continue
		}
		dst := filepath.Join(tmpDir, name)
		if err := unix.Clonefile(src, dst, 0); err != nil {
			_ = os.RemoveAll(tmpDir)
			return fmt.Errorf("clonefile %#q to %#q: %w", src, dst, err)
		}
	}
	if err := os.Rename(tmpDir, destDir); err != nil {
		_ = os.RemoveAll(tmpDir)
		return fmt.Errorf("rename %#q to %#q: %w", tmpDir, destDir, err)
	}
	warnPartialSnapshot(inst)
	return nil
}

// ApplyDiskSnapshot restores the boot disk of a stopped instance from the snapshot,
// along with each of extraFiles that the snapshot has.
func ApplyDiskSnapshot(inst *limatype.Instance, id string, extraFiles ...string) error {
	if err := requireStopped(inst); err != nil {
		return err
	}
	if err := validateSnapshotTag(id); err != nil {
		return err
	}
	snapDir := filepath.Join(inst.Dir, snapshotsDirName, id)
	if _, err := os.Stat(filepath.Join(snapDir, filenames.Disk)); err != nil {
		return fmt.Errorf("snapshot %q: %w", id, err)
	}
	// Clone all the files first, and only rename the clones into place after all of
	// them succeeded, so that a failed clone leaves the instance unchanged.
	type clone struct{ tmp, dst string }
	var clones []clone
	defer func() {
		for _, c := range clones {
			_ = os.Remove(c.tmp) // already gone if the rename succeeded
		}
	}()
	for i, name := range append([]string{filenames.Disk}, extraFiles...) {
		src := filepath.Join(snapDir, name)
		if i > 0 && !osutil.FileExists(src) {
			continue
		}
		dst, err := resolveSymlink(filepath.Join(inst.Dir, name))
		if err != nil {
			return err
		}
		tmp := dst + ".snap-tmp"
		_ = os.Remove(tmp)
		if err := unix.Clonefile(src, tmp, 0); err != nil {
			return fmt.Errorf("clonefile %#q to %#q: %w", src, tmp, err)
		}
		clones = append(clones, clone{tmp: tmp, dst: dst})
	}
	for _, c := range clones {
		if err := os.Rename(c.tmp, c.dst); err != nil {
			return err
		}
	}
	warnPartialSnapshot(inst)
	return nil
}

// resolveSymlink returns the file that path points to when path is a symlink, such as
// the `disk` -> `diffdisk` link of an instance created by an older version of Lima,
// so that applying a snapshot replaces that file and keeps the link.
func resolveSymlink(path string) (string, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if errors.Is(err, os.ErrNotExist) {
		return path, nil
	}
	return resolved, err
}

// DeleteDiskSnapshot deletes a snapshot of a stopped instance.
func DeleteDiskSnapshot(inst *limatype.Instance, id string) error {
	if err := requireStopped(inst); err != nil {
		return err
	}
	if err := validateSnapshotTag(id); err != nil {
		return err
	}
	dir := filepath.Join(inst.Dir, snapshotsDirName, id)
	if _, err := os.Stat(dir); err != nil {
		return fmt.Errorf("snapshot %q: %w", id, err)
	}
	return os.RemoveAll(dir)
}

// ListDiskSnapshots lists the snapshots created by CreateDiskSnapshot.
func ListDiskSnapshots(inst *limatype.Instance) ([]driver.Snapshot, error) {
	entries, err := os.ReadDir(filepath.Join(inst.Dir, snapshotsDirName))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	snapshots := make([]driver.Snapshot, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		tag := entry.Name()
		snapshots = append(snapshots, driver.Snapshot{ID: tag, Tag: tag})
	}
	return snapshots, nil
}

func requireStopped(inst *limatype.Instance) error {
	if inst == nil || inst.Config == nil || inst.Config.VMType == nil {
		return errors.New("instance is not configured")
	}
	if inst.Status != limatype.StatusStopped {
		return fmt.Errorf("%s disk snapshots require a stopped instance (status %q)", *inst.Config.VMType, inst.Status)
	}
	return nil
}

func validateSnapshotTag(tag string) error {
	if tag == "." || tag == ".." || tag != filepath.Base(tag) || strings.Contains(tag, `\`) {
		return fmt.Errorf("invalid snapshot tag %q", tag)
	}
	for _, r := range tag {
		if unicode.IsSpace(r) {
			return fmt.Errorf("invalid snapshot tag %q", tag)
		}
	}
	return nil
}

func warnPartialSnapshot(inst *limatype.Instance) {
	if len(inst.Config.AdditionalDisks) == 0 {
		return
	}
	names := make([]string, len(inst.Config.AdditionalDisks))
	for i, d := range inst.Config.AdditionalDisks {
		names[i] = d.Name
	}
	logrus.Warnf("%s snapshot covers only the boot disk; additional disks not included: %s", *inst.Config.VMType, strings.Join(names, ", "))
}
