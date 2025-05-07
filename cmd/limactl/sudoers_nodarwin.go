//go:build !darwin

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"

	"github.com/spf13/cobra"
)

func sudoersAction(_ *cobra.Command, _ []string) error {
	return errors.New("sudoers command is only supported on macOS right now")
}
