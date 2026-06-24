//go:build darwin && arm64
// +build darwin,arm64

package vz

/*
#cgo darwin CFLAGS: -mmacosx-version-min=11 -x objective-c -fno-objc-arc
#cgo darwin LDFLAGS: -lobjc -framework Foundation -framework Virtualization
# include "virtualization_13_arm64.h"
# include "virtualization_14_arm64.h"
*/
import "C"
import (
	"fmt"
	"runtime/cgo"
	"unsafe"

	"github.com/Code-Hex/vz/v3/internal/objc"
)

// LinuxRosettaAvailability represents an availability of Rosetta support for Linux binaries.
//
//go:generate go run ./cmd/addtags -tags=darwin,arm64 -file linuxrosettaavailability_string_arm64.go stringer -type=LinuxRosettaAvailability -output=linuxrosettaavailability_string_arm64.go
type LinuxRosettaAvailability int

const (
	// LinuxRosettaAvailabilityNotSupported Rosetta support for Linux binaries is not available on the host system.
	LinuxRosettaAvailabilityNotSupported LinuxRosettaAvailability = iota

	// LinuxRosettaAvailabilityNotInstalled Rosetta support for Linux binaries is not installed on the host system.
	LinuxRosettaAvailabilityNotInstalled

	// LinuxRosettaAvailabilityInstalled Rosetta support for Linux is installed on the host system.
	LinuxRosettaAvailabilityInstalled
)

//export linuxInstallRosettaWithCompletionHandler
func linuxInstallRosettaWithCompletionHandler(cgoHandleUintptr C.uintptr_t, errPtr unsafe.Pointer) {
	cgoHandle := cgo.Handle(cgoHandleUintptr)

	handler := cgoHandle.Value().(func(error))

	if err := newNSError(errPtr); err != nil {
		handler(err)
	} else {
		handler(nil)
	}
}

// LinuxRosettaDirectoryShare directory share to enable Rosetta support for Linux binaries.
// see: https://developer.apple.com/documentation/virtualization/vzlinuxrosettadirectoryshare?language=objc
type LinuxRosettaDirectoryShare struct {
	*pointer

	*baseDirectoryShare
}

var _ DirectoryShare = (*LinuxRosettaDirectoryShare)(nil)

// NewLinuxRosettaDirectoryShare creates a new Rosetta directory share if Rosetta support
// for Linux binaries is installed.
//
// This is only supported on macOS 13 and newer, error will
// be returned on older versions.
func NewLinuxRosettaDirectoryShare() (*LinuxRosettaDirectoryShare, error) {
	if err := macOSAvailable(13); err != nil {
		return nil, err
	}
	nserrPtr := newNSErrorAsNil()
	ds := &LinuxRosettaDirectoryShare{
		pointer: objc.NewPointer(
			C.newVZLinuxRosettaDirectoryShare(&nserrPtr),
		),
	}
	if err := newNSError(nserrPtr); err != nil {
		return nil, err
	}
	objc.SetFinalizer(ds, func(self *LinuxRosettaDirectoryShare) {
		objc.Release(self)
	})
	return ds, nil
}

// SetOptions enables translation caching and configure the socket communication type for Rosetta.
//
// This is only supported on macOS 14 and newer. Older versions do nothing.
func (ds *LinuxRosettaDirectoryShare) SetOptions(options LinuxRosettaCachingOptions) {
	if err := macOSAvailable(14); err != nil {
		return
	}
	C.setOptionsVZLinuxRosettaDirectoryShare(objc.Ptr(ds), objc.Ptr(options))
}

// LinuxRosettaDirectoryShareInstallRosetta download and install Rosetta support
// for Linux binaries if necessary.
//
// This is only supported on macOS 13 and newer, error will
// be returned on older versions.
func LinuxRosettaDirectoryShareInstallRosetta() error {
	if err := macOSAvailable(13); err != nil {
		return err
	}
	errCh := make(chan error, 1)
	cgoHandle := cgo.NewHandle(func(err error) {
		errCh <- err
	})
	C.linuxInstallRosetta(C.uintptr_t(cgoHandle))
	return <-errCh
}

// LinuxRosettaDirectoryShareAvailability checks the availability of Rosetta support
// for the directory share.
//
// This is only supported on macOS 13 and newer, LinuxRosettaAvailabilityNotSupported will
// be returned on older versions.
func LinuxRosettaDirectoryShareAvailability() LinuxRosettaAvailability {
	if err := macOSAvailable(13); err != nil {
		return LinuxRosettaAvailabilityNotSupported
	}
	return LinuxRosettaAvailability(C.availabilityVZLinuxRosettaDirectoryShare())
}

