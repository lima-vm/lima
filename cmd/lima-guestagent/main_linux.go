package main

import (
	"errors"
	"os"
	"strings"

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
	var rootCmd = &cobra.Command{
		Use:     "lima-guestagent",
		Short:   "Do not launch manually",
		Version: strings.TrimPrefix(version.Version, "v"),
	}
	rootCmd.PersistentFlags().Bool("debug", false, "debug mode")
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		debug, _ := cmd.Flags().GetBool("debug")
		if debug {
			logrus.SetLevel(logrus.DebugLevel)
		}
		if os.Geteuid() == 0 {
			return errors.New("must not run as the root")
		}
		return nil
	}
	rootCmd.AddCommand(
		newDaemonCommand(),
		newInstallSystemdCommand(),
	)
	return rootCmd
}
