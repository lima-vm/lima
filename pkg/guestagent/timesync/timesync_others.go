// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !linux

package timesync

import (
	"errors"
	"time"
)

var errNotSupported = errors.New("timesync: not supported on this platform")

func SetSystemTime(_ time.Time) error {
	return errNotSupported
}
