// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package proxyimgutil

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/lima-vm/go-qcow2reader/image/qcow2"
	"github.com/lima-vm/go-qcow2reader/image/raw"
	"gotest.tools/v3/assert"
)

// fakeQemuImg puts a `qemu-img` shell script in front of PATH that records its
// arguments to a file instead of creating the image.
func fakeQemuImg(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake qemu-img is a POSIX shell script")
	}
	dir := t.TempDir()
	logPath := filepath.Join(dir, "qemu-img.args")
	script := fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' \"$@\" > %q\n", logPath)
	assert.NilError(t, os.WriteFile(filepath.Join(dir, "qemu-img"), []byte(script), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return logPath
}

func TestCreateDiskPassesRequestedFormat(t *testing.T) {
	ctx := t.Context()
	for _, format := range []string{"raw", "qcow2"} {
		t.Run(format, func(t *testing.T) {
			logPath := fakeQemuImg(t)
			disk := filepath.Join(t.TempDir(), "disk")
			diskFormat := raw.Type
			if format == "qcow2" {
				diskFormat = qcow2.Type
			}
			util := NewDiskUtil(ctx)
			assert.NilError(t, util.CreateDisk(ctx, disk, 1024, diskFormat))

			args, err := os.ReadFile(logPath)
			assert.NilError(t, err)
			assert.Equal(t, string(args),
				"create\n-f\n"+format+"\n"+disk+"\n1024\n")
		})
	}
}

func TestCreateDiskDefaultFormat(t *testing.T) {
	ctx := t.Context()
	logPath := fakeQemuImg(t)
	disk := filepath.Join(t.TempDir(), "disk")
	util := NewDiskUtil(ctx)
	assert.NilError(t, util.CreateDisk(ctx, disk, 1024, ""))

	args, err := os.ReadFile(logPath)
	assert.NilError(t, err)
	assert.Equal(t, string(args), "create\n-f\nqcow2\n"+disk+"\n1024\n")
}

// Without qemu-img a raw disk still works, but a qcow2 disk has to fail instead
// of silently producing a raw file.
func TestCreateDiskWithoutQemuImg(t *testing.T) {
	ctx := t.Context()
	t.Setenv("PATH", t.TempDir())
	util := NewDiskUtil(ctx)

	rawDisk := filepath.Join(t.TempDir(), "disk.raw")
	assert.NilError(t, util.CreateDisk(ctx, rawDisk, 1024, raw.Type))
	assert.NilError(t, os.Remove(rawDisk))

	err := util.CreateDisk(ctx, filepath.Join(t.TempDir(), "disk.qcow2"), 1024, qcow2.Type)
	assert.ErrorContains(t, err, "without qemu-img")
}
