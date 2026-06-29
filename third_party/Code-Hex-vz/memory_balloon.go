package vz

/*
#cgo darwin CFLAGS: -mmacosx-version-min=11 -x objective-c -fno-objc-arc
#cgo darwin LDFLAGS: -lobjc -framework Foundation -framework Virtualization
# include "virtualization_11.h"
*/
import "C"
import (
	"unsafe"

	"github.com/Code-Hex/vz/v3/internal/objc"
)

// MemoryBalloonDeviceConfiguration for a memory balloon device configuration.
type MemoryBalloonDeviceConfiguration interface {
	objc.NSObject

	memoryBalloonDeviceConfiguration()
}

type baseMemoryBalloonDeviceConfiguration struct{}

func (*baseMemoryBalloonDeviceConfiguration) memoryBalloonDeviceConfiguration() {}

var _ MemoryBalloonDeviceConfiguration = (*VirtioTraditionalMemoryBalloonDeviceConfiguration)(nil)

// VirtioTraditionalMemoryBalloonDeviceConfiguration is a configuration of the Virtio traditional memory balloon device.
//
// see: https://developer.apple.com/documentation/virtualization/vzvirtiotraditionalmemoryballoondeviceconfiguration?language=objc
type VirtioTraditionalMemoryBalloonDeviceConfiguration struct {
	*pointer

	*baseMemoryBalloonDeviceConfiguration
}

// NewVirtioTraditionalMemoryBalloonDeviceConfiguration creates a new VirtioTraditionalMemoryBalloonDeviceConfiguration.
//
// This is only supported on macOS 11 and newer, error will
// be returned on older versions.
func NewVirtioTraditionalMemoryBalloonDeviceConfiguration() (*VirtioTraditionalMemoryBalloonDeviceConfiguration, error) {
	if err := macOSAvailable(11); err != nil {
		return nil, err
	}

	config := &VirtioTraditionalMemoryBalloonDeviceConfiguration{
		pointer: objc.NewPointer(
			C.newVZVirtioTraditionalMemoryBalloonDeviceConfiguration(),
		),
	}
	objc.SetFinalizer(config, func(self *VirtioTraditionalMemoryBalloonDeviceConfiguration) {
		objc.Release(self)
	})
	return config, nil
}

// MemoryBalloonDevice is the base interface for memory balloon devices.
//
// This represents MemoryBalloonDevice in the Virtualization framework.
// It is an abstract class that should not be directly used.
//
// see: https://developer.apple.com/documentation/virtualization/vzmemoryballoondevice?language=objc
type MemoryBalloonDevice interface {
	objc.NSObject

	memoryBalloonDevice()
}

type baseMemoryBalloonDevice struct{}

func (*baseMemoryBalloonDevice) memoryBalloonDevice() {}

// MemoryBalloonDevices returns the list of memory balloon devices configured on this virtual machine.
//
// Returns an empty array if no memory balloon device is configured.
//
// This is only supported on macOS 11 and newer.
func (v *VirtualMachine) MemoryBalloonDevices() []MemoryBalloonDevice {
	nsArray := objc.NewNSArray(
		C.VZVirtualMachine_memoryBalloonDevices(objc.Ptr(v)),
	)
	ptrs := nsArray.ToPointerSlice()
	devices := make([]MemoryBalloonDevice, len(ptrs))
	for i, ptr := range ptrs {
		// TODO: When Apple adds more memory balloon device types in future macOS versions,
		// implement type checking here to create the appropriate device wrapper.
		// Currently, VirtioTraditionalMemoryBalloonDevice is the only type supported.
		devices[i] = newVirtioTraditionalMemoryBalloonDevice(ptr, v)
	}
	return devices
}

// VirtioTraditionalMemoryBalloonDevice represents a Virtio traditional memory balloon device.
//
// The balloon device allows for dynamic memory management by inflating or deflating
// the balloon to control memory available to the guest OS.
//
// see: https://developer.apple.com/documentation/virtualization/vzvirtiotraditionalmemoryballoondevice?language=objc
type VirtioTraditionalMemoryBalloonDevice struct {
	*pointer
	vm *VirtualMachine

	*baseMemoryBalloonDevice
}

var _ MemoryBalloonDevice = (*VirtioTraditionalMemoryBalloonDevice)(nil)

// AsVirtioTraditionalMemoryBalloonDevice attempts to convert a MemoryBalloonDevice to a VirtioTraditionalMemoryBalloonDevice.
//
// Returns the VirtioTraditionalMemoryBalloonDevice if the device is of that type, or nil otherwise.
func AsVirtioTraditionalMemoryBalloonDevice(device MemoryBalloonDevice) *VirtioTraditionalMemoryBalloonDevice {
	if traditionalDevice, ok := device.(*VirtioTraditionalMemoryBalloonDevice); ok {
		return traditionalDevice
	}
	return nil
}

func newVirtioTraditionalMemoryBalloonDevice(pointer unsafe.Pointer, vm *VirtualMachine) *VirtioTraditionalMemoryBalloonDevice {
	device := &VirtioTraditionalMemoryBalloonDevice{
		pointer: objc.NewPointer(pointer),
		vm:      vm,
	}
	objc.SetFinalizer(device, func(self *VirtioTraditionalMemoryBalloonDevice) {
		objc.Release(self)
	})
	return device
}

// SetTargetVirtualMachineMemorySize sets the target memory size in bytes for the virtual machine.
//
// This method inflates or deflates the memory balloon to adjust the amount of memory
// available to the guest OS. The target memory size must be less than the total memory
// configured for the virtual machine.
//
// This is only supported on macOS 11 and newer.
func (v *VirtioTraditionalMemoryBalloonDevice) SetTargetVirtualMachineMemorySize(targetMemorySize uint64) {
	C.VZVirtioTraditionalMemoryBalloonDevice_setTargetVirtualMachineMemorySize(
		objc.Ptr(v),
		v.vm.dispatchQueue,
		C.ulonglong(targetMemorySize),
	)
}

// GetTargetVirtualMachineMemorySize returns the current target memory size in bytes for the virtual machine.
//
// This is only supported on macOS 11 and newer.
func (v *VirtioTraditionalMemoryBalloonDevice) GetTargetVirtualMachineMemorySize() uint64 {
	return uint64(C.VZVirtioTraditionalMemoryBalloonDevice_getTargetVirtualMachineMemorySize(objc.Ptr(v), v.vm.dispatchQueue))
}
