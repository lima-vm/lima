// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package osutil

import "golang.org/x/sys/unix"

// BootSessionID returns an identifier of the current boot of the host.
// The identifier changes on every reboot, so it can be used to detect state that was left
// behind by a previous boot.
func BootSessionID() (string, error) {
	return unix.Sysctl("kern.bootsessionuuid")
}
