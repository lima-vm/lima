// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package identifierutil

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestHostnameFromInstName(t *testing.T) {
	assert.Equal(t, "lima-default", HostnameFromInstName("default"))
	assert.Equal(t, "lima-ubuntu-24-04", HostnameFromInstName("ubuntu-24.04"))
	assert.Equal(t, "lima-foo-bar-baz", HostnameFromInstName("foo_bar.baz"))
}
