// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package fsutil

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
	"gotest.tools/v3/assert"
)

// skipUnlessBtrfs skips the test when dir is not on btrfs, where there is nothing to observe.
func skipUnlessBtrfs(t *testing.T, dir string) {
	t.Helper()
	ok, err := onBtrfs(dir)
	assert.NilError(t, err)
	if !ok {
		t.Skip("TMPDIR is not on btrfs")
	}
}

// newFileHasNoCOW reports whether a file created in dir inherits the No_COW attribute.
func newFileHasNoCOW(t *testing.T, dir string) bool {
	t.Helper()
	f, err := os.CreateTemp(dir, "disk")
	assert.NilError(t, err)
	assert.NilError(t, f.Close())
	noCOW, err := hasNoCOW(f.Name())
	assert.NilError(t, err)
	return noCOW
}

// captureLog collects what logrus logs until the test ends.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	out := logrus.StandardLogger().Out
	logrus.SetOutput(&buf)
	t.Cleanup(func() { logrus.SetOutput(out) })
	return &buf
}

// TestDisableCopyOnWrite covers the btrfs case only when TMPDIR is on btrfs.
func TestDisableCopyOnWrite(t *testing.T) {
	t.Setenv("LIMA_BTRFS_NOCOW", "")
	dir := t.TempDir()
	assert.NilError(t, setNoCOW(dir, false)) // whatever TMPDIR is
	assert.NilError(t, DisableCopyOnWrite(dir))
	skipUnlessBtrfs(t, dir)
	assert.Assert(t, newFileHasNoCOW(t, dir), "a file created in dir should inherit No_COW")
}

// TestDisableCopyOnWriteEnv covers the btrfs case only when TMPDIR is on btrfs.
func TestDisableCopyOnWriteEnv(t *testing.T) {
	for value, noCOW := range map[string]bool{
		"false": false,
		"0":     false,
		"true":  true,
		"bogus": true, // an invalid value is ignored
	} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("LIMA_BTRFS_NOCOW", value)
			dir := t.TempDir()
			assert.NilError(t, setNoCOW(dir, false)) // whatever TMPDIR is
			log := captureLog(t)
			assert.NilError(t, DisableCopyOnWrite(dir))
			assert.Equal(t, strings.Contains(log.String(), "invalid LIMA_BTRFS_NOCOW value"), value == "bogus")
			skipUnlessBtrfs(t, dir)
			assert.Equal(t, newFileHasNoCOW(t, dir), noCOW)
		})
	}
}

// TestMatchCopyOnWrite covers the btrfs case only when TMPDIR is on btrfs.
func TestMatchCopyOnWrite(t *testing.T) {
	cow, noCOW, dst := t.TempDir(), t.TempDir(), t.TempDir()
	assert.NilError(t, MatchCopyOnWrite(dst, cow))
	skipUnlessBtrfs(t, dst)
	assert.NilError(t, setNoCOW(cow, false)) // whatever TMPDIR is
	assert.NilError(t, setNoCOW(noCOW, true))
	// The opt-out is for new instances; a copy of an existing one still follows it.
	t.Setenv("LIMA_BTRFS_NOCOW", "false")

	assert.NilError(t, MatchCopyOnWrite(dst, noCOW))
	assert.Assert(t, newFileHasNoCOW(t, dst), "dst should follow a No_COW src")
	assert.NilError(t, MatchCopyOnWrite(dst, cow))
	assert.Assert(t, !newFileHasNoCOW(t, dst), "dst should follow a copy-on-write src")

	// A src on another filesystem (procfs here) gives dst the default, opt-out included.
	assert.NilError(t, MatchCopyOnWrite(dst, "/proc"))
	assert.Assert(t, !newFileHasNoCOW(t, dst), "LIMA_BTRFS_NOCOW=false should keep dst as it is")
	t.Setenv("LIMA_BTRFS_NOCOW", "")
	assert.NilError(t, MatchCopyOnWrite(dst, "/proc"))
	assert.Assert(t, newFileHasNoCOW(t, dst), "dst should get the default when src is not on btrfs")
}

// TestDisableCopyOnWriteCompressed covers the btrfs case only when TMPDIR is on btrfs.
func TestDisableCopyOnWriteCompressed(t *testing.T) {
	t.Setenv("LIMA_BTRFS_NOCOW", "")
	dir := t.TempDir()
	skipUnlessBtrfs(t, dir)
	assert.NilError(t, setNoCOW(dir, false)) // whatever TMPDIR is
	f, err := os.Open(dir)
	assert.NilError(t, err)
	defer f.Close()
	flags, err := unix.IoctlGetUint32(int(f.Fd()), unix.FS_IOC_GETFLAGS)
	assert.NilError(t, err)
	assert.NilError(t, unix.IoctlSetPointerInt(int(f.Fd()), unix.FS_IOC_SETFLAGS, int(flags|fsComprFL))) // chattr +c

	assert.NilError(t, DisableCopyOnWrite(dir), "a compressed directory should be skipped")
	assert.Assert(t, !newFileHasNoCOW(t, dir))
}

// TestWarnIfFullCopy covers the btrfs case only when TMPDIR is on btrfs.
func TestWarnIfFullCopy(t *testing.T) {
	cow, noCOW := t.TempDir(), t.TempDir()
	skipUnlessBtrfs(t, cow)
	assert.NilError(t, setNoCOW(cow, false)) // whatever TMPDIR is
	assert.NilError(t, setNoCOW(noCOW, true))
	src := filepath.Join(cow, "src")
	assert.NilError(t, os.WriteFile(src, []byte("disk"), 0o600))

	log := captureLog(t)
	WarnIfFullCopy(filepath.Join(cow, "dst"), src)
	assert.Equal(t, log.String(), "", "a copy within copy-on-write directories is a reflink")
	WarnIfFullCopy(filepath.Join(noCOW, "dst"), src)
	assert.Assert(t, strings.Contains(log.String(), "in full"), "a copy into a No_COW directory is a full copy")
}
