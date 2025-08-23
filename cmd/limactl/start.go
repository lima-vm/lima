// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/lima-vm/lima/v2/cmd/limactl/editflags"
	"github.com/lima-vm/lima/v2/pkg/driverutil"
	"github.com/lima-vm/lima/v2/pkg/editutil"
	"github.com/lima-vm/lima/v2/pkg/instance"
	"github.com/lima-vm/lima/v2/pkg/limatmpl"
	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limatype/dirnames"
	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
	"github.com/lima-vm/lima/v2/pkg/limayaml"
	networks "github.com/lima-vm/lima/v2/pkg/networks/reconcile"
	"github.com/lima-vm/lima/v2/pkg/registry"
	"github.com/lima-vm/lima/v2/pkg/store"
	"github.com/lima-vm/lima/v2/pkg/templatestore"
	"github.com/lima-vm/lima/v2/pkg/uiutil"
	"github.com/lima-vm/lima/v2/pkg/yqutil"
)

func registerCreateFlags(cmd *cobra.Command, commentPrefix string) {
	flags := cmd.Flags()
	flags.String("name", "", commentPrefix+"Override the instance name")
	flags.Bool("list-templates", false, commentPrefix+"List available templates and exit")
	flags.Bool("list-drivers", false, commentPrefix+"List available drivers and exit")
	editflags.RegisterCreate(cmd, commentPrefix)
}

func newCreateCommand() *cobra.Command {
	createCommand := &cobra.Command{
		Use: "create FILE.yaml|URL",
		Example: `
  To create an instance "default" from the default Ubuntu template:
  $ limactl create

  To create an instance "default" from a template "docker":
  $ limactl create --name=default template://docker

  To create an instance "default" with modified parameters:
  $ limactl create --cpus=2 --memory=2

  To create an instance "default" with yq expressions:
  $ limactl create --set='.cpus = 2 | .memory = "2GiB"'

  To see the template list:
  $ limactl create --list-templates

  To create an instance "default" from a local file:
  $ limactl create --name=default /usr/local/share/lima/templates/fedora.yaml

  To create an instance "default" from a remote URL (use carefully, with a trustable source):
  $ limactl create --name=default https://raw.githubusercontent.com/lima-vm/lima/master/templates/alpine.yaml

  To create an instance "local" from a template passed to stdin (--name parameter is required):
  $ cat template.yaml | limactl create --name=local -
`,
		Short:             "Create an instance of Lima",
		Args:              WrapArgsError(cobra.MaximumNArgs(1)),
		ValidArgsFunction: createBashComplete,
		RunE:              createAction,
		GroupID:           basicCommand,
	}
	registerCreateFlags(createCommand, "")
	return createCommand
}

func newStartCommand() *cobra.Command {
	startCommand := &cobra.Command{
		Use: "start NAME|FILE.yaml|URL",
		Example: `
  To create an instance "default" (if not created yet) from the default Ubuntu template, and start it:
  $ limactl start

  To create an instance "default" from a template "docker", and start it:
  $ limactl start --name=default template://docker
`,
		Short:             "Start an instance of Lima",
		Args:              WrapArgsError(cobra.MaximumNArgs(1)),
		ValidArgsFunction: startBashComplete,
		RunE:              startAction,
		GroupID:           basicCommand,
	}
	registerCreateFlags(startCommand, "[limactl create] ")
	if runtime.GOOS != "windows" {
		startCommand.Flags().Bool("foreground", false, "Run the hostagent in the foreground")
	}
	startCommand.Flags().Duration("timeout", instance.DefaultWatchHostAgentEventsTimeout, "Duration to wait for the instance to be running before timing out")
	startCommand.Flags().Bool("progress", false, "Show provision script progress by tailing cloud-init logs")
	startCommand.SetHelpFunc(func(cmd *cobra.Command, _ []string) {
		printCommandSummary(cmd)

		allFlags, createFlags := collectFlags(cmd)
		printFlags(allFlags, createFlags)

		printGlobalFlags(cmd)
	})

	return startCommand
}

func printCommandSummary(cmd *cobra.Command) {
	fmt.Fprintf(cmd.OutOrStdout(), "%s\n\n", cmd.Short)
	fmt.Fprintf(cmd.OutOrStdout(), "Usage:\n  %s\n\n", cmd.UseLine())

	if cmd.Example != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "Examples:\n%s\n\n", cmd.Example)
	}
}

