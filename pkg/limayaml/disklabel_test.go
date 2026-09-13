// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package limayaml

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"

	"gotest.tools/v3/assert"
)

func TestDiskFSLabel(t *testing.T) {
	assert.Equal(t, DiskFSLabel("disk1"), "lima-disk1")
	assert.Equal(t, DiskFSLabel(""), "lima-")
}

func TestFSLabelMaxLen(t *testing.T) {
	tests := []struct {
		fsType string
		want   int
	}{
		{"ext2", 16},
		{"ext3", 16},
		{"ext4", 16},
		{"xfs", 12},
		{"btrfs", 255},
		{"swap", 15},
		{"vfat", 0},
		{"unknown-fs", 0},
		{"", 0},
	}
	for _, tt := range tests {
		t.Run(tt.fsType, func(t *testing.T) {
			assert.Equal(t, FSLabelMaxLen(tt.fsType), tt.want)
		})
	}
}

// TestFSLabelMaxLenMatchesBootScript guards the invariant documented on
// FSLabelMaxLen: the guest clamps the label using its own copy of these limits,
// so the two must not drift apart.
func TestFSLabelMaxLenMatchesBootScript(t *testing.T) {
	script := filepath.Join("..", "cidata", "cidata.TEMPLATE.d", "boot.Linux", "05-lima-disks.sh")
	b, err := os.ReadFile(script)
	assert.NilError(t, err, "%s should exist; update this test if the boot script moved", script)

	// Matches the case arms of the form `xfs) FSLABEL_MAX=12 ;;`.
	re := regexp.MustCompile(`(?m)^\s*([a-z0-9]+)\)\s*FSLABEL_MAX=(\d+)\s*;;`)
	matches := re.FindAllStringSubmatch(string(b), -1)
	assert.Assert(t, len(matches) > 0, "no FSLABEL_MAX case arms found in %s", script)

	for _, m := range matches {
		fsType, want := m[1], m[2]
		got := strconv.Itoa(FSLabelMaxLen(fsType))
		assert.Equal(t, got, want, "FSLabelMaxLen(%q) disagrees with %s", fsType, script)
	}
}
