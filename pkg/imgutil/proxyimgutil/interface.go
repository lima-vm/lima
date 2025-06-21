// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package proxyimgutil

import (
	"os"

	imgutil "github.com/lima-vm/lima/pkg/imgutil/qemuimgutil"
)

// Interface defines the common operations for disk image utilities.
type Interface interface {
	// CreateDisk creates a new disk image with the specified size.
	CreateDisk(disk string, size int) error

	// ResizeDisk resizes an existing disk image to the specified size.
	ResizeDisk(disk string, size int) error

	// ConvertToRaw converts a disk image to raw format.
	ConvertToRaw(source, dest string, size *int64, allowSourceWithBackingFile bool) error

	// MakeSparse makes a file sparse, starting from the specified offset.
	MakeSparse(f *os.File, offset int64) error
}

// InfoProvider defines the interface for obtaining disk image information.
type InfoProvider interface {
	// GetInfo retrieves information about a disk image
	GetInfo(path string) (*imgutil.Info, error)

	// AcceptableAsBasedisk checks if a disk image is acceptable as a base disk
	AcceptableAsBasedisk(*imgutil.Info) error
}
