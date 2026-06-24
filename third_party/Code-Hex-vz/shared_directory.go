package vz

/*
#cgo darwin CFLAGS: -mmacosx-version-min=11 -x objective-c -fno-objc-arc
#cgo darwin LDFLAGS: -lobjc -framework Foundation -framework Virtualization
# include "virtualization_11.h"
# include "virtualization_12.h"
# include "virtualization_13.h"
*/
import "C"
import (
	"errors"
	"os"

	"github.com/Code-Hex/vz/v3/internal/objc"
)

// SetVirtioFileSystemDeviceShareAtIndex sets, at runtime, the directory share of the
// directory-sharing device at the given index. The index refers to the position in the
// slice passed to SetDirectorySharingDevicesVirtualMachineConfiguration at VM creation,
// and the device must be a Virtio file system device. This allows changing (or
// populating) the shared host directory of a running VM without restarting it.
//
// It is the runtime counterpart of VirtioFileSystemDeviceConfiguration.SetDirectoryShare.
// Available on macOS 12 and newer.
func (v *VirtualMachine) SetVirtioFileSystemDeviceShareAtIndex(index int, share DirectoryShare) error {
	if err := macOSAvailable(12); err != nil {
		return err
	}
	cerr := C.setDirectorySharingDeviceShareAtIndex(objc.Ptr(v), v.dispatchQueue, C.int(index), objc.Ptr(share))
	if cerr != nil {
		return errors.New(C.GoString(cerr))
	}
	return nil
}

// DirectorySharingDeviceConfiguration for a directory sharing device configuration.
type DirectorySharingDeviceConfiguration interface {
	objc.NSObject

	directorySharingDeviceConfiguration()
}

type baseDirectorySharingDeviceConfiguration struct{}

func (*baseDirectorySharingDeviceConfiguration) directorySharingDeviceConfiguration() {}

var _ DirectorySharingDeviceConfiguration = (*VirtioFileSystemDeviceConfiguration)(nil)

// VirtioFileSystemDeviceConfiguration is a configuration of a Virtio file system device.
//
// see: https://developer.apple.com/documentation/virtualization/vzvirtiofilesystemdeviceconfiguration?language=objc
type VirtioFileSystemDeviceConfiguration struct {
	*pointer

	*baseDirectorySharingDeviceConfiguration
}

// NewVirtioFileSystemDeviceConfiguration create a new VirtioFileSystemDeviceConfiguration.
//
// This is only supported on macOS 12 and newer, error will
// be returned on older versions.
func NewVirtioFileSystemDeviceConfiguration(tag string) (*VirtioFileSystemDeviceConfiguration, error) {
	if err := macOSAvailable(12); err != nil {
		return nil, err
	}
	tagChar := charWithGoString(tag)
	defer tagChar.Free()

	nserrPtr := newNSErrorAsNil()
	fsdConfig := &VirtioFileSystemDeviceConfiguration{
		pointer: objc.NewPointer(
			C.newVZVirtioFileSystemDeviceConfiguration(tagChar.CString(), &nserrPtr),
		),
	}
	if err := newNSError(nserrPtr); err != nil {
		return nil, err
	}
	objc.SetFinalizer(fsdConfig, func(self *VirtioFileSystemDeviceConfiguration) {
		objc.Release(self)
	})
	return fsdConfig, nil
}

// SetDirectoryShare sets the directory share associated with this configuration.
func (c *VirtioFileSystemDeviceConfiguration) SetDirectoryShare(share DirectoryShare) {
	C.setVZVirtioFileSystemDeviceConfigurationShare(objc.Ptr(c), objc.Ptr(share))
}

// SharedDirectory is a shared directory.
type SharedDirectory struct {
	*pointer
}

// NewSharedDirectory creates a new shared directory.
//
// This is only supported on macOS 12 and newer, error will
// be returned on older versions.
func NewSharedDirectory(dirPath string, readOnly bool) (*SharedDirectory, error) {
	if err := macOSAvailable(12); err != nil {
		return nil, err
	}
	if _, err := os.Stat(dirPath); err != nil {
		return nil, err
	}

	dirPathChar := charWithGoString(dirPath)
	defer dirPathChar.Free()
	sd := &SharedDirectory{
		pointer: objc.NewPointer(
			C.newVZSharedDirectory(dirPathChar.CString(), C.bool(readOnly)),
		),
	}
	objc.SetFinalizer(sd, func(self *SharedDirectory) {
		objc.Release(self)
	})
	return sd, nil
}

// DirectoryShare is the base interface for a directory share.
type DirectoryShare interface {
	objc.NSObject

	directoryShare()
}

type baseDirectoryShare struct{}

func (*baseDirectoryShare) directoryShare() {}

var _ DirectoryShare = (*SingleDirectoryShare)(nil)

// SingleDirectoryShare defines the directory share for a single directory.
type SingleDirectoryShare struct {
	*pointer

	*baseDirectoryShare
}

// NewSingleDirectoryShare creates a new single directory share.
//
// This is only supported on macOS 12 and newer, error will
// be returned on older versions.
func NewSingleDirectoryShare(share *SharedDirectory) (*SingleDirectoryShare, error) {
	if err := macOSAvailable(12); err != nil {
		return nil, err
	}
	config := &SingleDirectoryShare{
		pointer: objc.NewPointer(
			C.newVZSingleDirectoryShare(objc.Ptr(share)),
		),
	}
	objc.SetFinalizer(config, func(self *SingleDirectoryShare) {
		objc.Release(self)
	})
	return config, nil
}

// MultipleDirectoryShare defines the directory share for multiple directories.
type MultipleDirectoryShare struct {
	*pointer

	*baseDirectoryShare
}

var _ DirectoryShare = (*MultipleDirectoryShare)(nil)

// NewMultipleDirectoryShare creates a new multiple directories share.
//
// This is only supported on macOS 12 and newer, error will
// be returned on older versions.
func NewMultipleDirectoryShare(shares map[string]*SharedDirectory) (*MultipleDirectoryShare, error) {
	if err := macOSAvailable(12); err != nil {
		return nil, err
	}
	directories := make(map[string]objc.NSObject, len(shares))
	for k, v := range shares {
		directories[k] = v
	}

	dict := objc.ConvertToNSMutableDictionary(directories)

	config := &MultipleDirectoryShare{
		pointer: objc.NewPointer(
			C.newVZMultipleDirectoryShare(objc.Ptr(dict)),
		),
	}
	objc.SetFinalizer(config, func(self *MultipleDirectoryShare) {
		objc.Release(self)
	})
	return config, nil
}

// MacOSGuestAutomountTag returns the macOS automount tag.
//
// A device configured with this tag will be automatically mounted in a macOS guest.
// This is only supported on macOS 13 and newer, error will be returned on older versions.
func MacOSGuestAutomountTag() (string, error) {
	if err := macOSAvailable(13); err != nil {
		return "", err
	}
	cstring := (*char)(C.getMacOSGuestAutomountTag())
	return cstring.String(), nil
}
