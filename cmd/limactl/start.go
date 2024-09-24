package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/containerd/containerd/identifiers"
	"github.com/lima-vm/lima/cmd/limactl/editflags"
	"github.com/lima-vm/lima/cmd/limactl/guessarg"
	"github.com/lima-vm/lima/pkg/editutil"
	"github.com/lima-vm/lima/pkg/instance"
	"github.com/lima-vm/lima/pkg/ioutilx"
	"github.com/lima-vm/lima/pkg/limayaml"
	networks "github.com/lima-vm/lima/pkg/networks/reconcile"
	"github.com/lima-vm/lima/pkg/store"
	"github.com/lima-vm/lima/pkg/store/filenames"
	"github.com/lima-vm/lima/pkg/templatestore"
	"github.com/lima-vm/lima/pkg/uiutil"
	"github.com/lima-vm/lima/pkg/yqutil"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func registerCreateFlags(cmd *cobra.Command, commentPrefix string) {
	flags := cmd.Flags()
	flags.String("name", "", commentPrefix+"override the instance name")
	flags.Bool("list-templates", false, commentPrefix+"list available templates and exit")
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
$ limactl create --name=default https://raw.githubusercontent.com/lima-vm/lima/master/examples/alpine.yaml

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

'limactl start' also accepts the 'limactl create' flags such as '--set'.
See the examples in 'limactl create --help'.
`,
		Short:             "Start an instance of Lima",
		Args:              WrapArgsError(cobra.MaximumNArgs(1)),
		ValidArgsFunction: startBashComplete,
		RunE:              startAction,
		GroupID:           basicCommand,
	}
	registerCreateFlags(startCommand, "[limactl create] ")
	if runtime.GOOS != "windows" {
		startCommand.Flags().Bool("foreground", false, "run the hostagent in the foreground")
	}
	startCommand.Flags().Duration("timeout", instance.DefaultWatchHostAgentEventsTimeout, "duration to wait for the instance to be running before timing out")
	return startCommand
}

func loadOrCreateInstance(cmd *cobra.Command, args []string, createOnly bool) (*store.Instance, error) {
	var arg string // can be empty
	if len(args) > 0 {
		arg = args[0]
	}

	var (
		st  = &creatorState{}
		err error
	)

	flags := cmd.Flags()

	// Create an instance, with menu TUI when TTY is available
	tty, err := flags.GetBool("tty")
	if err != nil {
		return nil, err
	}

	st.instName, err = flags.GetString("name")
	if err != nil {
		return nil, err
	}

	const yBytesLimit = 4 * 1024 * 1024 // 4MiB

	isTemplateURL, templateURL := guessarg.SeemsTemplateURL(arg)
	switch {
	case isTemplateURL:
		// No need to use SecureJoin here. https://github.com/lima-vm/lima/pull/805#discussion_r853411702
		templateName := filepath.Join(templateURL.Host, templateURL.Path)
		logrus.Debugf("interpreting argument %q as a template name %q", arg, templateName)
		if st.instName == "" {
			// e.g., templateName = "deprecated/centos-7" , st.instName = "centos-7"
			st.instName = filepath.Base(templateName)
		}
		st.yBytes, err = templatestore.Read(templateName)
		if err != nil {
			return nil, err
		}
	case guessarg.SeemsHTTPURL(arg):
		if st.instName == "" {
			st.instName, err = guessarg.InstNameFromURL(arg)
			if err != nil {
				return nil, err
			}
		}
		logrus.Debugf("interpreting argument %q as a http url for instance %q", arg, st.instName)
		req, err := http.NewRequestWithContext(cmd.Context(), http.MethodGet, arg, http.NoBody)
		if err != nil {
			return nil, err
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		st.yBytes, err = ioutilx.ReadAtMaximum(resp.Body, yBytesLimit)
		if err != nil {
			return nil, err
		}
	case guessarg.SeemsFileURL(arg):
		if st.instName == "" {
			st.instName, err = guessarg.InstNameFromURL(arg)
			if err != nil {
				return nil, err
			}
		}
		logrus.Debugf("interpreting argument %q as a file url for instance %q", arg, st.instName)
		r, err := os.Open(strings.TrimPrefix(arg, "file://"))
		if err != nil {
			return nil, err
		}
		defer r.Close()
		st.yBytes, err = ioutilx.ReadAtMaximum(r, yBytesLimit)
		if err != nil {
			return nil, err
		}
	case guessarg.SeemsYAMLPath(arg):
		if st.instName == "" {
			st.instName, err = guessarg.InstNameFromYAMLPath(arg)
			if err != nil {
				return nil, err
			}
		}
		logrus.Debugf("interpreting argument %q as a file path for instance %q", arg, st.instName)
		r, err := os.Open(arg)
		if err != nil {
			return nil, err
		}
		defer r.Close()
		st.yBytes, err = ioutilx.ReadAtMaximum(r, yBytesLimit)
		if err != nil {
			return nil, err
		}
	case arg == "-":
		if st.instName == "" {
			return nil, errors.New("must pass instance name with --name when reading template from stdin")
		}
		st.yBytes, err = io.ReadAll(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("unexpected error reading stdin: %w", err)
		}
		// see if the tty was set explicitly or not
		ttySet := cmd.Flags().Changed("tty")
		if ttySet && tty {
			return nil, errors.New("cannot use --tty=true and read template from stdin together")
		}
		tty = false
	default:
		if arg == "" {
			if st.instName == "" {
				st.instName = DefaultInstanceName
			}
		} else {
			logrus.Debugf("interpreting argument %q as an instance name", arg)
			if st.instName != "" && st.instName != arg {
				return nil, fmt.Errorf("instance name %q and CLI flag --name=%q cannot be specified together", arg, st.instName)
			}
			st.instName = arg
		}
		if err := identifiers.Validate(st.instName); err != nil {
			return nil, fmt.Errorf("argument must be either an instance name, a YAML file path, or a URL, got %q: %w", st.instName, err)
		}
		inst, err := store.Inspect(st.instName)
		if err == nil {
			if createOnly {
				return nil, fmt.Errorf("instance %q already exists", st.instName)
			}
			logrus.Infof("Using the existing instance %q", st.instName)
			yqExprs, err := editflags.YQExpressions(flags, false)
			if err != nil {
				return nil, err
			}
			if len(yqExprs) > 0 {
				yq := yqutil.Join(yqExprs)
				inst, err = applyYQExpressionToExistingInstance(inst, yq)
				if err != nil {
					return nil, fmt.Errorf("failed to apply yq expression %q to instance %q: %w", yq, st.instName, err)
				}
			}
			return inst, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		if arg != "" && arg != DefaultInstanceName {
			logrus.Infof("Creating an instance %q from template://default (Not from template://%s)", st.instName, st.instName)
			logrus.Warnf("This form is deprecated. Use `limactl create --name=%s template://default` instead", st.instName)
		}
		// Read the default template for creating a new instance
		st.yBytes, err = templatestore.Read(templatestore.Default)
		if err != nil {
			return nil, err
		}
	}

	yqExprs, err := editflags.YQExpressions(flags, true)
	if err != nil {
		return nil, err
	}
	yq := yqutil.Join(yqExprs)
	if tty {
		var err error
		st, err = chooseNextCreatorState(st, yq)
		if err != nil {
			return nil, err
		}
	} else {
		logrus.Info("Terminal is not available, proceeding without opening an editor")
		if err := modifyInPlace(st, yq); err != nil {
			return nil, err
		}
	}
	saveBrokenYAML := tty
	return instance.Create(cmd.Context(), st.instName, st.yBytes, saveBrokenYAML)
}