func getFlagType(flag *pflag.Flag) string {
	switch flag.Value.Type() {
	case "bool":
		return ""
	case "string":
		return "string"
	case "int":
		return "int"
	case "duration":
		return "duration"
	case "stringSlice", "stringArray":
		return "strings"
	case "ipSlice":
		return "ipSlice"
	case "uint16":
		return "uint16"
	case "float32":
		return "float32"
	default:
		return flag.Value.Type()
	}
}

func formatFlag(flag *pflag.Flag) (flagName, shorthand string) {
	flagName = "--" + flag.Name

	if flag.Shorthand != "" {
		shorthand = "-" + flag.Shorthand
	}

	flagType := getFlagType(flag)
	if flagType != "" {
		flagName += " " + flagType
	}

	return flagName, shorthand
}

func collectFlags(cmd *cobra.Command) (allFlags, createFlags []string) {
	cmd.LocalFlags().VisitAll(func(flag *pflag.Flag) {
		flagName, shorthand := formatFlag(flag)
		flagUsage := flag.Usage

		var formattedFlag string
		if shorthand != "" {
			formattedFlag = fmt.Sprintf("  %s, %s", shorthand, flagName)
		} else {
			formattedFlag = fmt.Sprintf("      %s", flagName)
		}

		if strings.HasPrefix(flagUsage, "[limactl create]") {
			cleanUsage := strings.TrimPrefix(flagUsage, "[limactl create] ")
			createFlags = append(createFlags, fmt.Sprintf("%-25s %s", formattedFlag, cleanUsage))
		} else {
			allFlags = append(allFlags, fmt.Sprintf("%-25s %s", formattedFlag, flagUsage))
		}
	})
	return allFlags, createFlags
}

func printFlags(allFlags, createFlags []string) {
	if len(allFlags) > 0 {
		fmt.Fprint(os.Stdout, "Flags:\n")
		for _, flag := range allFlags {
			fmt.Fprintln(os.Stdout, flag)
		}
		fmt.Fprint(os.Stdout, "\n")
	}

	if len(createFlags) > 0 {
		fmt.Fprint(os.Stdout, "Flags inherited from `limactl create`:\n")
		for _, flag := range createFlags {
			fmt.Fprintln(os.Stdout, flag)
		}
		fmt.Fprint(os.Stdout, "\n")
	}
}

func printGlobalFlags(cmd *cobra.Command) {
	if cmd.HasAvailableInheritedFlags() {
		fmt.Fprintf(cmd.OutOrStdout(), "Global Flags:\n%s", cmd.InheritedFlags().FlagUsages())
	}
}

