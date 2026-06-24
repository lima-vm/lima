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

// PointingDeviceConfiguration is an interface for a pointing device configuration.
type PointingDeviceConfiguration interface {
	objc.NSObject

	pointingDeviceConfiguration()
}

type basePointingDeviceConfiguration struct{}

func (*basePointingDeviceConfiguration) pointingDeviceConfiguration() {}

// USBScreenCoordinatePointingDeviceConfiguration is a struct that defines the configuration
// for a USB pointing device that reports absolute coordinates.
type USBScreenCoordinatePointingDeviceConfiguration struct {
	*pointer

	*basePointingDeviceConfiguration
}

var _ PointingDeviceConfiguration = (*USBScreenCoordinatePointingDeviceConfiguration)(nil)

// NewUSBScreenCoordinatePointingDeviceConfiguration creates a new USBScreenCoordinatePointingDeviceConfiguration.
//
// This is only supported on macOS 12 and newer, error will
// be returned on older versions.
func NewUSBScreenCoordinatePointingDeviceConfiguration() (*USBScreenCoordinatePointingDeviceConfiguration, error) {
	if err := macOSAvailable(12); err != nil {
		return nil, err
	}
	config := &USBScreenCoordinatePointingDeviceConfiguration{
		pointer: objc.NewPointer(
			C.newVZUSBScreenCoordinatePointingDeviceConfiguration(),
		),
	}
	objc.SetFinalizer(config, func(self *USBScreenCoordinatePointingDeviceConfiguration) {
		objc.Release(self)
	})
	return config, nil
}
