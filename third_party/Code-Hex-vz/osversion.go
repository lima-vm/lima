package vz

/*
#cgo darwin CFLAGS: -mmacosx-version-min=11 -x objective-c -fno-objc-arc
#cgo darwin LDFLAGS: -lobjc -framework Foundation
# include "virtualization_helper.h"
*/
import "C"
import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"golang.org/x/mod/semver"
)

var (
	// ErrUnsupportedOSVersion is returned when calling a method which is only
	// available in newer macOS versions.
	ErrUnsupportedOSVersion = errors.New("unsupported macOS version")

	// ErrBuildTargetOSVersion indicates that the API is available but the
	// running program has disabled it.
	ErrBuildTargetOSVersion = errors.New("unsupported build target macOS version")
)

func macOSAvailable(version float64) error {
	if macOSMajorMinorVersion() < version {
		return ErrUnsupportedOSVersion
	}
	return macOSBuildTargetAvailable(version)
}

var (
	majorMinorVersion     float64
	majorMinorVersionOnce interface{ Do(func()) } = &sync.Once{}

	// This can be replaced in the test code to enable mock.
	// It will not be changed in production.
	sysctl = syscall.Sysctl
)

func fetchMajorMinorVersion() (float64, error) {
	osver, err := sysctl("kern.osproductversion")
	if err != nil {
		return 0, err
	}
	prefix := "v"
	majorMinor := strings.TrimPrefix(semver.MajorMinor(prefix+osver), prefix)
	version, err := strconv.ParseFloat(majorMinor, 64)
	if err != nil {
		return 0, err
	}
	return version, nil
}

func macOSMajorMinorVersion() float64 {
	majorMinorVersionOnce.Do(func() {
		version, err := fetchMajorMinorVersion()
		if err != nil {
			panic(err)
		}
		majorMinorVersion = version
	})
	return majorMinorVersion
}

var (
	maxAllowedVersion     int
	maxAllowedVersionOnce interface{ Do(func()) } = &sync.Once{}

	getMaxAllowedVersion = func() int {
		return int(C.mac_os_x_version_max_allowed())
	}
)

func fetchMaxAllowedVersion() int {
	maxAllowedVersionOnce.Do(func() {
		maxAllowedVersion = getMaxAllowedVersion()
	})
	return maxAllowedVersion
}

// macOSBuildTargetAvailable checks whether the API available in a given version has been compiled.
func macOSBuildTargetAvailable(version float64) error {
	allowedVersion := fetchMaxAllowedVersion()
	if allowedVersion == 0 {
		return fmt.Errorf("undefined __MAC_OS_X_VERSION_MAX_ALLOWED: %w", ErrBuildTargetOSVersion)
	}

	// FIXME(codehex): smart way
	// This list from AvailabilityVersions.h
	var target int
	switch version {
	case 11:
		target = 110000 // __MAC_11_0
	case 12:
		target = 120000 // __MAC_12_0
	case 12.3:
		target = 120300 // __MAC_12_3
	case 13:
		target = 130000 // __MAC_13_0
	case 14:
		target = 140000 // __MAC_14_0
	case 15:
		target = 150000 // __MAC_15_0
	}
	if allowedVersion < target {
		return fmt.Errorf("%w for %.1f (the binary was built with __MAC_OS_X_VERSION_MAX_ALLOWED=%d; needs recompilation)",
			ErrBuildTargetOSVersion, version, allowedVersion)
	}
	return nil
}
