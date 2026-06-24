package vz

/*
#cgo darwin CFLAGS: -mmacosx-version-min=11 -x objective-c -fno-objc-arc
#cgo darwin LDFLAGS: -lobjc -framework Foundation -framework Virtualization
# include "virtualization_15.h"
*/
import "C"
import (
	"runtime/cgo"
	"unsafe"

	"github.com/Code-Hex/vz/v3/internal/objc"
)

// NewUSBMassStorageDevice initialize the runtime USB Mass Storage device object.
//
// This is only supported on macOS 15 and newer, error will
// be returned on older versions.
func NewUSBMassStorageDevice(config *USBMassStorageDeviceConfiguration) (USBDevice, error) {
	if err := macOSAvailable(15); err != nil {
		return nil, err
	}
	ptr := C.newVZUSBMassStorageDeviceWithConfiguration(objc.Ptr(config))
	return newUSBDevice(ptr), nil
}

// USBControllerConfiguration for a usb controller configuration.
type USBControllerConfiguration interface {
	objc.NSObject

	usbControllerConfiguration()
}

type baseUSBControllerConfiguration struct{}

func (*baseUSBControllerConfiguration) usbControllerConfiguration() {}

// XHCIControllerConfiguration is a configuration of the USB XHCI controller.
//
// This configuration creates a USB XHCI controller device for the guest.
// see: https://developer.apple.com/documentation/virtualization/vzxhcicontrollerconfiguration?language=objc
type XHCIControllerConfiguration struct {
	*pointer

	*baseUSBControllerConfiguration
}

var _ USBControllerConfiguration = (*XHCIControllerConfiguration)(nil)

// NewXHCIControllerConfiguration creates a new XHCIControllerConfiguration.
//
// This is only supported on macOS 15 and newer, error will
// be returned on older versions.
func NewXHCIControllerConfiguration() (*XHCIControllerConfiguration, error) {
	if err := macOSAvailable(15); err != nil {
		return nil, err
	}

	config := &XHCIControllerConfiguration{
		pointer: objc.NewPointer(C.newVZXHCIControllerConfiguration()),
	}

	objc.SetFinalizer(config, func(self *XHCIControllerConfiguration) {
		objc.Release(self)
	})
	return config, nil
}

// USBController is representing a USB controller in a virtual machine.
type USBController struct {
	dispatchQueue unsafe.Pointer
	*pointer
}

func newUSBController(ptr, dispatchQueue unsafe.Pointer) *USBController {
	return &USBController{
		dispatchQueue: dispatchQueue,
		pointer:       objc.NewPointer(ptr),
	}
}

//export usbAttachDetachCompletionHandler
func usbAttachDetachCompletionHandler(cgoHandleUintptr C.uintptr_t, errPtr unsafe.Pointer) {
	cgoHandle := cgo.Handle(cgoHandleUintptr)

	handler := cgoHandle.Value().(func(error))

	if err := newNSError(errPtr); err != nil {
		handler(err)
	} else {
		handler(nil)
	}
}

// Attach attaches a USB device.
//
// This is only supported on macOS 15 and newer, error will
// be returned on older versions.
//
// If the device is successfully attached to the controller, it will appear in the usbDevices property,
// its usbController property will be set to point to the USB controller that it is attached to
// and completion handler will return nil.
// If the device was previously attached to this or another USB controller, attach function will fail
// with the `vz.ErrorDeviceAlreadyAttached`. If the device cannot be initialized correctly, attach
// function will fail with `vz.ErrorDeviceInitializationFailure`.
func (u *USBController) Attach(device USBDevice) error {
	if err := macOSAvailable(15); err != nil {
		return err
	}
	h, errCh := makeHandler()
	handle := cgo.NewHandle(h)
	defer handle.Delete()
	C.attachDeviceVZUSBController(
		objc.Ptr(u),
		objc.Ptr(device),
		u.dispatchQueue,
		C.uintptr_t(handle),
	)
	return <-errCh
}

// Detach detaches a USB device.
//
// This is only supported on macOS 15 and newer, error will
// be returned on older versions.
//
// If the device is successfully detached from the controller, it will disappear from the usbDevices property,
// its usbController property will be set to nil and completion handler will return nil.
// If the device wasn't attached to the controller at the time of calling detach method, it will fail
// with the `vz.ErrorDeviceNotFound` error.
func (u *USBController) Detach(device USBDevice) error {
	if err := macOSAvailable(15); err != nil {
		return err
	}
	h, errCh := makeHandler()
	handle := cgo.NewHandle(h)
	defer handle.Delete()
	C.detachDeviceVZUSBController(
		objc.Ptr(u),
		objc.Ptr(device),
		u.dispatchQueue,
		C.uintptr_t(handle),
	)
	return <-errCh
}

// USBDevices return a list of USB devices attached to controller.
//
// This is only supported on macOS 15 and newer, nil will
// be returned on older versions.
func (u *USBController) USBDevices() []USBDevice {
	if err := macOSAvailable(15); err != nil {
		return nil
	}
	nsArray := objc.NewNSArray(
		C.usbDevicesVZUSBController(objc.Ptr(u)),
	)
	ptrs := nsArray.ToPointerSlice()
	usbDevices := make([]USBDevice, len(ptrs))
	for i, ptr := range ptrs {
		usbDevices[i] = newUSBDevice(ptr)
	}
	return usbDevices
}

// USBDevice is an interface that represents a USB device in a VM.
type USBDevice interface {
	objc.NSObject

	UUID() string

	usbDevice()
}

func newUSBDevice(ptr unsafe.Pointer) *usbDevice {
	return &usbDevice{
		pointer: objc.NewPointer(ptr),
	}
}

type usbDevice struct {
	*pointer
}

func (*usbDevice) usbDevice() {}

var _ USBDevice = (*usbDevice)(nil)

// UUID returns the device UUID.
func (u *usbDevice) UUID() string {
	cs := (*char)(C.getUUIDUSBDevice(objc.Ptr(u)))
	return cs.String()
}
