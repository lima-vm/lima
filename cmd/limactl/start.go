package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/containerd/containerd/identifiers"
	"github.com/lima-vm/lima/pkg/limayaml"
	"github.com/lima-vm/lima/pkg/osutil"
	"github.com/lima-vm/lima/pkg/start"
	"github.com/lima-vm/lima/pkg/store"
	"github.com/lima-vm/lima/pkg/store/filenames"
	"github.com/mattn/go-isatty"
	"github.com/norouter/norouter/cmd/norouter/editorcmd"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func newStartCommand() *cobra.Command {
	var startCommand = &cobra.Command{
		Use:               "start NAME|FILE.yaml",
		Short:             fmt.Sprintf("Start an instance of Lima. If the instance does not exist, open an editor for creating new one, with name %q", DefaultInstanceName),
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: startBashComplete,
		RunE:              startAction,
	}
	startCommand.Flags().Bool("tty", isatty.IsTerminal(os.Stdout.Fd()), "enable TUI interactions such as opening an editor, defaults to true when stdout is a terminal")
	return startCommand
}

func loadOrCreateInstance(cmd *cobra.Command, args []string) (*store.Instance, error) {
	var arg string
	if len(args) == 0 {
		arg = DefaultInstanceName
	} else {
		arg = args[0]
	}

	var (
		instName string
		yBytes   = limayaml.DefaultTemplate
		err      error
	)

	if argSeemsYAMLPath(arg) {
		instName, err = instNameFromYAMLPath(arg)
		if err != nil {
			return nil, err
		}
		logrus.Debugf("interpreting argument %q as a file path for instance %q", arg, instName)
		yBytes, err = os.ReadFile(arg)
		if err != nil {
			return nil, err
		}
	} else {
		instName = arg
		logrus.Debugf("interpreting argument %q as an instance name %q", arg, instName)
		if err := identifiers.Validate(instName); err != nil {
			return nil, fmt.Errorf("argument must be either an instance name or a YAML file path, got %q: %w", instName, err)
		}
		if inst, err := store.Inspect(instName); err == nil {
			logrus.Infof("Using the existing instance %q", instName)
			return inst, nil
		} else {
			if !errors.Is(err, os.ErrNotExist) {
				return nil, err
			}
		}
	}
	// create a new instance from the template
	instDir, err := store.InstanceDir(instName)
	if err != nil {
		return nil, err
	}

	// the full path of the socket name must be less than UNIX_PATH_MAX chars.
	maxSockName := filepath.Join(instDir, filenames.LongestSock)
	if len(maxSockName) >= osutil.UnixPathMax {
		return nil, fmt.Errorf("instance name %q too long: %q must be less than UNIX_PATH_MAX=%d characers, but is %d",
			instName, maxSockName, osutil.UnixPathMax, len(maxSockName))
	}
	if _, err := os.Stat(instDir); !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("instance %q already exists (%q)", instName, instDir)
	}

	tty, err := cmd.Flags().GetBool("tty")
	if err != nil {
		return nil, err
	}
	if tty {
		answerOpenEditor, err := askWhetherToOpenEditor(instName)
		if err != nil {
			logrus.WithError(err).Warn("Failed to open TUI")
			answerOpenEditor = false
		}
		if answerOpenEditor {
			yBytes, err = openEditor(cmd, instName, yBytes)
			if err != nil {
				return nil, err
			}
			if len(yBytes) == 0 {
				logrus.Info("Aborting, as requested by saving the file with empty content")
				os.Exit(0)
				return nil, errors.New("should not reach here")
			}
		}
	} else {
		logrus.Info("Terminal is not available, proceeding without opening an editor")
	}
	// limayaml.Load() needs to pass the store file path to limayaml.FillDefault() to calculate default MAC addresses
	filePath := filepath.Join(instDir, filenames.LimaYAML)
	y, err := limayaml.Load(yBytes, filePath)
	if err != nil {
		return nil, err
	}
	if err := limayaml.Validate(*y); err != nil {
		if !tty {
			return nil, err
		}
		rejectedYAML := "lima.REJECTED.yaml"
		if writeErr := os.WriteFile(rejectedYAML, yBytes, 0644); writeErr != nil {
			return nil, fmt.Errorf("the YAML is invalid, attempted to save the buffer as %q but failed: %v: %w", rejectedYAML, writeErr, err)
		}
		return nil, fmt.Errorf("the YAML is invalid, saved the buffer as %q: %w", rejectedYAML, err)
	}
	if err := os.MkdirAll(instDir, 0700); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filePath, yBytes, 0644); err != nil {
		return nil, err
	}
	return store.Inspect(instName)
}

