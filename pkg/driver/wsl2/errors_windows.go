//go:build windows && !no_wsl

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package wsl2

import "errors"

var errUnimplemented = errors.New("unimplemented")
