package vz

/*
#cgo darwin CFLAGS: -mmacosx-version-min=11 -x objective-c -fno-objc-arc
#cgo darwin LDFLAGS: -lobjc -framework Foundation -framework Virtualization
# include "virtualization_11.h"
# include "virtualization_12.h"
*/
import "C"
import (
	"github.com/Code-Hex/vz/v3/internal/objc"
)

// KeyboardConfiguration interface for a keyboard configuration.
type KeyboardConfiguration interface {
	objc.NSObject

	keyboardConfiguration()
}

type baseKeyboardConfiguration struct{}

func (*baseKeyboardConfiguration) keyboardConfiguration() {}

// USBKeyboardConfiguration is a device that defines the configuration for a USB keyboard.
type USBKeyboardConfiguration struct {
	*pointer

	*baseKeyboardConfiguration
}

var _ KeyboardConfiguration = (*USBKeyboardConfiguration)(nil)

// NewUSBKeyboardConfiguration creates a new USB keyboard configuration.
//
// This is only supported on macOS 12 and newer, error will
// be returned on older versions.
func NewUSBKeyboardConfiguration() (*USBKeyboardConfiguration, error) {
	if err := macOSAvailable(12); err != nil {
		return nil, err
	}
	config := &USBKeyboardConfiguration{
		pointer: objc.NewPointer(C.newVZUSBKeyboardConfiguration()),
	}
	objc.SetFinalizer(config, func(self *USBKeyboardConfiguration) {
		objc.Release(self)
	})
	return config, nil
}
