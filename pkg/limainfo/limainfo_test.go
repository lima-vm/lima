// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package limainfo

import (
	"testing"

	"gotest.tools/v3/assert"

	// Register the internal qemu driver.
	_ "github.com/lima-vm/lima/v2/pkg/driver/qemu"
)

func TestDriverCanSnapshot(t *testing.T) {
	assert.Assert(t, *driverCanSnapshot(t.Context(), "qemu"))
	assert.Assert(t, driverCanSnapshot(t.Context(), "no-such-driver") == nil)
}
