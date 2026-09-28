// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package krunkit

import (
	"testing"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/limatype"
)

func TestValidateConfigRejectsBlockDevices(t *testing.T) {
	err := validateConfig(&limatype.LimaYAML{BlockDevices: []string{"/dev/disk4"}})
	assert.ErrorContains(t, err, "field `blockDevices` is not supported for vmType: krunkit")
}
