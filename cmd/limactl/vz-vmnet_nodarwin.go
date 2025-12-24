//go:build !darwin

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"

	"github.com/spf13/cobra"
)

func newvzvmnetAction(_ *cobra.Command, _ []string) error {
	return errors.New("vz-vmnet command is only supported on macOS")
}
