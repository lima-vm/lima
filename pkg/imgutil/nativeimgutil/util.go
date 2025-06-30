// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package nativeimgutil

import (
	"os"
)

// NewNativeImageUtil returns a new NativeImageUtil instance.
func NewNativeImageUtil() *NativeImageUtil {
	return &NativeImageUtil{}
}

// CreateDisk creates a new raw disk image with the specified size.
func (n *NativeImageUtil) CreateDisk(disk string, size int) error {
	return createRawDisk(disk, size)
}

// ResizeDisk resizes an existing raw disk image to the specified size.
func (n *NativeImageUtil) ResizeDisk(disk string, size int) error {
	return resizeRawDisk(disk, size)
}

// ConvertToRaw converts a disk image to raw format.
func (n *NativeImageUtil) ConvertToRaw(source, dest string, size *int64, allowSourceWithBackingFile bool) error {
	return convertToRaw(source, dest, size, allowSourceWithBackingFile)
}

func (n *NativeImageUtil) MakeSparse(f *os.File, offset int64) error {
	return makeSparse(f, offset)
}
