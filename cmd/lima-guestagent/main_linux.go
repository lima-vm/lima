// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"strings"

	yq "github.com/mikefarah/yq/v4/cmd"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/lima-vm/lima/v2/pkg/debugutil"
	"github.com/lima-vm/lima/v2/pkg/version"
)

func main() {
	// `lima-guestagent yq` executes the embedded `yq` command instead.
	if len(os.Args) > 1 && os.Args[1] == "yq" {
		os.Args = os.Args[1:]

		cmd := yq.New()
		args := os.Args[1:]
		_, _, err := cmd.Find(args)
		if err != nil && args[0] != "__complete" {
			// default command when nothing matches...
			newArgs := []string{"eval"}
			cmd.SetArgs(append(newArgs, os.Args[1:]...))
		}

		if err := cmd.Execute(); err != nil {
			os.Exit(1)
		}
		return
	}

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
	rootCmd.PersistentFlags().Bool("debug", false, "Debug mode")
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
