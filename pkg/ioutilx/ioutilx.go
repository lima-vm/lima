// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package ioutilx

import (
	"errors"
	"fmt"
	"io"
)

// ReadAtMaximum reads n at maximum.
func ReadAtMaximum(r io.Reader, n int64) ([]byte, error) {
	lr := &io.LimitedReader{
		R: r,
		N: n,
	}
	b, err := io.ReadAll(lr)
	if err != nil {
		if errors.Is(err, io.EOF) && lr.N <= 0 {
			err = fmt.Errorf("exceeded the limit (%d bytes): %w", n, err)
		}
	}
	return b, err
}
