package hostagent

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strings"
	"text/template"

	"github.com/lima-vm/lima/pkg/limayaml"
	"github.com/lima-vm/lima/pkg/ptr"
	"github.com/lima-vm/lima/pkg/store"
	"github.com/lima-vm/lima/pkg/textutil"
	"github.com/mattn/go-shellwords"
	"github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"
)

var (
	defaultArguments = map[string][]string{
		limayaml.HostProvisionShellBash:       {"--noprofile", "--norc", "-e", "-o", "pipefail", "{{.ScriptName}}"},
		limayaml.HostProvisionShellSh:         {"-e", "{{.ScriptName}}"},
		limayaml.HostProvisionShellPwsh:       {"-command", ". '{{.ScriptName}}'"},
		limayaml.HostProvisionShellPowerShell: {"-command", ". '{{.ScriptName}}'"},
		limayaml.HostProvisionShellCmd:        {"/D", "/E:ON", "/V:OFF", "/S", "/C", "CALL \"{{.ScriptName}}\""},
	}
	defaultHostOS = map[string]*limayaml.StringArray{
		limayaml.HostProvisionShellBash:       {"darwin", "linux"},
		limayaml.HostProvisionShellSh:         {"darwin", "linux"},
		limayaml.HostProvisionShellPwsh:       {"windows"},
		limayaml.HostProvisionShellPowerShell: {"windows"},
		limayaml.HostProvisionShellCmd:        {"windows"},
	}
	extensions = map[string]string{
		limayaml.HostProvisionShellBash:       ".sh",
		limayaml.HostProvisionShellSh:         ".sh",
		limayaml.HostProvisionShellPwsh:       ".ps1",
		limayaml.HostProvisionShellPowerShell: ".ps1",
		limayaml.HostProvisionShellCmd:        ".cmd",
	}
)

func interpretShorthandHostProvision(p *limayaml.HostProvision) (limayaml.HostProvision, error) {
	var interpreted limayaml.HostProvision
	interpreted.Debug = p.Debug
	interpreted.Wait = p.Wait
	switch {
	case p.Bash != nil:
		interpreted.Shell = ptr.Of(limayaml.HostProvisionShellBash)
		interpreted.Script = p.Bash
	case p.Sh != nil:
		interpreted.Shell = ptr.Of(limayaml.HostProvisionShellSh)
		interpreted.Script = p.Sh
	case p.Pwsh != nil:
		interpreted.Shell = ptr.Of(limayaml.HostProvisionShellPwsh)
		interpreted.Script = p.Pwsh
	case p.PowerShell != nil:
		interpreted.Shell = ptr.Of(limayaml.HostProvisionShellPowerShell)
		interpreted.Script = p.PowerShell
	case p.Cmd != nil:
		interpreted.Shell = ptr.Of(limayaml.HostProvisionShellCmd)
		interpreted.Script = p.Cmd
	case p.Shell != nil && p.Script != nil:
		interpreted.Shell = p.Shell
		interpreted.Script = p.Script
	case p.Shell == nil && p.Script != nil:
		if runtime.GOOS == "windows" {
			interpreted.Shell = ptr.Of(limayaml.HostProvisionShellPwsh)
		} else {
			interpreted.Shell = ptr.Of(limayaml.HostProvisionShellBash)
		}
		interpreted.Script = p.Script
	case p.Shell != nil && p.Script == nil:
		interpreted.Shell = p.Shell
	}
	if p.HostOS != nil {
		interpreted.HostOS = p.HostOS
	} else if interpreted.Shell != nil {
		interpreted.HostOS = defaultHostOS[*interpreted.Shell]
	} else if runtime.GOOS == "windows" {
		interpreted.HostOS = defaultHostOS[limayaml.HostProvisionShellPwsh]
	} else {
		interpreted.HostOS = defaultHostOS[limayaml.HostProvisionShellBash]
	}
	return interpreted, nil
}

type HostProvisionFormatData struct {
	store.Instance
	ScriptName string
	Index      int
}

func executeHostProvisionTemplate(format string, data *HostProvisionFormatData) (bytes.Buffer, error) {
	tmpl, err := template.New("executeHostProvisionTemplate").Funcs(textutil.TemplateFuncMap).Parse(format)
	if err == nil {
		var out bytes.Buffer
		if err := tmpl.Execute(&out, data); err == nil {
			return out, nil
		}
	}
	return bytes.Buffer{}, err
}

func templateAppliedDefaultArguments(shell string, data *HostProvisionFormatData) ([]string, error) {
	args := defaultArguments[shell]
	templateAppliedArgs := make([]string, len(args))
	for i, arg := range args {
		out, err := executeHostProvisionTemplate(arg, data)
		if err != nil {
			return nil, err
		}
		templateAppliedArgs[i] = out.String()
	}
	return templateAppliedArgs, nil
}

