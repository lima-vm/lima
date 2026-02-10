//go:build !darwin

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"

	"github.com/spf13/cobra"
)

func newVmnetAction(_ *cobra.Command, _ []string) error {
	return errors.New("vmnet command is only supported on macOS")
}
