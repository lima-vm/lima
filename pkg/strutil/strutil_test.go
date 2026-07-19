// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package strutil

import (
	"bytes"
	"encoding/binary"
	"testing"

	"gotest.tools/v3/assert"
)

func TestFromUTF16leToString(t *testing.T) {
	// "Hi" in UTF-16LE without BOM
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint16(buf[0:], 'H')
	binary.LittleEndian.PutUint16(buf[2:], 'i')

	got, err := FromUTF16leToString(bytes.NewReader(buf))
	assert.NilError(t, err)
	assert.Equal(t, got, "Hi")
}
