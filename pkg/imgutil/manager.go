// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package imgutil

import (
	"os"
)

// ImageDiskManager defines the common operations for disk image utilities.
type ImageDiskManager interface {
	// CreateDisk creates a new disk image with the specified size.
	CreateDisk(disk string, size int64) error

	// ResizeDisk resizes an existing disk image to the specified size.
	ResizeDisk(disk string, size int64) error

	// ConvertToRaw converts a disk image to raw format.
	ConvertToRaw(source, dest string, size *int64, allowSourceWithBackingFile bool) error

	// MakeSparse makes a file sparse, starting from the specified offset.
	MakeSparse(f *os.File, offset int64) error
}
