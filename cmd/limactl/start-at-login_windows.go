// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"

	"github.com/spf13/cobra"
)

func startAtLoginAction(_ *cobra.Command, _ []string) error {
	return errors.New("start-at-login command is only supported on macOS and Linux right now")
}
