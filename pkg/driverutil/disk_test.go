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

func checkDisk(t *testing.T, diff string, expectedType image.Type) {
	t.Helper()
	fi, err := os.Stat(diff)
	assert.NilError(t, err)
	assert.Assert(t, fi.Size() > 0)
	assert.Equal(t, detectImageType(t, diff), expectedType)
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

func TestEnsureDisk_WithISOBaseImage(t *testing.T) {
	instDir := t.TempDir()
	base := filepath.Join(instDir, filenames.BaseDisk)
	diff := filepath.Join(instDir, filenames.DiffDisk)

	writeMinimalISO(t, base)
	isISO, err := iso9660util.IsISO9660(base)
	assert.NilError(t, err)
	assert.Assert(t, isISO)
	baseHashBefore := sha256File(t, base)

	formats := []image.Type{typeRAW}
	if isMacOS26OrHigher() {
		formats = append(formats, typeASIF)
	}

	for _, format := range formats {
		assert.NilError(t, EnsureDisk(t.Context(), instDir, "2MiB", format))
		isISO, err = iso9660util.IsISO9660(base)
		assert.NilError(t, err)
		assert.Assert(t, isISO)
		assert.Equal(t, baseHashBefore, sha256File(t, base))
		checkDisk(t, diff, format)
		assert.NilError(t, os.Remove(diff))
	}
}

func TestEnsureDisk_WithNonISOBaseImage(t *testing.T) {
	instDir := t.TempDir()
	base := filepath.Join(instDir, filenames.BaseDisk)
	diff := filepath.Join(instDir, filenames.DiffDisk)

	writeNonISO(t, base)
	isISO, err := iso9660util.IsISO9660(base)
	assert.NilError(t, err)
	assert.Assert(t, !isISO)

	formats := []image.Type{typeRAW}
	if isMacOS26OrHigher() {
		formats = append(formats, typeASIF)
	}

	for _, format := range formats {
		assert.NilError(t, EnsureDisk(t.Context(), instDir, "2MiB", format))
		checkDisk(t, diff, format)
		assert.NilError(t, os.Remove(diff))
	}
}

func TestEnsureDisk_ExistingDiffDisk(t *testing.T) {
	instDir := t.TempDir()
	base := filepath.Join(instDir, filenames.BaseDisk)
	diff := filepath.Join(instDir, filenames.DiffDisk)

	writeNonISO(t, base)

	formats := []image.Type{typeRAW}
	if isMacOS26OrHigher() {
		formats = append(formats, typeASIF)
	}

	for _, format := range formats {
		assert.NilError(t, os.WriteFile(diff, []byte("preexisting"), 0o644))
		origHash := sha256File(t, diff)
		assert.NilError(t, EnsureDisk(t.Context(), instDir, "2MiB", format))
		assert.Equal(t, sha256File(t, diff), origHash)
		assert.NilError(t, os.Remove(diff))
	}
}
