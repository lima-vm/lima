// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import "github.com/spf13/cobra"

func additionalAdvancedCommands() []*cobra.Command {
	return []*cobra.Command{
		newSudoersCommand(),
		startAtLoginCommand(),
	}
}
