package main

import (
	"strings"

	"github.com/lima-vm/lima/pkg/version"
	"github.com/reproducible-containers/repro-get/pkg/envutil"
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
	flags := rootCmd.PersistentFlags()
	flags.String("cache", envutil.String("REPRO_GET_CACHE", "/var/cache/repro-get"), "Cache directory [$REPRO_GET_CACHE]")

	defaultDistro, err := getDistroByName("")
	if err != nil {
		panic(err)
	}

	flags.String("distro", envutil.String("REPRO_GET_DISTRO", defaultDistro.Info().Name), "Distribution driver [$REPRO_GET_DISTRO]")
	flags.StringSlice("provider", envutil.StringSlice("REPRO_GET_PROVIDER", nil), "File provider [$REPRO_GET_PROVIDER]")
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		debug, _ := cmd.Flags().GetBool("debug")
		if debug {
			logrus.SetLevel(logrus.DebugLevel)
		}
		return nil
	}
	rootCmd.AddCommand(
		newDaemonCommand(),
		newHashGenerateCommand(),
		newInstallSystemdCommand(),
	)
	return rootCmd
}
