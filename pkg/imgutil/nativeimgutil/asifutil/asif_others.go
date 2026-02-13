//go:build !darwin

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package asifutil

import (
	"errors"
	"os"
)

var ErrASIFNotSupported = errors.New("ASIF is only supported on macOS")

func NewASIF(_ string, _ int64) error {
	return ErrASIFNotSupported
}

func NewAttachedASIF(_ string, _ int64) (string, *os.File, error) {
	return "", nil, ErrASIFNotSupported
}

func DetachASIF(_ string) error {
	return ErrASIFNotSupported
}

func ResizeASIF(_ string, _ int64) error {
	return ErrASIFNotSupported
}
