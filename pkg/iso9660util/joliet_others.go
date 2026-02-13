//go:build !darwin

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package iso9660util

import (
	"errors"
	"runtime"
)

func writeJoliet(_, _ string, _ []Entry) error {
	return errors.New("joliet is not supported on " + runtime.GOOS)
}
