// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package driverutil

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/lima-vm/go-qcow2reader"
	"github.com/lima-vm/go-qcow2reader/image"
	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/iso9660util"
	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
	"github.com/lima-vm/lima/v2/pkg/osutil"
)

const (
	typeRAW  = image.Type("raw")
	typeASIF = image.Type("asif")
)

func writeMinimalISO(t *testing.T, path string) {
	t.Helper()
	entries := []iso9660util.Entry{
		{Path: "/hello.txt", Reader: strings.NewReader("hello world")},
	}
	assert.NilError(t, iso9660util.Write(path, "TESTISO", entries))
}

func writeNonISO(t *testing.T, path string) {
	t.Helper()
	size := 64 * 1024
	buf := make([]byte, size)
	copy(buf[0x8001:], "XXXXX")
	assert.NilError(t, os.WriteFile(path, buf, 0o644))
}

func sha256File(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	assert.NilError(t, err)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func detectImageType(t *testing.T, path string) image.Type {
	t.Helper()
	f, err := os.Open(path)
	assert.NilError(t, err)
	defer f.Close()
	img, err := qcow2reader.Open(f)
	assert.NilError(t, err)
	return img.Type()
}

func checkDisk(t *testing.T, diskPath string, expectedType image.Type) {
	t.Helper()
	fi, err := os.Stat(diskPath)
	assert.NilError(t, err)
	assert.Assert(t, fi.Size() > 0)
	assert.Equal(t, detectImageType(t, diskPath), expectedType)
}

func assertSymlink(t *testing.T, path, expectedTarget string) {
	t.Helper()
	target, err := os.Readlink(path)
	assert.NilError(t, err)
	assert.Equal(t, target, expectedTarget)
}

func isMacOS26OrHigher() bool {
	if runtime.GOOS != "darwin" {
		return false
	}
	version, err := osutil.ProductVersion()
	if err != nil {
		return false
	}
	return version.Major >= 26
}

func TestEnsureDisk_WithISOImage(t *testing.T) {
	instDir := t.TempDir()
	imagePath := filepath.Join(instDir, filenames.Image)
	diskPath := filepath.Join(instDir, filenames.Disk)
	isoPath := filepath.Join(instDir, filenames.ISO)

	formats := []image.Type{typeRAW}
	if isMacOS26OrHigher() {
		formats = append(formats, typeASIF)
	}

	for _, format := range formats {
		writeMinimalISO(t, imagePath)
		imageHashBefore := sha256File(t, imagePath)

		assert.NilError(t, EnsureDisk(t.Context(), instDir, "2MiB", format))

		// image should have been renamed to iso
		assert.Assert(t, !osutil.FileExists(imagePath))
		assert.Equal(t, imageHashBefore, sha256File(t, isoPath))
		isISO, err := iso9660util.IsISO9660(isoPath)
		assert.NilError(t, err)
		assert.Assert(t, isISO)

		// disk should be a real file (empty data disk)
		checkDisk(t, diskPath, format)

		assert.NilError(t, os.Remove(diskPath))
		assert.NilError(t, os.Remove(isoPath))
	}
}

func TestEnsureDisk_WithNonISOImage(t *testing.T) {
	instDir := t.TempDir()
	imagePath := filepath.Join(instDir, filenames.Image)
	diskPath := filepath.Join(instDir, filenames.Disk)

	formats := []image.Type{typeRAW}
	if isMacOS26OrHigher() {
		formats = append(formats, typeASIF)
	}

	for _, format := range formats {
		writeNonISO(t, imagePath)

		assert.NilError(t, EnsureDisk(t.Context(), instDir, "2MiB", format))

		// image should have been consumed
		assert.Assert(t, !osutil.FileExists(imagePath))

		// disk should be the converted image
		checkDisk(t, diskPath, format)

		assert.NilError(t, os.Remove(diskPath))
	}
}

func TestEnsureDisk_ExistingDisk(t *testing.T) {
	instDir := t.TempDir()
	imagePath := filepath.Join(instDir, filenames.Image)
	diskPath := filepath.Join(instDir, filenames.Disk)

	writeNonISO(t, imagePath)

	formats := []image.Type{typeRAW}
	if isMacOS26OrHigher() {
		formats = append(formats, typeASIF)
	}

	for _, format := range formats {
		assert.NilError(t, os.WriteFile(diskPath, []byte("preexisting"), 0o644))
		origHash := sha256File(t, diskPath)
		assert.NilError(t, EnsureDisk(t.Context(), instDir, "2MiB", format))
		assert.Equal(t, sha256File(t, diskPath), origHash)
		assert.NilError(t, os.Remove(diskPath))
	}
}

func TestMigrateDiskLayout_LegacyDiffDisk(t *testing.T) {
	instDir := t.TempDir()
	diffDiskPath := filepath.Join(instDir, filenames.DiffDiskLegacy)

	assert.NilError(t, os.WriteFile(diffDiskPath, []byte("legacy-disk"), 0o644))
	origHash := sha256File(t, diffDiskPath)

	assert.NilError(t, MigrateDiskLayout(instDir))

	// disk should be a symlink to diffdisk
	diskPath := filepath.Join(instDir, filenames.Disk)
	assertSymlink(t, diskPath, filenames.DiffDiskLegacy)
	assert.Equal(t, sha256File(t, diskPath), origHash)

	// diffdisk should still exist (untouched)
	assert.Assert(t, osutil.FileExists(diffDiskPath))
}

func TestMigrateDiskLayout_LegacyISOBaseDisk(t *testing.T) {
	instDir := t.TempDir()
	baseDiskPath := filepath.Join(instDir, filenames.BaseDiskLegacy)
	diffDiskPath := filepath.Join(instDir, filenames.DiffDiskLegacy)

	writeMinimalISO(t, baseDiskPath)
	baseHash := sha256File(t, baseDiskPath)
	assert.NilError(t, os.WriteFile(diffDiskPath, []byte("legacy-disk"), 0o644))
	diffHash := sha256File(t, diffDiskPath)

	assert.NilError(t, MigrateDiskLayout(instDir))

	// disk should be a symlink to diffdisk
	diskPath := filepath.Join(instDir, filenames.Disk)
	assertSymlink(t, diskPath, filenames.DiffDiskLegacy)
	assert.Equal(t, sha256File(t, diskPath), diffHash)

	// iso should be a symlink to basedisk
	isoPath := filepath.Join(instDir, filenames.ISO)
	assertSymlink(t, isoPath, filenames.BaseDiskLegacy)
	assert.Equal(t, sha256File(t, isoPath), baseHash)

	// original files should still exist
	assert.Assert(t, osutil.FileExists(diffDiskPath))
	assert.Assert(t, osutil.FileExists(baseDiskPath))
}

func TestMigrateDiskLayout_LegacyNonISOBaseDisk(t *testing.T) {
	instDir := t.TempDir()
	baseDiskPath := filepath.Join(instDir, filenames.BaseDiskLegacy)
	diffDiskPath := filepath.Join(instDir, filenames.DiffDiskLegacy)

	writeNonISO(t, baseDiskPath)
	baseHash := sha256File(t, baseDiskPath)
	assert.NilError(t, os.WriteFile(diffDiskPath, []byte("legacy-disk"), 0o644))

	assert.NilError(t, MigrateDiskLayout(instDir))

	// disk should be a symlink to diffdisk
	diskPath := filepath.Join(instDir, filenames.Disk)
	assertSymlink(t, diskPath, filenames.DiffDiskLegacy)

	// non-ISO basedisk should remain unchanged (qcow2 backing file)
	assert.Equal(t, sha256File(t, baseDiskPath), baseHash)

	// no iso symlink should be created
	isoPath := filepath.Join(instDir, filenames.ISO)
	_, err := os.Lstat(isoPath)
	assert.Assert(t, os.IsNotExist(err))
}

func TestMigrateDiskLayout_AlreadyMigrated(t *testing.T) {
	instDir := t.TempDir()
	diskPath := filepath.Join(instDir, filenames.Disk)

	assert.NilError(t, os.WriteFile(diskPath, []byte("current-disk"), 0o644))
	origHash := sha256File(t, diskPath)

	assert.NilError(t, MigrateDiskLayout(instDir))

	// disk should be unchanged
	assert.Equal(t, sha256File(t, diskPath), origHash)
}

func TestEnsureDiskSymlink(t *testing.T) {
	t.Run("empty disk path is a NOP", func(t *testing.T) {
		instDir := t.TempDir()

		assert.NilError(t, EnsureDiskSymlink(instDir, ""))

		_, err := os.Lstat(filepath.Join(instDir, filenames.Disk))
		assert.Assert(t, os.IsNotExist(err), "no disk symlink must be created")
	})

	t.Run("creates the symlink before the disk exists", func(t *testing.T) {
		instDir := t.TempDir()
		diskPath := filepath.Join(t.TempDir(), "nested", "disks", "custom.img")

		assert.NilError(t, EnsureDiskSymlink(instDir, diskPath))

		realDiskPath, err := filepath.Abs(diskPath)
		assert.NilError(t, err)
		assertSymlink(t, filepath.Join(instDir, filenames.Disk), realDiskPath)

		fi, err := os.Stat(filepath.Dir(diskPath))
		assert.NilError(t, err)
		assert.Assert(t, fi.IsDir(), "%q must be created as a directory", filepath.Dir(diskPath))

		// The disk file itself is created later, through the symlink.
		_, err = os.Lstat(realDiskPath)
		assert.Assert(t, os.IsNotExist(err), "the disk file must not be created here")
	})

	t.Run("is idempotent", func(t *testing.T) {
		instDir := t.TempDir()
		diskPath := filepath.Join(t.TempDir(), "custom.img")

		assert.NilError(t, EnsureDiskSymlink(instDir, diskPath))
		assert.NilError(t, EnsureDiskSymlink(instDir, diskPath))

		realDiskPath, err := filepath.Abs(diskPath)
		assert.NilError(t, err)
		assertSymlink(t, filepath.Join(instDir, filenames.Disk), realDiskPath)
	})

	t.Run("rejects a disk path inside of the instance directory", func(t *testing.T) {
		instDir := t.TempDir()

		err := EnsureDiskSymlink(instDir, filepath.Join(instDir, "disks", "custom.img"))

		assert.ErrorContains(t, err, "must be outside of the instance directory")
	})

	t.Run("rejects a disk path that is a directory", func(t *testing.T) {
		instDir := t.TempDir()

		err := EnsureDiskSymlink(instDir, t.TempDir())

		assert.ErrorContains(t, err, "is a directory")
	})

	t.Run("rejects an existing disk that is not a symlink", func(t *testing.T) {
		instDir := t.TempDir()
		assert.NilError(t, os.WriteFile(filepath.Join(instDir, filenames.Disk), []byte("disk"), 0o600))

		err := EnsureDiskSymlink(instDir, filepath.Join(t.TempDir(), "custom.img"))

		assert.ErrorContains(t, err, "already exists and is not a symlink")
	})

	t.Run("accepts an existing relative symlink to the same disk", func(t *testing.T) {
		instDir := filepath.Join(t.TempDir(), "instance")
		assert.NilError(t, os.MkdirAll(instDir, 0o700))
		diskPath := filepath.Join(filepath.Dir(instDir), "custom.img")
		relTarget, err := filepath.Rel(instDir, diskPath)
		assert.NilError(t, err)
		assert.NilError(t, os.Symlink(relTarget, filepath.Join(instDir, filenames.Disk)))

		assert.NilError(t, EnsureDiskSymlink(instDir, diskPath))

		// The symlink must be kept as-is.
		target, err := os.Readlink(filepath.Join(instDir, filenames.Disk))
		assert.NilError(t, err)
		assert.Equal(t, target, relTarget)
	})

	t.Run("rejects a symlink to another disk", func(t *testing.T) {
		instDir := t.TempDir()
		otherDiskPath := filepath.Join(t.TempDir(), "other.img")
		assert.NilError(t, os.Symlink(otherDiskPath, filepath.Join(instDir, filenames.Disk)))

		err := EnsureDiskSymlink(instDir, filepath.Join(t.TempDir(), "custom.img"))

		assert.ErrorContains(t, err, "already points to")
	})

	t.Run("refuses to reuse an existing disk file", func(t *testing.T) {
		instDir := t.TempDir()
		diskPath := filepath.Join(t.TempDir(), "custom.img")
		assert.NilError(t, os.WriteFile(diskPath, []byte("disk"), 0o600))

		err := EnsureDiskSymlink(instDir, diskPath)

		assert.ErrorContains(t, err, "already exists")
	})
}

// EnsureDisk has to follow the symlink created by `--disk-path`, so that the
// image is moved next to the relocated disk and the symlink is kept.
func TestEnsureDisk_WithSymlinkedDisk(t *testing.T) {
	instDir := t.TempDir()
	imagePath := filepath.Join(instDir, filenames.Image)
	diskPath := filepath.Join(instDir, filenames.Disk)

	writeNonISO(t, imagePath)
	imageHash := sha256File(t, imagePath)

	realDiskPath := filepath.Join(t.TempDir(), "custom.img")
	assert.NilError(t, os.Symlink(realDiskPath, diskPath))

	assert.NilError(t, EnsureDisk(t.Context(), instDir, "2MiB", typeRAW))

	assert.Assert(t, osutil.IsSymlink(diskPath), "%q must still be a symlink", diskPath)
	assert.Equal(t, sha256File(t, realDiskPath), imageHash)
	assert.Assert(t, !osutil.FileExists(imagePath), "the image must be moved to the symlink target")
}
