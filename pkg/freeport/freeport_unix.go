//go:build !windows

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package freeport

import "errors"

func VSock() (int, error) {
	return 0, errors.New("freeport.VSock is not implemented for non-Windows hosts")
}
