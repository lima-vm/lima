//go:build darwin && !no_vz

package vz

import "errors"

//nolint:revive // error-strings
var errRosettaUnsupported = errors.New("Rosetta is unsupported on non-ARM64 hosts")