func loadOrCreateInstance(cmd *cobra.Command, args []string, createOnly bool) (*limatype.Instance, error) {
	ctx := cmd.Context()
	var arg string // can be empty
	if len(args) > 0 {
		arg = args[0]
	}

	flags := cmd.Flags()

	// Create an instance, with menu TUI when TTY is available
	tty, err := flags.GetBool("tty")
	if err != nil {
		return nil, err
	}

	name, err := flags.GetString("name")
	if err != nil {
		return nil, err
	}
	if name != "" {
		err := dirnames.ValidateInstName(name)
		if err != nil {
			return nil, err
		}
	}
	if isTemplateURL, templateName := limatmpl.SeemsTemplateURL(arg); isTemplateURL {
		switch templateName {
		case "experimental/vz":
			logrus.Warn("template://experimental/vz was merged into the default template in Lima v1.0. See also <https://lima-vm.io/docs/config/vmtype/>.")
		case "experimental/riscv64":
			logrus.Warn("template://experimental/riscv64 was merged into the default template in Lima v1.0. Use `limactl create --arch=riscv64 template://default` instead.")
		case "experimental/armv7l":
			logrus.Warn("template://experimental/armv7l was merged into the default template in Lima v1.0. Use `limactl create --arch=armv7l template://default` instead.")
		case "vmnet":
			logrus.Warn("template://vmnet was removed in Lima v1.0. Use `limactl create --network=lima:shared template://default` instead. See also <https://lima-vm.io/docs/config/network/>.")
		case "experimental/net-user-v2":
			logrus.Warn("template://experimental/net-user-v2 was removed in Lima v1.0. Use `limactl create --network=lima:user-v2 template://default` instead. See also <https://lima-vm.io/docs/config/network/>.")
		case "experimental/9p":
			logrus.Warn("template://experimental/9p was removed in Lima v1.0. Use `limactl create --vm-type=qemu --mount-type=9p template://default` instead. See also <https://lima-vm.io/docs/config/mount/>.")
		case "experimental/virtiofs-linux":
			logrus.Warn("template://experimental/virtiofs-linux was removed in Lima v1.0. Use `limactl create --mount-type=virtiofs template://default` instead. See also <https://lima-vm.io/docs/config/mount/>.")
		}
	}
	if arg == "-" {
		if name == "" {
			return nil, errors.New("must pass instance name with --name when reading template from stdin")
		}
		// see if the tty was set explicitly or not
		ttySet := cmd.Flags().Changed("tty")
		if ttySet && tty {
			return nil, errors.New("cannot use --tty=true when reading template from stdin")
		}
		tty = false
	}
	var tmpl *limatmpl.Template
	if err := dirnames.ValidateInstName(arg); arg == "" || err == nil {
		tmpl = &limatmpl.Template{Name: name}
		if arg == "" {
			if name == "" {
				tmpl.Name = DefaultInstanceName
			}
		} else {
			logrus.Debugf("interpreting argument %q as an instance name", arg)
			if name != "" && name != arg {
				return nil, fmt.Errorf("instance name %q and CLI flag --name=%q cannot be specified together", arg, tmpl.Name)
			}
			tmpl.Name = arg
		}
		// store.Inspect() will validate the template name (in case it has been set to arg)
		inst, err := store.Inspect(ctx, tmpl.Name)
		if err == nil {
			if createOnly {
				return nil, fmt.Errorf("instance %q already exists", tmpl.Name)
			}
			logrus.Infof("Using the existing instance %q", tmpl.Name)
			yqExprs, err := editflags.YQExpressions(flags, false)
			if err != nil {
				return nil, err
			}
			if len(yqExprs) > 0 {
				yq := yqutil.Join(yqExprs)
				inst, err = applyYQExpressionToExistingInstance(ctx, inst, yq)
				if err != nil {
					return nil, fmt.Errorf("failed to apply yq expression %q to instance %q: %w", yq, tmpl.Name, err)
				}
			}
			return inst, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		if arg != "" && arg != DefaultInstanceName {
			logrus.Infof("Creating an instance %q from template://default (Not from template://%s)", tmpl.Name, tmpl.Name)
			logrus.Warnf("This form is deprecated. Use `limactl create --name=%s template://default` instead", tmpl.Name)
		}
		// Read the default template for creating a new instance
		tmpl.Bytes, err = templatestore.Read(templatestore.Default)
		if err != nil {
			return nil, err
		}
	} else {
		tmpl, err = limatmpl.Read(cmd.Context(), name, arg)
		if err != nil {
			return nil, err
		}
		if createOnly {
			// store.Inspect() will also validate the instance name
			if _, err := store.Inspect(ctx, tmpl.Name); err == nil {
				return nil, fmt.Errorf("instance %q already exists", tmpl.Name)
			}
		} else if err := dirnames.ValidateInstName(tmpl.Name); err != nil {
			return nil, err
		}
	}

	if err := tmpl.Embed(cmd.Context(), true, true); err != nil {
		return nil, err
	}
	yqExprs, err := editflags.YQExpressions(flags, true)
	if err != nil {
		return nil, err
	}
	yq := yqutil.Join(yqExprs)
	if tty {
		var err error
		tmpl, err = chooseNextCreatorState(cmd.Context(), tmpl, yq)
		if err != nil {
			return nil, err
		}
	} else {
		logrus.Info("Terminal is not available, proceeding without opening an editor")
		if err := modifyInPlace(tmpl, yq); err != nil {
			return nil, err
		}
	}
	saveBrokenYAML := tty
	return instance.Create(cmd.Context(), tmpl.Name, tmpl.Bytes, saveBrokenYAML)
}

func applyYQExpressionToExistingInstance(ctx context.Context, inst *limatype.Instance, yq string) (*limatype.Instance, error) {
	if strings.TrimSpace(yq) == "" {
		return inst, nil
	}
	filePath := filepath.Join(inst.Dir, filenames.LimaYAML)
	yContent, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	logrus.Debugf("Applying yq expression %q to an existing instance %q", yq, inst.Name)
	yBytes, err := yqutil.EvaluateExpression(yq, yContent)
	if err != nil {
		return nil, err
	}
	y, err := limayaml.Load(ctx, yBytes, filePath)
	if err != nil {
		return nil, err
	}
	if err := driverutil.ResolveVMType(y, filePath); err != nil {
		return nil, fmt.Errorf("failed to accept config for %q: %w", filePath, err)
	}
	if err := limayaml.Validate(y, true); err != nil {
		rejectedYAML := "lima.REJECTED.yaml"
		if writeErr := os.WriteFile(rejectedYAML, yBytes, 0o644); writeErr != nil {
			return nil, fmt.Errorf("the YAML is invalid, attempted to save the buffer as %q but failed: %w: %w", rejectedYAML, writeErr, err)
		}
		// TODO: may need to support editing the rejected YAML
		return nil, fmt.Errorf("the YAML is invalid, saved the buffer as %q: %w", rejectedYAML, err)
	}
	if err := os.WriteFile(filePath, yBytes, 0o644); err != nil {
		return nil, err
	}
	// Reload
	return store.Inspect(ctx, inst.Name)
}

func modifyInPlace(st *limatmpl.Template, yq string) error {
	out, err := yqutil.EvaluateExpression(yq, st.Bytes)
	if err != nil {
		return err
	}
	st.Bytes = out
	return nil
}

// exitSuccessError is an error that indicates a successful exit.
type exitSuccessError struct {
	Msg string
}

// Error implements error.
func (e exitSuccessError) Error() string {
	return e.Msg
}

// ExitCode implements ExitCoder.
func (exitSuccessError) ExitCode() int {
	return 0
}

func chooseNextCreatorState(ctx context.Context, tmpl *limatmpl.Template, yq string) (*limatmpl.Template, error) {
	for {
		if err := modifyInPlace(tmpl, yq); err != nil {
			logrus.WithError(err).Warn("Failed to evaluate yq expression")
			return tmpl, err
		}
		message := fmt.Sprintf("Creating an instance %q", tmpl.Name)
		options := []string{
			"Proceed with the current configuration",
			"Open an editor to review or modify the current configuration",
			"Choose another template (docker, podman, archlinux, fedora, ...)",
			"Exit",
		}
		ans, err := uiutil.Select(message, options)
		if err != nil {
			if errors.Is(err, uiutil.InterruptErr) {
				logrus.Fatal("Interrupted by user")
			}
			logrus.WithError(err).Warn("Failed to open TUI")
			return tmpl, nil
		}
		switch ans {
		case 0: // "Proceed with the current configuration"
			return tmpl, nil
		case 1: // "Open an editor ..."
			hdr := fmt.Sprintf("# Review and modify the following configuration for Lima instance %q.\n", tmpl.Name)
			if tmpl.Name == DefaultInstanceName {
				hdr += "# - In most cases, you do not need to modify this file.\n"
			}
			hdr += "# - To cancel starting Lima, just save this file as an empty file.\n"
			hdr += "\n"
			hdr += editutil.GenerateEditorWarningHeader()
			var err error
			tmpl.Bytes, err = editutil.OpenEditor(ctx, tmpl.Bytes, hdr)
			tmpl.Config = nil
			if err != nil {
				return tmpl, err
			}
			if len(tmpl.Bytes) == 0 {
				const msg = "Aborting, as requested by saving the file with empty content"
				logrus.Info(msg)
				return nil, exitSuccessError{Msg: msg}
			}
			err = tmpl.Embed(ctx, true, true)
			if err != nil {
				return nil, err
			}
			return tmpl, nil
		case 2: // "Choose another template..."
			templates, err := filterHiddenTemplates()
			if err != nil {
				return tmpl, err
			}
			message := "Choose a template"
			options := make([]string, len(templates))
			for i := range templates {
				options[i] = templates[i].Name
			}
			ansEx, err := uiutil.Select(message, options)
			if err != nil {
				return tmpl, err
			}
			if ansEx > len(templates)-1 {
				return tmpl, fmt.Errorf("invalid answer %d for %d entries", ansEx, len(templates))
			}
			yamlPath := templates[ansEx].Location
			if tmpl.Name == "" {
				tmpl.Name, err = limatmpl.InstNameFromYAMLPath(yamlPath)
				if err != nil {
					return nil, err
				}
			}
			tmpl, err = limatmpl.Read(ctx, tmpl.Name, yamlPath)
			if err != nil {
				return nil, err
			}
			err = tmpl.Embed(ctx, true, true)
			if err != nil {
				return nil, err
			}
			continue
		case 3: // "Exit"
			return nil, exitSuccessError{Msg: "Choosing to exit"}
		default:
			return tmpl, fmt.Errorf("unexpected answer %q", ans)
		}
	}
}

// createStartActionCommon is shared by createAction and startAction.
func createStartActionCommon(cmd *cobra.Command, _ []string) (exit bool, err error) {
	if listTemplates, err := cmd.Flags().GetBool("list-templates"); err != nil {
		return true, err
	} else if listTemplates {
		templates, err := filterHiddenTemplates()
		if err != nil {
			return true, err
		}
		w := cmd.OutOrStdout()
		for _, f := range templates {
			_, _ = fmt.Fprintln(w, f.Name)
		}
		return true, nil
	} else if listDrivers, err := cmd.Flags().GetBool("list-drivers"); err != nil {
		return true, err
	} else if listDrivers {
		w := cmd.OutOrStdout()
		for k := range registry.List() {
			_, _ = fmt.Fprintln(w, k)
		}
		return true, nil
	}
	return false, nil
}

func filterHiddenTemplates() ([]templatestore.Template, error) {
	templates, err := templatestore.Templates()
	if err != nil {
		return nil, err
	}
	var filtered []templatestore.Template
	for _, f := range templates {
		// Don't show internal base templates like `_default/*` and `_images/*`.
		if !strings.HasPrefix(f.Name, "_") {
			filtered = append(filtered, f)
		}
	}
	return filtered, nil
}

func createAction(cmd *cobra.Command, args []string) error {
	if exit, err := createStartActionCommon(cmd, args); err != nil {
		return err
	} else if exit {
		return nil
	}
	inst, err := loadOrCreateInstance(cmd, args, true)
	if err != nil {
		return err
	}
	if len(inst.Errors) > 0 {
		return fmt.Errorf("errors inspecting instance: %+v", inst.Errors)
	}
	if _, err = instance.Prepare(cmd.Context(), inst); err != nil {
		return err
	}
	logrus.Infof("Run `limactl start %s` to start the instance.", inst.Name)
	return nil
}

func startAction(cmd *cobra.Command, args []string) error {
	if exit, err := createStartActionCommon(cmd, args); err != nil {
		return err
	} else if exit {
		return nil
	}
	inst, err := loadOrCreateInstance(cmd, args, false)
	if err != nil {
		return err
	}
	if len(inst.Errors) > 0 {
		return fmt.Errorf("errors inspecting instance: %+v", inst.Errors)
	}
	switch inst.Status {
	case limatype.StatusRunning:
		logrus.Infof("The instance %q is already running. Run `%s` to open the shell.",
			inst.Name, instance.LimactlShellCmd(inst.Name))
		// Not an error
		return nil
	case limatype.StatusStopped:
		// NOP
	default:
		logrus.Warnf("expected status %q, got %q", limatype.StatusStopped, inst.Status)
	}
	ctx := cmd.Context()
	err = networks.Reconcile(ctx, inst.Name)
	if err != nil {
		return err
	}

	launchHostAgentForeground := false
	if runtime.GOOS != "windows" {
		foreground, err := cmd.Flags().GetBool("foreground")
		if err != nil {
			return err
		}
		launchHostAgentForeground = foreground
	}
	timeout, err := cmd.Flags().GetDuration("timeout")
	if err != nil {
		return err
	}
	if timeout > 0 {
		ctx = instance.WithWatchHostAgentTimeout(ctx, timeout)
	}

	progress, err := cmd.Flags().GetBool("progress")
	if err != nil {
		return err
	}

	return instance.Start(ctx, inst, "", launchHostAgentForeground, progress)
}

func createBashComplete(cmd *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return bashCompleteTemplateNames(cmd, toComplete)
}

func startBashComplete(cmd *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	compInst, _ := bashCompleteInstanceNames(cmd)
	compTmpl, _ := bashCompleteTemplateNames(cmd, toComplete)
	return append(compInst, compTmpl...), cobra.ShellCompDirectiveDefault
}
