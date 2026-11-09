//go:build !(darwin && cgo)

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"

	"github.com/spf13/cobra"
)

func newUsbCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "usb",
		Short:   "Live USB passthrough management (vz only, macOS only)",
		GroupID: advancedCommand,
		RunE: func(_ *cobra.Command, _ []string) error {
			return errors.New("USB passthrough is currently only supported on macOS with the vz driver")
		},
		FParseErrWhitelist: cobra.FParseErrWhitelist{UnknownFlags: true},
	}
}
