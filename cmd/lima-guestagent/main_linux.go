// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"strings"

	"github.com/lima-vm/lima/pkg/debugutil"
	"github.com/lima-vm/lima/pkg/version"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func main() {
	if err := newApp().Execute(); err != nil {
		logrus.Fatal(err)
	}
}

func newApp() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:     "lima-guestagent",
		Short:   "Do not launch manually",
		Version: strings.TrimPrefix(version.Version, "v"),
	}
	rootCmd.PersistentFlags().Bool("debug", false, "debug mode")
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
		debug, _ := cmd.Flags().GetBool("debug")
		if debug {
			logrus.SetLevel(logrus.DebugLevel)
			debugutil.Debug = true
		}
		return nil
	}
	rootCmd.AddCommand(
		newDaemonCommand(),
		newInstallSystemdCommand(),
	)
	return rootCmd
}
