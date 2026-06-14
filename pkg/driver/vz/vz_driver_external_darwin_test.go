//go:build darwin && !no_vz && external_vz

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package vz

import (
	"testing"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/limatype"
)

func TestValidateConfigRejectsBlockDevicesWithExternalVZ(t *testing.T) {
	cfg := &limatype.LimaYAML{BlockDevices: []string{"/dev/disk4"}}
	err := validateConfig(cfg)
	assert.ErrorContains(t, err, "external VZ")
}
