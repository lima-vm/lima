// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package ioutilx

import (
	"fmt"
	"io"
)

// ReadAtMaximum reads n bytes at maximum.
// It returns an error if r has more than n bytes.
func ReadAtMaximum(r io.Reader, n int64) ([]byte, error) {
	// Read one extra byte to detect whether r exceeds the limit.
	// io.ReadAll never returns io.EOF, so the size has to be checked explicitly.
	b, err := io.ReadAll(io.LimitReader(r, n+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > n {
		return nil, fmt.Errorf("exceeded the limit (%d bytes)", n)
	}
	return b, nil
}
