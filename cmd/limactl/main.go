// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/mattn/go-isatty"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/lima-vm/lima/v2/cmd/yq"
	"github.com/lima-vm/lima/v2/pkg/debugutil"
	"github.com/lima-vm/lima/v2/pkg/driver/external/server"
	"github.com/lima-vm/lima/v2/pkg/fsutil"
	"github.com/lima-vm/lima/v2/pkg/limatype/dirnames"
	"github.com/lima-vm/lima/v2/pkg/osutil"
	"github.com/lima-vm/lima/v2/pkg/plugins"
	"github.com/lima-vm/lima/v2/pkg/version"
)

const (
	DefaultInstanceName = "default"
	basicCommand        = "basic"
	advancedCommand     = "advanced"
)

func main() {
	if os.Geteuid() == 0 && (len(os.Args) < 2 || os.Args[1] != "generate-doc") {
		panic("must not run as the root user")
	}

	yq.MaybeRunYQ()
	if runtime.GOOS == "windows" {
		extras, hasExtra := os.LookupEnv("_LIMA_WINDOWS_EXTRA_PATH")
		if hasExtra && strings.TrimSpace(extras) != "" {
			p := os.Getenv("PATH")
			err := os.Setenv("PATH", strings.TrimSpace(extras)+string(filepath.ListSeparator)+p)
			if err != nil {
				logrus.Warning("Can't add extras to PATH, relying entirely on system PATH")
			}
		}
	}
	err := newApp().Execute()
	server.StopAllExternalDrivers()
	osutil.HandleExitError(err)
	if err != nil {
		logrus.Fatal(err)
	}
}

func processGlobalFlags(rootCmd *cobra.Command) error {
	// --log-level will override --debug, but will not reset debugutil.Debug
	if debug, _ := rootCmd.Flags().GetBool("debug"); debug {
		logrus.SetLevel(logrus.DebugLevel)
		debugutil.Debug = true
	}

	l, _ := rootCmd.Flags().GetString("log-level")
	if l != "" {
		lvl, err := logrus.ParseLevel(l)
		if err != nil {
			return err
		}
		logrus.SetLevel(lvl)
	}

	logFormat, _ := rootCmd.Flags().GetString("log-format")
	switch logFormat {
	case "json":
		formatter := new(logrus.JSONFormatter)
		logrus.StandardLogger().SetFormatter(formatter)
	case "text":
		// logrus use text format by default.
		if runtime.GOOS == "windows" && isatty.IsCygwinTerminal(os.Stderr.Fd()) {
			formatter := new(logrus.TextFormatter)
			// the default setting does not recognize cygwin on windows
			formatter.ForceColors = true
			logrus.StandardLogger().SetFormatter(formatter)
		}
	default:
		return fmt.Errorf("unsupported log-format: %q", logFormat)
	}
	return nil
}

