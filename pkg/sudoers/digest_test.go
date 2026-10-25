// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package sudoers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

const emptyFileDigest = "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

func TestIsPrivilegedHelper(t *testing.T) {
	assert.Assert(t, isPrivilegedHelper("/usr/local/libexec/lima/privileged/lima-privileged-net"))
	assert.Assert(t, isPrivilegedHelper("/opt/homebrew/lib/lima/privileged/lima-privileged-block-device"))
	assert.Assert(t, !isPrivilegedHelper("/usr/local/libexec/lima/limactl-mcp"))
	assert.Assert(t, !isPrivilegedHelper("/opt/socket_vmnet/bin/socket_vmnet"))
	assert.Assert(t, !isPrivilegedHelper("/usr/bin/pkill"))
	assert.Assert(t, !isPrivilegedHelper("/opt/privileged/lima-privileged-new"))
}

func TestNOPASSWDPinsPrivilegedHelpers(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "libexec", "lima", "privileged")
	assert.NilError(t, os.MkdirAll(dir, 0o755))
	// Any helper in the privileged directory is pinned, without a per-helper opt-in.
	binary := filepath.Join(dir, "lima-privileged-new")
	assert.NilError(t, os.WriteFile(binary, nil, 0o755))

	got, err := NOPASSWD("alice", "root", "wheel", binary+" start", binary+" tap", "/usr/bin/pkill -F x.pid")
	assert.NilError(t, err)
	assert.Equal(t, got, "alice ALL=(root:wheel) NOPASSWD:NOSETENV: \\\n"+
		"    "+emptyFileDigest+" "+binary+" start, \\\n"+
		"    "+emptyFileDigest+" "+binary+" tap, \\\n"+
		"    /usr/bin/pkill -F x.pid\n")

	_, err = NOPASSWD("alice", "root", "wheel", filepath.Join(dir, "missing")+" start")
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestUnpinAndContainsFragment(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "libexec", "lima", "privileged")
	assert.NilError(t, os.MkdirAll(dir, 0o755))
	binary := filepath.Join(dir, "lima-privileged-new")
	assert.NilError(t, os.WriteFile(binary, nil, 0o755))
	pinned, err := NOPASSWD("alice", "root", "wheel", binary+" start", "/usr/bin/pkill -F x.pid")
	assert.NilError(t, err)

	unpinned := Unpin(pinned)
	assert.Equal(t, unpinned, render("alice", "root", "wheel", binary+" start", "/usr/bin/pkill -F x.pid"))
	assert.Equal(t, Unpin(unpinned), unpinned)

	assert.Assert(t, ContainsFragment(pinned, pinned))
	assert.Assert(t, ContainsFragment(unpinned, pinned))
	// Only ContainsFragment accepts the unpinned form; the strict check does not.
	assert.Assert(t, !ContainsActiveFragment(unpinned, pinned))
	// A pinned file with a stale digest is still out of sync.
	stale := strings.ReplaceAll(pinned, emptyFileDigest, "sha256:"+strings.Repeat("0", 64))
	assert.Assert(t, !ContainsFragment(stale, pinned))
}
