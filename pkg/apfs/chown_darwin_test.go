// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package apfs

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"gotest.tools/v3/assert"
)

// TestChownIntegration creates a real APFS disk image with hdiutil,
// writes a test file, runs Chown on the raw image, and verifies
// ownership via os.Stat after remounting with ownership enabled.
// No root required.
func TestChownIntegration(t *testing.T) {
	tmpDir := t.TempDir()
	imgPath := filepath.Join(tmpDir, "test.dmg")

	// Create a 64MB APFS disk image (GPT + APFS container).
	ctx := t.Context()
	cmd := exec.CommandContext(ctx, "hdiutil", "create",
		"-size", "64m",
		"-fs", "APFS",
		"-volname", "TestVol",
		imgPath,
	)
	out, err := cmd.CombinedOutput()
	assert.NilError(t, err, "hdiutil create failed: %s", out)

	// Attach, write a test file, and detach. Default mount uses
	// noowners, so the on-disk UID will be 99 (nobody).
	mntDir := filepath.Join(tmpDir, "mnt")
	cmd = exec.CommandContext(ctx, "hdiutil", "attach", imgPath, "-mountpoint", mntDir)
	out, err = cmd.CombinedOutput()
	assert.NilError(t, err, "hdiutil attach failed: %s", out)

	assert.NilError(t, os.WriteFile(filepath.Join(mntDir, "testfile.txt"), []byte("hello"), 0o644))

	cmd = exec.CommandContext(ctx, "hdiutil", "detach", mntDir)
	out, err = cmd.CombinedOutput()
	assert.NilError(t, err, "hdiutil detach failed: %s", out)

	// Change ownership to root:wheel via raw APFS modification.
	// hdiutil creates a single volume with role=0 (APFS_VOL_ROLE_NONE).
	assert.NilError(t, Chown(imgPath, 0, 0, 0, "testfile.txt"))

	// Verify filesystem integrity after raw block modification.
	cmd = exec.CommandContext(ctx, "hdiutil", "attach", imgPath, "-nomount")
	out, err = cmd.CombinedOutput()
	assert.NilError(t, err, "hdiutil attach -nomount failed: %s", out)
	dev := strings.Fields(string(out))[0]
	cmd = exec.CommandContext(ctx, "fsck_apfs", "-n", dev)
	out, err = cmd.CombinedOutput()
	assert.NilError(t, err, "fsck_apfs failed: %s", out)
	cmd = exec.CommandContext(ctx, "hdiutil", "detach", dev)
	out, err = cmd.CombinedOutput()
	assert.NilError(t, err, "hdiutil detach failed: %s", out)

	// Re-attach with ownership enabled so os.Stat reflects on-disk UIDs.
	cmd = exec.CommandContext(ctx, "hdiutil", "attach", imgPath,
		"-mountpoint", mntDir, "-owners", "on")
	out, err = cmd.CombinedOutput()
	assert.NilError(t, err, "hdiutil re-attach failed: %s", out)
	defer func() {
		_ = exec.CommandContext(ctx, "hdiutil", "detach", mntDir, "-force").Run()
	}()

	fi, err := os.Stat(filepath.Join(mntDir, "testfile.txt"))
	assert.NilError(t, err)
	stat := fi.Sys().(*syscall.Stat_t)
	assert.Equal(t, stat.Uid, uint32(0), "expected uid=0, got uid=%d", stat.Uid)
	assert.Equal(t, stat.Gid, uint32(0), "expected gid=0, got gid=%d", stat.Gid)
}
