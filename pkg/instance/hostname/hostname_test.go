// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package hostname_test

import (
	"testing"

	"github.com/lima-vm/lima/v2/pkg/instance/hostname"
	"gotest.tools/v3/assert"
)

func TestFromInstName(t *testing.T) {
	assert.Equal(t, hostname.FromInstName("default"), "lima-default")
	assert.Equal(t, hostname.FromInstName("ubuntu-24.04"), "lima-ubuntu-24-04")
	assert.Equal(t, hostname.FromInstName("foo_bar.baz"), "lima-foo-bar-baz")
}
