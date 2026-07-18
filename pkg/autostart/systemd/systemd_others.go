//go:build !linux

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package systemd

func CurrentUnitName() string {
	return ""
}

func IsRunningSystemd() bool {
	return false
}