func applyYQExpressionToExistingInstance(inst *store.Instance, yq string) (*store.Instance, error) {
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
	y, err := limayaml.Load(yBytes, filePath)
	if err != nil {
		return nil, err
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
	return store.Inspect(inst.Name)
}

type creatorState struct {
	instName string // instance name
	yBytes   []byte // yaml bytes
}

func modifyInPlace(st *creatorState, yq string) error {
	out, err := yqutil.EvaluateExpression(yq, st.yBytes)
	if err != nil {
		return err
	}
	st.yBytes = out
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

func chooseNextCreatorState(st *creatorState, yq string) (*creatorState, error) {
	for {
		if err := modifyInPlace(st, yq); err != nil {
			logrus.WithError(err).Warn("Failed to evaluate yq expression")
			return st, err
		}
		message := fmt.Sprintf("Creating an instance %q", st.instName)
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
			return st, nil
		}
		switch ans {
		case 0: // "Proceed with the current configuration"
			return st, nil
		case 1: // "Open an editor ..."
			hdr := fmt.Sprintf("# Review and modify the following configuration for Lima instance %q.\n", st.instName)
			if st.instName == DefaultInstanceName {
				hdr += "# - In most cases, you do not need to modify this file.\n"
			}
			hdr += "# - To cancel starting Lima, just save this file as an empty file.\n"
			hdr += "\n"
			hdr += editutil.GenerateEditorWarningHeader()
			var err error
			st.yBytes, err = editutil.OpenEditor(st.yBytes, hdr)
			if err != nil {
				return st, err
			}
			if len(st.yBytes) == 0 {
				const msg = "Aborting, as requested by saving the file with empty content"
				logrus.Info(msg)
				return nil, exitSuccessError{Msg: msg}
			}
			return st, nil
		case 2: // "Choose another template..."
			templates, err := templatestore.Templates()
			if err != nil {
				return st, err
			}
			message := "Choose a template"
			options := make([]string, len(templates))
			for i := range templates {
				options[i] = templates[i].Name
			}
			ansEx, err := uiutil.Select(message, options)
			if err != nil {
				return st, err
			}
			if ansEx > len(templates)-1 {
				return st, fmt.Errorf("invalid answer %d for %d entries", ansEx, len(templates))
			}
			yamlPath := templates[ansEx].Location
			if st.instName == "" {
				st.instName, err = guessarg.InstNameFromYAMLPath(yamlPath)
				if err != nil {
					return nil, err
				}
			}
			st.yBytes, err = os.ReadFile(yamlPath)
			if err != nil {
				return nil, err
			}
			continue
		case 3: // "Exit"
			return nil, exitSuccessError{Msg: "Choosing to exit"}
		default:
			return st, fmt.Errorf("unexpected answer %q", ans)
		}
	}
}

// createStartActionCommon is shared by createAction and startAction.
func createStartActionCommon(cmd *cobra.Command, _ []string) (exit bool, err error) {
	if listTemplates, err := cmd.Flags().GetBool("list-templates"); err != nil {
		return true, err
	} else if listTemplates {
		if templates, err := templatestore.Templates(); err == nil {
			w := cmd.OutOrStdout()
			for _, f := range templates {
				fmt.Fprintln(w, f.Name)
			}
			return true, nil
		}
	}
	return false, nil
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
	case store.StatusRunning:
		logrus.Infof("The instance %q is already running. Run `%s` to open the shell.",
			inst.Name, instance.LimactlShellCmd(inst.Name))
		// Not an error
		return nil
	case store.StatusStopped:
		// NOP
	default:
		logrus.Warnf("expected status %q, got %q", store.StatusStopped, inst.Status)
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

	return instance.Start(ctx, inst, "", launchHostAgentForeground)
}

func createBashComplete(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return bashCompleteTemplateNames(cmd)
}

func startBashComplete(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	compInst, _ := bashCompleteInstanceNames(cmd)
	compTmpl, _ := bashCompleteTemplateNames(cmd)
	return append(compInst, compTmpl...), cobra.ShellCompDirectiveDefault
}
