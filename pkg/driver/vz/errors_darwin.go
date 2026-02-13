//go:build darwin && !no_vz

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package vz

import "errors"

//nolint:revive,staticcheck,unused // false positives with proper nouns and GOARCH check
var (
	errRosettaUnsupported    = errors.New("Rosetta is unsupported on non-ARM64 hosts")
	errMacOSGuestUnsupported = errors.New("macOS guest is unsupported on non-ARM64 hosts")
	errUnimplemented         = errors.New("unimplemented")
)
