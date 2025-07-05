//go:build darwin && !no_vz

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package vz

import "errors"

//nolint:revive,staticcheck // false positives with proper nouns
var errRosettaUnsupported = errors.New("Rosetta is unsupported on non-ARM64 hosts")
