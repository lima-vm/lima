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

// AudioDeviceConfiguration interface for an audio device configuration.
type AudioDeviceConfiguration interface {
	objc.NSObject

	audioDeviceConfiguration()
}

type baseAudioDeviceConfiguration struct{}

func (*baseAudioDeviceConfiguration) audioDeviceConfiguration() {}

// VirtioSoundDeviceConfiguration is a struct that defines a Virtio sound device configuration.
//
// Use a VirtioSoundDeviceConfiguration to configure an audio device for your VM. After creating
// this struct, assign appropriate values via the SetStreams method which defines the behaviors of
// the underlying audio streams for this audio device.
//
// After creating and configuring a VirtioSoundDeviceConfiguration struct, assign it to the
// SetAudioDevicesVirtualMachineConfiguration method of your VM’s configuration.
type VirtioSoundDeviceConfiguration struct {
	*pointer

	*baseAudioDeviceConfiguration
}

var _ AudioDeviceConfiguration = (*VirtioSoundDeviceConfiguration)(nil)

// NewVirtioSoundDeviceConfiguration creates a new sound device configuration.
//
// This is only supported on macOS 12 and newer, error will be returned
// on older versions.
func NewVirtioSoundDeviceConfiguration() (*VirtioSoundDeviceConfiguration, error) {
	if err := macOSAvailable(12); err != nil {
		return nil, err
	}
	config := &VirtioSoundDeviceConfiguration{
		pointer: objc.NewPointer(
			C.newVZVirtioSoundDeviceConfiguration(),
		),
	}
	objc.SetFinalizer(config, func(self *VirtioSoundDeviceConfiguration) {
		objc.Release(self)
	})
	return config, nil
}

// SetStreams sets the list of audio streams exposed by this device.
func (v *VirtioSoundDeviceConfiguration) SetStreams(streams ...VirtioSoundDeviceStreamConfiguration) {
	ptrs := make([]objc.NSObject, len(streams))
	for i, val := range streams {
		ptrs[i] = val
	}
	array := objc.ConvertToNSMutableArray(ptrs)
	C.setStreamsVZVirtioSoundDeviceConfiguration(
		objc.Ptr(v), objc.Ptr(array),
	)
}

// VirtioSoundDeviceStreamConfiguration interface for Virtio Sound Device Stream Configuration.
type VirtioSoundDeviceStreamConfiguration interface {
	objc.NSObject

	virtioSoundDeviceStreamConfiguration()
}

type baseVirtioSoundDeviceStreamConfiguration struct{}

func (*baseVirtioSoundDeviceStreamConfiguration) virtioSoundDeviceStreamConfiguration() {}

// VirtioSoundDeviceHostInputStreamConfiguration is a PCM stream of input audio data,
// such as from a microphone via host.
type VirtioSoundDeviceHostInputStreamConfiguration struct {
	*pointer

	*baseVirtioSoundDeviceStreamConfiguration
}

var _ VirtioSoundDeviceStreamConfiguration = (*VirtioSoundDeviceHostInputStreamConfiguration)(nil)

// NewVirtioSoundDeviceHostInputStreamConfiguration creates a new PCM stream configuration of input audio data from host.
//
// This is only supported on macOS 12 and newer, error will be returned
// on older versions.
func NewVirtioSoundDeviceHostInputStreamConfiguration() (*VirtioSoundDeviceHostInputStreamConfiguration, error) {
	if err := macOSAvailable(12); err != nil {
		return nil, err
	}
	config := &VirtioSoundDeviceHostInputStreamConfiguration{
		pointer: objc.NewPointer(
			C.newVZVirtioSoundDeviceHostInputStreamConfiguration(),
		),
	}
	objc.SetFinalizer(config, func(self *VirtioSoundDeviceHostInputStreamConfiguration) {
		objc.Release(self)
	})
	return config, nil
}

// VirtioSoundDeviceHostOutputStreamConfiguration is a struct that
// defines a Virtio host sound device output stream configuration.
//
// A PCM stream of output audio data, such as to a speaker from host.
type VirtioSoundDeviceHostOutputStreamConfiguration struct {
	*pointer

	*baseVirtioSoundDeviceStreamConfiguration
}

var _ VirtioSoundDeviceStreamConfiguration = (*VirtioSoundDeviceHostOutputStreamConfiguration)(nil)

// NewVirtioSoundDeviceHostOutputStreamConfiguration creates a new sounds device output stream configuration.
//
// This is only supported on macOS 12 and newer, error will be returned
// on older versions.
func NewVirtioSoundDeviceHostOutputStreamConfiguration() (*VirtioSoundDeviceHostOutputStreamConfiguration, error) {
	if err := macOSAvailable(12); err != nil {
		return nil, err
	}
	config := &VirtioSoundDeviceHostOutputStreamConfiguration{
		pointer: objc.NewPointer(
			C.newVZVirtioSoundDeviceHostOutputStreamConfiguration(),
		),
	}
	objc.SetFinalizer(config, func(self *VirtioSoundDeviceHostOutputStreamConfiguration) {
		objc.Release(self)
	})
	return config, nil
}