func newApp() *cobra.Command {
	templatesDir := "$PREFIX/share/lima/templates"
	if exe, err := os.Executable(); err == nil {
		binDir := filepath.Dir(exe)
		prefixDir := filepath.Dir(binDir)
		templatesDir = filepath.Join(prefixDir, "share/lima/templates")
	}

	rootCmd := &cobra.Command{
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

  See also template YAMLs: %s`, templatesDir),
		SilenceUsage:      true,
		SilenceErrors:     true,
		DisableAutoGenTag: true,
	}
	rootCmd.PersistentFlags().String("log-level", "", "Set the logging level [trace, debug, info, warn, error]")
	rootCmd.PersistentFlags().String("log-format", "text", "Set the logging format [text, json]")
	rootCmd.PersistentFlags().Bool("debug", false, "Debug mode")
	// TODO: "survey" does not support using cygwin terminal on windows yet
	rootCmd.PersistentFlags().Bool("tty", isatty.IsTerminal(os.Stdout.Fd()), "Enable TUI interactions such as opening an editor. Defaults to true when stdout is a terminal. Set to false for automation.")
	rootCmd.PersistentFlags().BoolP("yes", "y", false, "Alias of --tty=false")
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
		if err := processGlobalFlags(rootCmd); err != nil {
			return err
		}

		if osutil.IsBeingRosettaTranslated() && cmd.Parent().Name() != "completion" && cmd.Name() != "generate-doc" && cmd.Name() != "validate" {
			// running under rosetta would provide inappropriate runtime.GOARCH info, see: https://github.com/lima-vm/lima/issues/543
			// allow commands that are used for packaging to run under rosetta to allow cross-architecture builds
			return errors.New("limactl is running under rosetta, please reinstall lima with native arch")
		}

		// Make sure either $HOME or $LIMA_HOME is defined, so we don't need
		// to check for errors later
		dir, err := dirnames.LimaDir()
		if err != nil {
			return err
		}
		nfs, err := fsutil.IsNFS(dir)
		if err != nil {
			return err
		}
		if nfs {
			return errors.New("must not run on NFS dir")
		}

		if cmd.Flags().Changed("yes") && cmd.Flags().Changed("tty") {
			return errors.New("cannot use both --tty and --yes flags at the same time")
		}

		if cmd.Flags().Changed("yes") {
			switch cmd.Name() {
			case "clone", "edit", "rename":
				logrus.Warn("--yes flag is deprecated (--tty=false is still supported and works in the same way. Also consider using --start)")
			}

			// Sets the value of the yesValue flag by using the yes flag.
			yesValue, _ := cmd.Flags().GetBool("yes")
			if yesValue {
				// Sets to the default value false
				err := cmd.Flags().Set("tty", "false")
				if err != nil {
					return err
				}
			}
		}
		return nil
	}
	rootCmd.AddGroup(&cobra.Group{ID: "basic", Title: "Basic Commands:"})
	rootCmd.AddGroup(&cobra.Group{ID: "advanced", Title: "Advanced Commands:"})
	rootCmd.AddGroup(&cobra.Group{ID: "plugin", Title: "Available Plugins (Experimental):"})

	rootCmd.AddCommand(
		newCreateCommand(),
		newStartCommand(),
		newStopCommand(),
		newShellCommand(),
		newCopyCommand(),
		newListCommand(),
		newDeleteCommand(),
		newValidateCommand(),
		newPruneCommand(),
		newHostagentCommand(),
		newGuestInstallCommand(),
		newInfoCommand(),
		newShowSSHCommand(),
		newDebugCommand(),
		newEditCommand(),
		newFactoryResetCommand(),
		newDiskCommand(),
		newUsernetCommand(),
		newGenDocCommand(),
		newGenSchemaCommand(),
		newSnapshotCommand(),
		newProtectCommand(),
		newUnprotectCommand(),
		newTunnelCommand(),
		newTemplateCommand(),
		newRestartCommand(),
		newSudoersCommand(),
		newStartAtLoginCommand(),
		newNetworkCommand(),
		newCloneCommand(),
		newRenameCommand(),
	)
	addPluginCommands(rootCmd)

	return rootCmd
}

func addPluginCommands(rootCmd *cobra.Command) {
	// The global options are only processed when rootCmd.Execute() is called.
	// Let's take a sneak peek here to help debug the plugin discovery code.
	if len(os.Args) > 1 && os.Args[1] == "--debug" {
		logrus.SetLevel(logrus.DebugLevel)
	}

	allPlugins, err := plugins.Discover()
	if err != nil {
		logrus.Warnf("Failed to discover plugins: %v", err)
		return
	}

	for _, plugin := range allPlugins {
		pluginCmd := &cobra.Command{
			Use:                plugin.Name,
			Short:              plugin.Description,
			GroupID:            "plugin",
			DisableFlagParsing: true,
			SilenceErrors:      true,
			SilenceUsage:       true,
			PreRunE: func(*cobra.Command, []string) error {
				for i, arg := range os.Args {
					if arg == plugin.Name {
						// parse global options but ignore plugin options
						err := rootCmd.ParseFlags(os.Args[1:i])
						if err == nil {
							err = processGlobalFlags(rootCmd)
						}
						return err
					}
				}
				// unreachable
				return nil
			},
			Run: func(cmd *cobra.Command, _ []string) {
				for i, arg := range os.Args {
					if arg == plugin.Name {
						// ignore global options
						plugin.Run(cmd.Context(), os.Args[i+1:])
						// plugin.Run() never returns because it calls os.Exit()
					}
				}
				// unreachable
			},
		}
		// Don't show the url scheme helper in the help output.
		if strings.HasPrefix(plugin.Name, "url-") {
			pluginCmd.Hidden = true
		}
		rootCmd.AddCommand(pluginCmd)
	}
}

// WrapArgsError annotates cobra args error with some context, so the error message is more user-friendly.
func WrapArgsError(argFn cobra.PositionalArgs) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		err := argFn(cmd, args)
		if err == nil {
			return nil
		}

		return fmt.Errorf("%q %s.\nSee '%s --help'.\n\nUsage:  %s\n\n%s",
			cmd.CommandPath(), err.Error(),
			cmd.CommandPath(),
			cmd.UseLine(), cmd.Short,
		)
	}
}
