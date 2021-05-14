package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/AkihiroSuda/lima/pkg/limayaml"
	"github.com/AkihiroSuda/lima/pkg/start"
	"github.com/AkihiroSuda/lima/pkg/store"
	"github.com/containerd/containerd/identifiers"
	"github.com/mattn/go-isatty"
	"github.com/norouter/norouter/cmd/norouter/editorcmd"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var startCommand = &cli.Command{
	Name:      "start",
	Usage:     fmt.Sprintf("Start an instance of Lima. If the instance does not exist, open an editor for creating new one, with name %q", DefaultInstanceName),
	ArgsUsage: "[flags] NAME|FILE.yaml",
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:  "tty",
			Usage: "enable TUI interactions such as opening an editor, defaults to true when stdout is a terminal",
			Value: isatty.IsTerminal(os.Stdout.Fd()),
		},
	},
	Action:       startAction,
	BashComplete: startBashComplete,
}

func loadOrCreateYAML(clicontext *cli.Context) (y *limayaml.LimaYAML, instDir string, err error) {
	if clicontext.NArg() > 1 {
		return nil, "", errors.Errorf("too many arguments")
	}

	arg := clicontext.Args().First()
	if arg == "" {
		arg = DefaultInstanceName
	}

	yBytes := limayaml.DefaultTemplate
	var instName string

	if argSeemsYAMLPath(arg) {
		instName = instNameFromYAMLPath(arg)
		logrus.Debugf("interpreting argument %q as a file path for instance %q", arg, instName)
		yBytes, err = os.ReadFile(arg)
		if err != nil {
			return nil, "", err
		}
	} else {
		instName = arg
		logrus.Debugf("interpreting argument %q as an instance name %q", arg, instName)
		if err = identifiers.Validate(instName); err != nil {
			return nil, "", errors.Wrapf(err, "argument must be either an instance name or a YAML file path, got %q", instName)
		}
		y, instDir, err = store.LoadYAMLByInstanceName(instName)
		if err == nil {
			logrus.Infof("Using the existing instance %q", instName)
			return y, instDir, nil
		} else {
			if !errors.Is(err, os.ErrNotExist) {
				return nil, "", err
			}
		}
	}
	// create a new instance from the template
	instDir, err = store.InstanceDir(instName)
	if err != nil {
		return nil, "", err
	}

	if _, err := os.Stat(instDir); !errors.Is(err, os.ErrNotExist) {
		return nil, "", errors.Errorf("instance %q already exists (%q)", instName, instDir)
	}

	if clicontext.Bool("tty") {
		yBytes, err = openEditor(clicontext, instName, yBytes)
		if err != nil {
			return nil, "", err
		}
		if len(yBytes) == 0 {
			logrus.Info("Aborting, as requested by saving the file with empty content")
			os.Exit(0)
			return nil, "", errors.New("should not reach here")
		}
	} else {
		logrus.Info("Terminal is not available, proceeding without opening an editor")
	}
	y, err = limayaml.Load(yBytes)
	if err != nil {
		return nil, "", err
	}
	if err := limayaml.Validate(*y); err != nil {
		if !clicontext.Bool("tty") {
			return nil, "", err
		}
		rejectedYAML := "lima.REJECTED.yaml"
		if writeErr := os.WriteFile(rejectedYAML, yBytes, 0644); writeErr != nil {
			return nil, "", errors.Wrapf(err, "the YAML is invalid, attempted to save the buffer as %q but failed: %v", rejectedYAML, writeErr)
		}
		return nil, "", errors.Wrapf(err, "the YAML is invalid, saved the buffer as %q", rejectedYAML)
	}
	if err := os.MkdirAll(instDir, 0700); err != nil {
		return nil, "", err
	}
	if err := os.WriteFile(filepath.Join(instDir, store.YAMLFileName), yBytes, 0644); err != nil {
		return nil, "", err
	}
	return y, instDir, nil
}

// openEditor opens an editor, and returns the content (not path) of the modified yaml.
//
// openEditor returns nil when the file was saved as an empty file, optionally with whitespaces.
func openEditor(clicontext *cli.Context, name string, initialContent []byte) ([]byte, error) {
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
		return nil, errors.Wrapf(err, "could not execute editor %q for a file %q", editor, tmpYAMLPath)
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

func startAction(clicontext *cli.Context) error {
	y, instDir, err := loadOrCreateYAML(clicontext)
	if err != nil {
		return err
	}
	instName, err := store.InstanceNameFromInstanceDir(instDir)
	if err != nil {
		return err
	}
	ctx := clicontext.Context
	return start.Start(ctx, instName, instDir, y)
}

func argSeemsYAMLPath(arg string) bool {
	if strings.Contains(arg, "/") {
		return true
	}
	lower := strings.ToLower(arg)
	return strings.HasSuffix(lower, ".yml") || strings.HasSuffix(lower, ".yaml")
}

func instNameFromYAMLPath(yamlPath string) string {
	s := strings.ToLower(filepath.Base(yamlPath))
	s = strings.TrimSuffix(strings.TrimSuffix(s, ".yml"), ".yaml")
	s = strings.ReplaceAll(s, ".", "-")
	if err := identifiers.Validate(s); err != nil {
		logrus.WithField("candidate", s).WithError(err).
			Warnf("failed to determine the name of the new instance from file path %q, using the default name %q",
				yamlPath, DefaultInstanceName)
		return DefaultInstanceName
	}
	return s
}

func startBashComplete(clicontext *cli.Context) {
	bashCompleteInstanceNames(clicontext)
}
