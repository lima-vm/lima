// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package mountutil

import (
	"testing"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/limatype"
)

func ptr[T any](v T) *T { return &v }

func TestFSType(t *testing.T) {
	cases := []struct {
		mt   limatype.MountType
		os   limatype.OS
		want string
	}{
		{limatype.REVSSHFS, limatype.LINUX, "sshfs"},
		{limatype.NINEP, limatype.LINUX, "9p"},
		{limatype.NINEP, limatype.FREEBSD, "p9fs"},
		{limatype.VIRTIOFS, limatype.LINUX, "virtiofs"},
	}
	for _, c := range cases {
		if got := FSType(c.mt, c.os); got != c.want {
			t.Errorf("FSType(%q,%q)=%q want %q", c.mt, c.os, got, c.want)
		}
	}
}

func TestMountOptions(t *testing.T) {
	nineP := limatype.NineP{
		ProtocolVersion: ptr("9p2000.L"),
		Msize:           ptr("8MiB"),
		Cache:           ptr("fscache"),
	}
	cases := []struct {
		name string
		m    limatype.Mount
		mt   limatype.MountType
		os   limatype.OS
		want string
	}{
		{
			"9p-writable-linux",
			limatype.Mount{Location: "/h", MountPoint: ptr("/g"), Writable: ptr(true), NineP: nineP},
			limatype.NINEP, limatype.LINUX,
			"rw,trans=virtio,version=9p2000.L,msize=8388608,cache=fscache,nofail",
		},
		{
			"virtiofs-ro-linux",
			limatype.Mount{Location: "/h", MountPoint: ptr("/g"), Writable: ptr(false)},
			limatype.VIRTIOFS, limatype.LINUX,
			"ro,nofail",
		},
		{
			"virtiofs-rw-linux",
			limatype.Mount{Location: "/h", MountPoint: ptr("/g"), Writable: ptr(true)},
			limatype.VIRTIOFS, limatype.LINUX,
			"rw,nofail",
		},
		{
			"sshfs-linux",
			limatype.Mount{Location: "/h", MountPoint: ptr("/g"), Writable: ptr(true)},
			limatype.REVSSHFS, limatype.LINUX,
			"defaults",
		},
	}
	for _, c := range cases {
		got, err := MountOptions(&c.m, c.mt, c.os)
		assert.NilError(t, err, c.name)
		if got != c.want {
			t.Errorf("%s: got %q want %q", c.name, got, c.want)
		}
	}
}

func TestMountOptions9pMissingOpts(t *testing.T) {
	m := limatype.Mount{Location: "/h", MountPoint: ptr("/g"), Writable: ptr(true)}
	if _, err := MountOptions(&m, limatype.NINEP, limatype.LINUX); err == nil {
		t.Error("expected error when 9p options are unset")
	}
}
