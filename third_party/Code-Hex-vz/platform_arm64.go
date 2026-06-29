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

// MacPlatformConfiguration is the platform configuration for booting macOS on Apple silicon.
//
// When creating a VM, the hardwareModel and auxiliaryStorage depend on the restore image that you use to install macOS.
//
// To choose the hardware model, start from MacOSRestoreImage.MostFeaturefulSupportedConfiguration method to get a supported
// configuration, then use its MacOSConfigurationRequirements.HardwareModel method to get the hardware model.
//
// Use the hardware model to set up MacPlatformConfiguration and to initialize a new auxiliary storage with
// `WithCreatingStorage` functional option of the `NewMacAuxiliaryStorage`.
//
// When you save a VM to disk and load it again, you must restore the HardwareModel, MachineIdentifier and
// AuxiliaryStorage methods to their original values.
//
// If you create multiple VMs from the same configuration, each should have a unique auxiliaryStorage and machineIdentifier.
type MacPlatformConfiguration struct {
	*pointer

	*basePlatformConfiguration

	hardwareModel     *MacHardwareModel
	machineIdentifier *MacMachineIdentifier
	auxiliaryStorage  *MacAuxiliaryStorage
}

var _ PlatformConfiguration = (*MacPlatformConfiguration)(nil)

// MacPlatformConfigurationOption is an optional function to create its configuration.
type MacPlatformConfigurationOption func(*MacPlatformConfiguration)

// WithMacHardwareModel is an option to create a new MacPlatformConfiguration.
func WithMacHardwareModel(m *MacHardwareModel) MacPlatformConfigurationOption {
	return func(mpc *MacPlatformConfiguration) {
		mpc.hardwareModel = m
		C.setHardwareModelVZMacPlatformConfiguration(objc.Ptr(mpc), objc.Ptr(m))
	}
}

// WithMacMachineIdentifier is an option to create a new MacPlatformConfiguration.
func WithMacMachineIdentifier(m *MacMachineIdentifier) MacPlatformConfigurationOption {
	return func(mpc *MacPlatformConfiguration) {
		mpc.machineIdentifier = m
		C.setMachineIdentifierVZMacPlatformConfiguration(objc.Ptr(mpc), objc.Ptr(m))
	}
}

// WithMacAuxiliaryStorage is an option to create a new MacPlatformConfiguration.
func WithMacAuxiliaryStorage(m *MacAuxiliaryStorage) MacPlatformConfigurationOption {
	return func(mpc *MacPlatformConfiguration) {
		mpc.auxiliaryStorage = m
		C.setAuxiliaryStorageVZMacPlatformConfiguration(objc.Ptr(mpc), objc.Ptr(m))
	}
}

// NewMacPlatformConfiguration creates a new MacPlatformConfiguration. see also it's document.
//
// This is only supported on macOS 12 and newer, error will
// be returned on older versions.
func NewMacPlatformConfiguration(opts ...MacPlatformConfigurationOption) (*MacPlatformConfiguration, error) {
	if err := macOSAvailable(12); err != nil {
		return nil, err
	}

	platformConfig := &MacPlatformConfiguration{
		pointer: objc.NewPointer(
			C.newVZMacPlatformConfiguration(),
		),
	}
	for _, optFunc := range opts {
		optFunc(platformConfig)
	}
	objc.SetFinalizer(platformConfig, func(self *MacPlatformConfiguration) {
		objc.Release(self)
	})
	return platformConfig, nil
}

// HardwareModel returns the Mac hardware model.
func (m *MacPlatformConfiguration) HardwareModel() *MacHardwareModel { return m.hardwareModel }

// MachineIdentifier returns the Mac machine identifier.
func (m *MacPlatformConfiguration) MachineIdentifier() *MacMachineIdentifier {
	return m.machineIdentifier
}

// AuxiliaryStorage returns the Mac auxiliary storage.
func (m *MacPlatformConfiguration) AuxiliaryStorage() *MacAuxiliaryStorage { return m.auxiliaryStorage }