// LinuxRosettaCachingOptions for a directory sharing device configuration.
type LinuxRosettaCachingOptions interface {
	objc.NSObject

	linuxRosettaCachingOptions()
}

type baseLinuxRosettaCachingOptions struct{}

func (*baseLinuxRosettaCachingOptions) linuxRosettaCachingOptions() {}

// LinuxRosettaUnixSocketCachingOptions is an struct that represents caching options
// for a UNIX domain socket.
//
// This struct configures Rosetta to communicate with the Rosetta daemon using a UNIX domain socket.
type LinuxRosettaUnixSocketCachingOptions struct {
	*pointer

	*baseLinuxRosettaCachingOptions
}

var _ LinuxRosettaCachingOptions = (*LinuxRosettaUnixSocketCachingOptions)(nil)

// NewLinuxRosettaUnixSocketCachingOptions creates a new Rosetta caching options object for
// a UNIX domain socket with the path you specify.
//
// The path of the Unix Domain Socket to be used to communicate with the Rosetta translation daemon.
//
// This is only supported on macOS 14 and newer, error will
// be returned on older versions.
func NewLinuxRosettaUnixSocketCachingOptions(path string) (*LinuxRosettaUnixSocketCachingOptions, error) {
	if err := macOSAvailable(14); err != nil {
		return nil, err
	}
	maxPathLen := maximumPathLengthLinuxRosettaUnixSocketCachingOptions()
	if maxPathLen < len(path) {
		return nil, fmt.Errorf("path length exceeds maximum allowed length of %d", maxPathLen)
	}

	cs := charWithGoString(path)
	defer cs.Free()

	nserrPtr := newNSErrorAsNil()
	usco := &LinuxRosettaUnixSocketCachingOptions{
		pointer: objc.NewPointer(
			C.newVZLinuxRosettaUnixSocketCachingOptionsWithPath(cs.CString(), &nserrPtr),
		),
	}
	if err := newNSError(nserrPtr); err != nil {
		return nil, err
	}
	objc.SetFinalizer(usco, func(self *LinuxRosettaUnixSocketCachingOptions) {
		objc.Release(self)
	})
	return usco, nil
}

func maximumPathLengthLinuxRosettaUnixSocketCachingOptions() int {
	return int(uint32(C.maximumPathLengthVZLinuxRosettaUnixSocketCachingOptions()))
}

// LinuxRosettaAbstractSocketCachingOptions is caching options for an abstract socket.
//
// Use this object to configure Rosetta to communicate with the Rosetta daemon using an abstract socket.
type LinuxRosettaAbstractSocketCachingOptions struct {
	*pointer

	*baseLinuxRosettaCachingOptions
}

var _ LinuxRosettaCachingOptions = (*LinuxRosettaAbstractSocketCachingOptions)(nil)

// NewLinuxRosettaAbstractSocketCachingOptions creates a new LinuxRosettaAbstractSocketCachingOptions.
//
// The name of the Abstract Socket to be used to communicate with the Rosetta translation daemon.
//
// This is only supported on macOS 14 and newer, error will
// be returned on older versions.
func NewLinuxRosettaAbstractSocketCachingOptions(name string) (*LinuxRosettaAbstractSocketCachingOptions, error) {
	if err := macOSAvailable(14); err != nil {
		return nil, err
	}
	maxNameLen := maximumNameLengthVZLinuxRosettaAbstractSocketCachingOptions()
	if maxNameLen < len(name) {
		return nil, fmt.Errorf("name length exceeds maximum allowed length of %d", maxNameLen)
	}

	cs := charWithGoString(name)
	defer cs.Free()

	nserrPtr := newNSErrorAsNil()
	asco := &LinuxRosettaAbstractSocketCachingOptions{
		pointer: objc.NewPointer(
			C.newVZLinuxRosettaAbstractSocketCachingOptionsWithName(cs.CString(), &nserrPtr),
		),
	}
	if err := newNSError(nserrPtr); err != nil {
		return nil, err
	}
	objc.SetFinalizer(asco, func(self *LinuxRosettaAbstractSocketCachingOptions) {
		objc.Release(self)
	})
	return asco, nil
}

func maximumNameLengthVZLinuxRosettaAbstractSocketCachingOptions() int {
	return int(uint32(C.maximumNameLengthVZLinuxRosettaAbstractSocketCachingOptions()))
}
