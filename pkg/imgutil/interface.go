// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package imgutil

import (
	"encoding/json"
	"os"
)

// ImageDiskManager defines the common operations for disk image utilities.
type ImageDiskManager interface {
	// CreateDisk creates a new disk image with the specified size.
	CreateDisk(disk string, size int) error

	// ResizeDisk resizes an existing disk image to the specified size.
	ResizeDisk(disk string, size int) error

	// ConvertToRaw converts a disk image to raw format.
	ConvertToRaw(source, dest string, size *int64, allowSourceWithBackingFile bool) error

	// MakeSparse makes a file sparse, starting from the specified offset.
	MakeSparse(f *os.File, offset int64) error
}

type InfoChild struct {
	Name string `json:"name,omitempty"` // since QEMU 8.0
	Info Info   `json:"info,omitempty"` // since QEMU 8.0
}

type InfoFormatSpecific struct {
	Type string          `json:"type,omitempty"` // since QEMU 1.7
	Data json.RawMessage `json:"data,omitempty"` // since QEMU 1.7
}

func (sp *InfoFormatSpecific) Qcow2() *InfoFormatSpecificDataQcow2 {
	if sp.Type != "qcow2" {
		return nil
	}
	var x InfoFormatSpecificDataQcow2
	if err := json.Unmarshal(sp.Data, &x); err != nil {
		panic(err)
	}
	return &x
}

func (sp *InfoFormatSpecific) Vmdk() *InfoFormatSpecificDataVmdk {
	if sp.Type != "vmdk" {
		return nil
	}
	var x InfoFormatSpecificDataVmdk
	if err := json.Unmarshal(sp.Data, &x); err != nil {
		panic(err)
	}
	return &x
}

type InfoFormatSpecificDataQcow2 struct {
	Compat          string `json:"compat,omitempty"`           // since QEMU 1.7
	LazyRefcounts   bool   `json:"lazy-refcounts,omitempty"`   // since QEMU 1.7
	Corrupt         bool   `json:"corrupt,omitempty"`          // since QEMU 2.2
	RefcountBits    int    `json:"refcount-bits,omitempty"`    // since QEMU 2.3
	CompressionType string `json:"compression-type,omitempty"` // since QEMU 5.1
	ExtendedL2      bool   `json:"extended-l2,omitempty"`      // since QEMU 5.2
}

type InfoFormatSpecificDataVmdk struct {
	CreateType string                             `json:"create-type,omitempty"` // since QEMU 1.7
	CID        int                                `json:"cid,omitempty"`         // since QEMU 1.7
	ParentCID  int                                `json:"parent-cid,omitempty"`  // since QEMU 1.7
	Extents    []InfoFormatSpecificDataVmdkExtent `json:"extents,omitempty"`     // since QEMU 1.7
}

type InfoFormatSpecificDataVmdkExtent struct {
	Filename    string `json:"filename,omitempty"`     // since QEMU 1.7
	Format      string `json:"format,omitempty"`       // since QEMU 1.7
	VSize       int64  `json:"virtual-size,omitempty"` // since QEMU 1.7
	ClusterSize int    `json:"cluster-size,omitempty"` // since QEMU 1.7
}

// Info corresponds to the output of `qemu-img info --output=json FILE`.
type Info struct {
	Filename              string              `json:"filename,omitempty"`                // since QEMU 1.3
	Format                string              `json:"format,omitempty"`                  // since QEMU 1.3
	VSize                 int64               `json:"virtual-size,omitempty"`            // since QEMU 1.3
	ActualSize            int64               `json:"actual-size,omitempty"`             // since QEMU 1.3
	DirtyFlag             bool                `json:"dirty-flag,omitempty"`              // since QEMU 5.2
	ClusterSize           int                 `json:"cluster-size,omitempty"`            // since QEMU 1.3
	BackingFilename       string              `json:"backing-filename,omitempty"`        // since QEMU 1.3
	FullBackingFilename   string              `json:"full-backing-filename,omitempty"`   // since QEMU 1.3
	BackingFilenameFormat string              `json:"backing-filename-format,omitempty"` // since QEMU 1.3
	FormatSpecific        *InfoFormatSpecific `json:"format-specific,omitempty"`         // since QEMU 1.7
	Children              []InfoChild         `json:"children,omitempty"`                // since QEMU 8.0
}
