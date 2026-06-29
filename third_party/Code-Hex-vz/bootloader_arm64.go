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

// MacOSBootLoader is a boot loader configuration for booting macOS on Apple Silicon.
type MacOSBootLoader struct {
	*pointer

	*baseBootLoader
}

var _ BootLoader = (*MacOSBootLoader)(nil)

// NewMacOSBootLoader creates a new MacOSBootLoader struct.
//
// This is only supported on macOS 12 and newer, error will
// be returned on older versions.
func NewMacOSBootLoader() (*MacOSBootLoader, error) {
	if err := macOSAvailable(12); err != nil {
		return nil, err
	}

	bootLoader := &MacOSBootLoader{
		pointer: objc.NewPointer(
			C.newVZMacOSBootLoader(),
		),
	}
	objc.SetFinalizer(bootLoader, func(self *MacOSBootLoader) {
		objc.Release(self)
	})
	return bootLoader, nil
}