func prepareHostProvision(hp limayaml.HostProvision, index int, instance *store.Instance) (func(o, e io.Writer) error, error) {
	var data HostProvisionFormatData
	data.Instance = *instance
	data.Index = index
	isDebug := hp.Debug != nil && *hp.Debug
	if hp.Script != nil {
		debugProvisionScriptPath := path.Join(instance.Dir, fmt.Sprintf("provision%d%s", index, extensions[*hp.Shell]))
		var tmpHostProvisionScriptFile *os.File
		var err error
		if isDebug {
			tmpHostProvisionScriptFile, err = os.Create(debugProvisionScriptPath)
		} else {
			os.RemoveAll(debugProvisionScriptPath)
			tmpHostProvisionScriptFile, err = os.CreateTemp("", "lima-provision-*"+extensions[*hp.Shell])
		}
		if err != nil {
			return nil, err
		}
		data.ScriptName = tmpHostProvisionScriptFile.Name()
		if runtime.GOOS == "windows" {
			*hp.Script = strings.ReplaceAll(*hp.Script, "\r\n", "\n")
		}
		out, err := executeHostProvisionTemplate(*hp.Script, &data)
		if err != nil {
			return nil, err
		}
		if _, err := tmpHostProvisionScriptFile.Write(out.Bytes()); err != nil {
			return nil, err
		}
		if err := tmpHostProvisionScriptFile.Close(); err != nil {
			return nil, err
		}
	}
	var arg0 string
	var args []string
	var err error
	if hp.Shell == nil { // The default shell has fallback functionality.
		var defaultShells []string
		if runtime.GOOS == "windows" {
			defaultShells = []string{limayaml.HostProvisionShellPwsh, limayaml.HostProvisionShellPowerShell}
		} else {
			defaultShells = []string{limayaml.HostProvisionShellBash, limayaml.HostProvisionShellSh}
		}
		for _, shell := range defaultShells {
			if found, err := exec.LookPath(shell); err == nil {
				arg0 = found
				args, err = templateAppliedDefaultArguments(shell, &data)
				if err != nil {
					return nil, err
				}
				break
			}
		}
		if arg0 == "" {
			return nil, fmt.Errorf("failed to find a default shell in %v", defaultShells)
		}
	} else {
		args, err = templateAppliedDefaultArguments(*hp.Shell, &data)
		if err != nil {
			return nil, err
		}
		if len(args) == 0 { // custom shell
			out, err := executeHostProvisionTemplate(*hp.Shell, &data)
			if err != nil {
				return nil, err
			}
			parsedArgs, err := shellwords.Parse(out.String())
			if err != nil {
				return nil, err
			}
			arg0 = parsedArgs[0]
			args = parsedArgs[1:]
		} else {
			arg0 = *hp.Shell
		}
		arg0, err = exec.LookPath(arg0)
		if err != nil {
			return nil, err
		}
	}

	cmd := exec.Command(arg0, args...)
	cmd.Dir = instance.Dir
	return func(o, e io.Writer) error {
		if hp.Script != nil && !isDebug {
			defer os.RemoveAll(data.ScriptName)
		}
		cmd.Stdout = o
		cmd.Stderr = e
		return cmd.Run()
	}, nil
}

func (a *HostAgent) ExecuteHostProvision() error {
	if len(a.y.HostProvision) == 0 {
		return nil
	}
	instance, err := store.Inspect(a.instName)
	if err != nil {
		return err
	}
	for i, p := range a.y.HostProvision {
		hp, err := interpretShorthandHostProvision(&p)
		if err != nil {
			return err
		}
		log := logrus.WithField("hostProvision", i)
		log.WithField("interpreted", hp).Debug("interpreted hostProvision")
		if hp.HostOS == nil {
			log.Debugf("hostProvision[%d] executing because hostOS is not specified", i)
		} else if !slices.Contains(*hp.HostOS, runtime.GOOS) {
			log.Warnf("hostProvision[%d] skipped because runtime.GOOS=%q is not in %v", i, runtime.GOOS, *hp.HostOS)
			continue
		}
		output, err := prepareHostProvision(hp, i, instance)
		if err != nil {
			return err
		}
		o := log.WriterLevel(logrus.DebugLevel)
		e := log.WriterLevel(logrus.ErrorLevel)
		if hp.Wait != nil && *hp.Wait {
			if err := output(o, e); err != nil {
				return fmt.Errorf("hostProvision[%d] failed: %w", i, err)
			}
			log.Debugf("hostProvision[%d] succeeded", i)
		} else {
			i := i
			go func() {
				log.Debugf("hostProvision[%d] started", i)
				if err := output(o, e); err != nil {
					log.Errorf("hostProvision[%d] failed: %v", i, err)
				} else {
					log.Debugf("hostProvision[%d] terminated", i)
				}
			}()
		}
	}

	return nil
}
