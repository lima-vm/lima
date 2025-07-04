//go:build windows

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package limayaml

func hostTimeZone() string {
	// WSL2 will automatically set the timezone
	return ""
}
