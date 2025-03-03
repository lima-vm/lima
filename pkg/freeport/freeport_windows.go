// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package freeport

import "github.com/lima-vm/lima/pkg/windows"

func VSock() (int, error) {
	return windows.GetRandomFreeVSockPort(0, 2147483647)
}
