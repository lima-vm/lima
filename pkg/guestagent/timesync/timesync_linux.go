// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package timesync

import (
	"time"

	"golang.org/x/sys/unix"
)

func SetSystemTime(t time.Time) error {
	v := unix.NsecToTimeval(t.UnixNano())
	return unix.Settimeofday(&v)
}
