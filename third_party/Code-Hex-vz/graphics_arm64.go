//go:build darwin && arm64
// +build darwin,arm64

package vz

/*
#cgo darwin CFLAGS: -mmacosx-version-min=11 -x objective-c -fno-objc-arc
#cgo darwin LDFLAGS: -lobjc -framework Foundation -framework Virtualization
# include "virtualization_12_arm64.h"
*/
import "C"
import (
	"github.com/Code-Hex/vz/v3/internal/objc"
)

// MacGraphicsDeviceConfiguration is a configuration for a display attached to a Mac graphics device.
type MacGraphicsDeviceConfiguration struct {
	*pointer

	*baseGraphicsDeviceConfiguration
}

var _ GraphicsDeviceConfiguration = (*MacGraphicsDeviceConfiguration)(nil)

// NewMacGraphicsDeviceConfiguration creates a new MacGraphicsDeviceConfiguration.
//
// This is only supported on macOS 12 and newer, error will
// be returned on older versions.
func NewMacGraphicsDeviceConfiguration() (*MacGraphicsDeviceConfiguration, error) {
	if err := macOSAvailable(12); err != nil {
		return nil, err
	}

	graphicsConfiguration := &MacGraphicsDeviceConfiguration{
		pointer: objc.NewPointer(
			C.newVZMacGraphicsDeviceConfiguration(),
		),
	}
	objc.SetFinalizer(graphicsConfiguration, func(self *MacGraphicsDeviceConfiguration) {
		objc.Release(self)
	})
	return graphicsConfiguration, nil
}

// SetDisplays sets the displays associated with this graphics device.
func (m *MacGraphicsDeviceConfiguration) SetDisplays(displayConfigs ...*MacGraphicsDisplayConfiguration) {
	ptrs := make([]objc.NSObject, len(displayConfigs))
	for i, val := range displayConfigs {
		ptrs[i] = val
	}
	array := objc.ConvertToNSMutableArray(ptrs)
	C.setDisplaysVZMacGraphicsDeviceConfiguration(objc.Ptr(m), objc.Ptr(array))
}

// MacGraphicsDisplayConfiguration is the configuration for a Mac graphics device.
type MacGraphicsDisplayConfiguration struct {
	*pointer
}

// NewMacGraphicsDisplayConfiguration creates a new MacGraphicsDisplayConfiguration.
//
// Creates a display configuration with the specified pixel dimensions and pixel density.
//
// This is only supported on macOS 12 and newer, error will
// be returned on older versions.
func NewMacGraphicsDisplayConfiguration(widthInPixels int64, heightInPixels int64, pixelsPerInch int64) (*MacGraphicsDisplayConfiguration, error) {
	if err := macOSAvailable(12); err != nil {
		return nil, err
	}

	graphicsDisplayConfiguration := &MacGraphicsDisplayConfiguration{
		pointer: objc.NewPointer(
			C.newVZMacGraphicsDisplayConfiguration(
				C.NSInteger(widthInPixels),
				C.NSInteger(heightInPixels),
				C.NSInteger(pixelsPerInch),
			),
		),
	}
	objc.SetFinalizer(graphicsDisplayConfiguration, func(self *MacGraphicsDisplayConfiguration) {
		objc.Release(self)
	})
	return graphicsDisplayConfiguration, nil
}
