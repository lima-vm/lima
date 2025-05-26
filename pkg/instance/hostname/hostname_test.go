// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package instance_test

import (
	"testing"

	instance "github.com/lima-vm/lima/pkg/instance/hostname"
	"gotest.tools/v3/assert"
)

func TestHostnameFromInstName(t *testing.T) {
	assert.Equal(t, instance.HostnameFromInstName("default"), "lima-default")
	assert.Equal(t, instance.HostnameFromInstName("ubuntu-24.04"), "lima-ubuntu-24-04")
	assert.Equal(t, instance.HostnameFromInstName("foo_bar.baz"), "lima-foo-bar-baz")
}
