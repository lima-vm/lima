//go:build darwin && !no_vz
// +build darwin,!no_vz

package vz

import "errors"

var errRosettaUnsupported = errors.New("Rosetta is unsupported on non-ARM64 hosts")
