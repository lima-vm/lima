// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package nativeimgutil

import (
	"os"

	imgutil "github.com/lima-vm/lima/pkg/imgutil/qemuimgutil"
)

// NativeImageUtil is the native implementation of the proxyimgutil Interface.
type NativeImageUtil struct{}

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

// NativeInfoProvider is the native implementation of the proxyimgutil.InfoProvider.
type NativeInfoProvider struct{}

// NewNativeInfoProvider returns a new NativeInfoProvider instance.
func NewNativeInfoProvider() *NativeInfoProvider {
	return &NativeInfoProvider{}
}

// GetInfo retrieves information about a disk image
// This is a stub implementation as the native package doesn't provide this functionality.
func (n *NativeInfoProvider) GetInfo(_ string) (*imgutil.Info, error) {
	return nil, nil
}

// AcceptableAsBasedisk checks if a disk image is acceptable as a base disk
// This is a stub implementation as the native package doesn't provide this functionality.
func (n *NativeInfoProvider) AcceptableAsBasedisk(_ *imgutil.Info) error {
	return nil
}
