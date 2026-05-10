//go:build !darwin

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import "github.com/spf13/cobra"

func isPrivilegedHelperCommand(string) bool {
	return false
}

func registerHiddenCommands(_ *cobra.Command) {}
