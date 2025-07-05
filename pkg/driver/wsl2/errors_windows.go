//go:build windows && !no_wsl

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package wsl2

import "errors"

//nolint:revive,staticcheck // false positives with proper nouns
var errUnimplemented = errors.New("unimplemented")
