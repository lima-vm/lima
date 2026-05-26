// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

// Package ptr holds utilities for taking pointer references to values.
package ptr

// Of returns pointer to value.
func Of[T any](value T) *T {
	return &value
}
