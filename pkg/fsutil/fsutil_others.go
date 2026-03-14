//go:build !linux

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package fsutil

func IsNFS(string) (bool, error) {
	return false, nil
}
