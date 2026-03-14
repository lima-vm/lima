// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package qemuimgutil

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestParseInfo(t *testing.T) {
	t.Run("qcow2", func(t *testing.T) {
		// qemu-img create -f qcow2 foo.qcow2 4G
		// (QEMU 8.0)
		const s = `{
    "children": [
        {
            "name": "file",
            "info": {
                "children": [
                ],
                "virtual-size": 197120,
                "filename": "foo.qcow2",
                "format": "file",
                "actual-size": 200704,
                "format-specific": {
                    "type": "file",
                    "data": {
                    }
                },
                "dirty-flag": false
            }
        }
    ],
    "virtual-size": 4294967296,
    "filename": "foo.qcow2",
    "cluster-size": 65536,
    "format": "qcow2",
    "actual-size": 200704,
    "format-specific": {
        "type": "qcow2",
        "data": {
            "compat": "1.1",
            "compression-type": "zlib",
            "lazy-refcounts": false,
            "refcount-bits": 16,
            "corrupt": false,
            "extended-l2": false
        }
    },
    "dirty-flag": false
}`

		info, err := parseInfo([]byte(s))
		assert.NilError(t, err)
		assert.Equal(t, 1, len(info.Children))
		assert.Check(t, info.FormatSpecific != nil)
		qcow2 := info.FormatSpecific.Qcow2()
		assert.Check(t, qcow2 != nil)
		assert.Equal(t, qcow2.Compat, "1.1")

		t.Run("diff", func(t *testing.T) {
			// qemu-img create -f qcow2 -F qcow2 -b foo.qcow2 bar.qcow2
			// (QEMU 8.0)
			const s = `{
    "children": [
        {
            "name": "file",
            "info": {
                "children": [
                ],
                "virtual-size": 197120,
                "filename": "bar.qcow2",
                "format": "file",
                "actual-size": 200704,
                "format-specific": {
                    "type": "file",
                    "data": {
                    }
                },
                "dirty-flag": false
            }
        }
    ],
    "backing-filename-format": "qcow2",
    "virtual-size": 4294967296,
    "filename": "bar.qcow2",
    "cluster-size": 65536,
    "format": "qcow2",
    "actual-size": 200704,
    "format-specific": {
        "type": "qcow2",
        "data": {
            "compat": "1.1",
            "compression-type": "zlib",
            "lazy-refcounts": false,
            "refcount-bits": 16,
            "corrupt": false,
            "extended-l2": false
        }
    },
    "full-backing-filename": "foo.qcow2",
    "backing-filename": "foo.qcow2",
    "dirty-flag": false
}`
			info, err := parseInfo([]byte(s))
			assert.NilError(t, err)
			assert.Equal(t, 1, len(info.Children))
			assert.Equal(t, "foo.qcow2", info.BackingFilename)
			assert.Equal(t, "bar.qcow2", info.Filename)
			assert.Check(t, info.FormatSpecific != nil)
			qcow2 := info.FormatSpecific.Qcow2()
			assert.Check(t, qcow2 != nil)
			assert.Equal(t, qcow2.Compat, "1.1")
		})
	})
	t.Run("vmdk", func(t *testing.T) {
		t.Run("twoGbMaxExtentSparse", func(t *testing.T) {
			// qemu-img create -f vmdk foo.vmdk 4G -o subformat=twoGbMaxExtentSparse
			// (QEMU 8.0)
			const s = `{
    "children": [
        {
            "name": "extents.1",
            "info": {
                "children": [
                ],
                "virtual-size": 327680,
                "filename": "foo-s002.vmdk",
                "format": "file",
                "actual-size": 327680,
                "format-specific": {
                    "type": "file",
                    "data": {
                    }
                },
                "dirty-flag": false
            }
        },
        {
            "name": "extents.0",
            "info": {
                "children": [
                ],
                "virtual-size": 327680,
                "filename": "foo-s001.vmdk",
                "format": "file",
                "actual-size": 327680,
                "format-specific": {
                    "type": "file",
                    "data": {
                    }
                },
                "dirty-flag": false
            }
        },
        {
            "name": "file",
            "info": {
                "children": [
                ],
                "virtual-size": 512,
                "filename": "foo.vmdk",
                "format": "file",
                "actual-size": 4096,
                "format-specific": {
                    "type": "file",
                    "data": {
                    }
                },
                "dirty-flag": false
            }
        }
    ],
    "virtual-size": 4294967296,
    "filename": "foo.vmdk",
    "cluster-size": 65536,
    "format": "vmdk",
    "actual-size": 659456,
    "format-specific": {
        "type": "vmdk",
        "data": {
            "cid": 918420663,
            "parent-cid": 4294967295,
            "create-type": "twoGbMaxExtentSparse",
            "extents": [
                {
                    "virtual-size": 2147483648,
                    "filename": "foo-s001.vmdk",
                    "cluster-size": 65536,
                    "format": "SPARSE"
                },
                {
                    "virtual-size": 2147483648,
                    "filename": "foo-s002.vmdk",
                    "cluster-size": 65536,
                    "format": "SPARSE"
                }
            ]
        }
    },
    "dirty-flag": false
}`
			info, err := parseInfo([]byte(s))
			assert.NilError(t, err)
			assert.Equal(t, 3, len(info.Children))
			assert.Equal(t, "foo.vmdk", info.Filename)
			assert.Check(t, info.FormatSpecific != nil)
			vmdk := info.FormatSpecific.Vmdk()
			assert.Check(t, vmdk != nil)
			assert.Equal(t, vmdk.CreateType, "twoGbMaxExtentSparse")
		})
	})
}
