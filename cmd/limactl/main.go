package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/lima-vm/lima/pkg/osutil"
	"github.com/lima-vm/lima/pkg/store/dirnames"
	"github.com/lima-vm/lima/pkg/version"
	"github.com/mattn/go-isatty"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

const (
	DefaultInstanceName = "default"
)

func main() {
	if err := newApp().Execute(); err != nil {
		handleExitCoder(err)
		logrus.Fatal(err)
	}
}

func newApp() *cobra.Command {
	examplesDir := "$PREFIX/share/doc/lima/examples"
	if exe, err := os.Executable(); err == nil {
		binDir := filepath.Dir(exe)
		prefixDir := filepath.Dir(binDir)
		examplesDir = filepath.Join(prefixDir, "share/doc/lima/examples")
	}

	var rootCmd = &cobra.Command{
		Use:     "limactl",
		Short:   "Lima: Linux virtual machines",
		Version: strings.TrimPrefix(version.Version, "v"),
		Example: fmt.Sprintf(`  Start the default instance:
  $ limactl start

  Open a shell:
  $ lima

  Run a container:
  $ lima nerdctl run -d --name nginx -p 8080:80 nginx:alpine

  Stop the default instance:
  $ limactl stop

  See also example YAMLs: %s`, examplesDir),
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	rootCmd.PersistentFlags().Bool("debug", false, "debug mode")
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		debug, _ := cmd.Flags().GetBool("debug")
		if debug {
			logrus.SetLevel(logrus.DebugLevel)
		}

		if osutil.IsBeingRosettaTranslated() {
			// running under rosetta would provide inappropriate runtime.GOARCH info, see: https://github.com/lima-vm/lima/issues/543
			return errors.New("limactl is running under rosetta, please reinstall lima with native arch")
		}

		if runtime.GOOS == "windows" && isatty.IsCygwinTerminal(os.Stdout.Fd()) {
			formatter := new(logrus.TextFormatter)
			// the default setting does not recognize cygwin on windows
			formatter.ForceColors = true
			logrus.StandardLogger().SetFormatter(formatter)
		}
		// if os.Geteuid() == 0 {
		// 	return errors.New("must not run as the root")
		// }
		// Make sure either $HOME or $LIMA_HOME is defined, so we don't need
		// to check for errors later
		if _, err := dirnames.LimaDir(); err != nil {
			return err
		}
		return nil
	}
	rootCmd.AddCommand(
		newStartCommand(),
		newStopCommand(),
		newShellCommand(),
		newCopyCommand(),
		newListCommand(),
		newDeleteCommand(),
		newValidateCommand(),
		newSudoersCommand(),
		newPruneCommand(),
		newHostagentCommand(),
		newInfoCommand(),
		newShowSSHCommand(),
		newDebugCommand(),
		newEditCommand(),
		newFactoryResetCommand(),
		newDiskCommand(),
	)
	return rootCmd
}

type ExitCoder interface {
	error
	ExitCode() int
}

func handleExitCoder(err error) {
	if err == nil {
		return
	}

	if exitErr, ok := err.(ExitCoder); ok {
		os.Exit(exitErr.ExitCode())
		return
	}
}
