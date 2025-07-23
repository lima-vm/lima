//go:build !windows

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"github.com/lima-vm/lima/v2/pkg/limatype"
)

func inspectStatus(instDir string, inst *limatype.Instance, y *limatype.LimaYAML) {
	inspectStatusWithPIDFiles(instDir, inst, y)
}

func GetSSHAddress(_ string) (string, error) {
	return "127.0.0.1", nil
}
