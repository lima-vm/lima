// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package must

func Must[T any](obj T, err error) T {
	if err != nil {
		panic(err)
	}
	return obj
}
