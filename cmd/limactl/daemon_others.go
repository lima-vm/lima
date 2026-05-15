//go:build !darwin

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"

	"github.com/spf13/cobra"
)

func daemonInstallAction(_ *cobra.Command, _ []string) error {
	return errors.New("daemon install is only supported on macOS")
}

func daemonUninstallAction(_ *cobra.Command, _ []string) error {
	return errors.New("daemon uninstall is only supported on macOS")
}