func askWhetherToOpenEditor(name string) (bool, error) {
	var ans string
	prompt := &survey.Select{
		Message: fmt.Sprintf("Creating an instance %q", name),
		Options: []string{
			"Proceed with the default configuration",
			"Open an editor to override the configuration",
			"Exit",
		},
	}
	if err := survey.AskOne(prompt, &ans); err != nil {
		return false, err
	}
	switch ans {
	case prompt.Options[0]:
		return false, nil
	case prompt.Options[1]:
		return true, nil
	case prompt.Options[2]:
		os.Exit(0)
		return false, errors.New("should not reach here")
	default:
		return false, fmt.Errorf("unexpected answer %q", ans)
	}
}

// openEditor opens an editor, and returns the content (not path) of the modified yaml.
//
// openEditor returns nil when the file was saved as an empty file, optionally with whitespaces.
func openEditor(cmd *cobra.Command, name string, initialContent []byte) ([]byte, error) {
	editor := editorcmd.Detect()
	if editor == "" {
		return nil, errors.New("could not detect a text editor binary, try setting $EDITOR")
	}
	tmpYAMLFile, err := ioutil.TempFile("", "lima-editor-")
	if err != nil {
		return nil, err
	}
	tmpYAMLPath := tmpYAMLFile.Name()
	defer os.RemoveAll(tmpYAMLPath)
	hdr := fmt.Sprintf("# Review and modify the following configuration for Lima instance %q.\n", name)
	if name == DefaultInstanceName {
		hdr += "# - In most cases, you do not need to modify this file.\n"
	}
	hdr += "# - To cancel starting Lima, just save this file as an empty file.\n"
	hdr += "\n"
	if err := ioutil.WriteFile(tmpYAMLPath,
		append([]byte(hdr), initialContent...),
		0o600); err != nil {
		return nil, err
	}

	editorCmd := exec.Command(editor, tmpYAMLPath)
	editorCmd.Env = os.Environ()
	editorCmd.Stdin = os.Stdin
	editorCmd.Stdout = os.Stdout
	editorCmd.Stderr = os.Stderr
	logrus.Debugf("opening editor %q for a file %q", editor, tmpYAMLPath)
	if err := editorCmd.Run(); err != nil {
		return nil, fmt.Errorf("could not execute editor %q for a file %q: %w", editor, tmpYAMLPath, err)
	}
	b, err := os.ReadFile(tmpYAMLPath)
	if err != nil {
		return nil, err
	}
	modifiedInclHdr := string(b)
	modifiedExclHdr := strings.TrimPrefix(modifiedInclHdr, hdr)
	if strings.TrimSpace(modifiedExclHdr) == "" {
		return nil, nil
	}
	return []byte(modifiedExclHdr), nil
}

func startAction(cmd *cobra.Command, args []string) error {
	inst, err := loadOrCreateInstance(cmd, args)
	if err != nil {
		return err
	}
	switch inst.Status {
	case store.StatusRunning:
		logrus.Infof("The instance %q is already running. Run `%s` to open the shell.",
			inst.Name, start.LimactlShellCmd(inst.Name))
		// Not an error
		return nil
	case store.StatusStopped:
		// NOP
	default:
		logrus.Warnf("expected status %q, got %q", store.StatusStopped, inst.Status)
	}
	ctx := cmd.Context()
	return start.Start(ctx, inst)
}

func argSeemsYAMLPath(arg string) bool {
	if strings.Contains(arg, "/") {
		return true
	}
	lower := strings.ToLower(arg)
	return strings.HasSuffix(lower, ".yml") || strings.HasSuffix(lower, ".yaml")
}

func instNameFromYAMLPath(yamlPath string) (string, error) {
	s := strings.ToLower(filepath.Base(yamlPath))
	s = strings.TrimSuffix(strings.TrimSuffix(s, ".yml"), ".yaml")
	s = strings.ReplaceAll(s, ".", "-")
	if err := identifiers.Validate(s); err != nil {
		return "", fmt.Errorf("filename %q is invalid: %w", yamlPath, err)
	}
	return s, nil
}

func startBashComplete(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	instances, _ := bashCompleteInstanceNames(cmd)
	return instances, cobra.ShellCompDirectiveDefault
}
