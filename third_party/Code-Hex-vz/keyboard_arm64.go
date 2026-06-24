//go:build darwin && arm64
// +build darwin,arm64

package vz

/*
#cgo darwin CFLAGS: -mmacosx-version-min=11 -x objective-c -fno-objc-arc
#cgo darwin LDFLAGS: -lobjc -framework Foundation -framework Virtualization
# include "virtualization_14_arm64.h"
*/
import "C"
import (
	"github.com/Code-Hex/vz/v3/internal/objc"
)

// MacKeyboardConfiguration is a struct that defines the configuration
// for a Mac keyboard.
//
// This device is only recognized by virtual machines running macOS 14.0 and later.
// In order to support both macOS 13.0 and earlier guests, VirtualMachineConfiguration.keyboards
// can be set to an array containing both a MacKeyboardConfiguration and
// a USBKeyboardConfiguration object. macOS 14.0 and later guests will use the Mac keyboard device,
// while earlier versions of macOS will use the USB keyboard device.
//
// see: https://developer.apple.com/documentation/virtualization/vzmackeyboardconfiguration?language=objc
type MacKeyboardConfiguration struct {
	*pointer

	*baseKeyboardConfiguration
}

var _ KeyboardConfiguration = (*MacKeyboardConfiguration)(nil)

// NewMacKeyboardConfiguration creates a new MacKeyboardConfiguration.
//
// This is only supported on macOS 14 and newer, error will
// be returned on older versions.
func NewMacKeyboardConfiguration() (*MacKeyboardConfiguration, error) {
	if err := macOSAvailable(14); err != nil {
		return nil, err
	}
	config := &MacKeyboardConfiguration{
		pointer: objc.NewPointer(
			C.newVZMacKeyboardConfiguration(),
		),
	}
	objc.SetFinalizer(config, func(self *MacKeyboardConfiguration) {
		objc.Release(self)
	})
	return config, nil
}
