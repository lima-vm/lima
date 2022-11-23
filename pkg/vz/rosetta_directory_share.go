//go:build darwin && !arm64 && !no_vz
// +build darwin,!arm64,!no_vz

package vz

import (
	"github.com/Code-Hex/vz/v3"
)

func createRosettaDirectoryShareConfiguration() (*vz.VirtioFileSystemDeviceConfiguration, error) {
	return nil, errRosettaUnsupported
}
