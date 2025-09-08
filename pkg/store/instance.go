// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"text/tabwriter"
	"text/template"
	"time"

	"github.com/docker/go-units"
	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/driverutil"
	hostagentclient "github.com/lima-vm/lima/v2/pkg/hostagent/api/client"
	"github.com/lima-vm/lima/v2/pkg/instance/hostname"
	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limatype/dirnames"
	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
	"github.com/lima-vm/lima/v2/pkg/textutil"
	"github.com/lima-vm/lima/v2/pkg/version/versionutil"
)

// Inspect returns err only when the instance does not exist (os.ErrNotExist).
// Other errors are returned as *Instance.Errors.
func Inspect(ctx context.Context, instName string) (*limatype.Instance, error) {
	inst := &limatype.Instance{
		Name: instName,
		// TODO: support customizing hostname
		Hostname: hostname.FromInstName(instName),
		Status:   limatype.StatusUnknown,
	}
	// InstanceDir validates the instName but does not check whether the instance exists
	instDir, err := dirnames.InstanceDir(instName)
	if err != nil {
		return nil, err
	}
	// Make sure inst.Dir is set, even when YAML validation fails
	inst.Dir = instDir
	yamlPath := filepath.Join(instDir, filenames.LimaYAML)
	y, err := LoadYAMLByFilePath(ctx, yamlPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		inst.Errors = append(inst.Errors, err)
		return inst, nil
	}
	inst.Config = y
	inst.Arch = *y.Arch
	inst.VMType = *y.VMType
	inst.SSHAddress = "127.0.0.1"
	inst.SSHLocalPort = *y.SSH.LocalPort // maybe 0
	inst.SSHConfigFile = filepath.Join(instDir, filenames.SSHConfig)
	inst.HostAgentPID, err = ReadPIDFile(filepath.Join(instDir, filenames.HostAgentPID))
	if err != nil {
		inst.Status = limatype.StatusBroken
		inst.Errors = append(inst.Errors, err)
	}

	if inst.HostAgentPID != 0 {
		haSock := filepath.Join(instDir, filenames.HostAgentSock)
		haClient, err := hostagentclient.NewHostAgentClient(haSock)
		if err != nil {
			inst.Status = limatype.StatusBroken
			inst.Errors = append(inst.Errors, fmt.Errorf("failed to connect to %q: %w", haSock, err))
		} else {
			ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
			defer cancel()
			info, err := haClient.Info(ctx)
			if err != nil {
				inst.Status = limatype.StatusBroken
				inst.Errors = append(inst.Errors, fmt.Errorf("failed to get Info from %q: %w", haSock, err))
			} else {
				inst.SSHLocalPort = info.SSHLocalPort
			}
		}
	}

	inst.CPUs = *y.CPUs
	memory, err := units.RAMInBytes(*y.Memory)
	if err == nil {
		inst.Memory = memory
	}
	disk, err := units.RAMInBytes(*y.Disk)
	if err == nil {
		inst.Disk = disk
	}
	inst.AdditionalDisks = y.AdditionalDisks
	inst.Networks = y.Networks

	// 0 out values since not configurable on WSL2
	if inst.VMType == limatype.WSL2 {
		inst.Memory = 0
		inst.CPUs = 0
		inst.Disk = 0
	}

	protected := filepath.Join(instDir, filenames.Protected)
	if _, err := os.Lstat(protected); !errors.Is(err, os.ErrNotExist) {
		inst.Protected = true
	}

	inspectStatus(ctx, instDir, inst, y)

	tmpl, err := template.New("format").Parse(y.Message)
	if err != nil {
		inst.Errors = append(inst.Errors, fmt.Errorf("message %q is not a valid template: %w", y.Message, err))
		inst.Status = limatype.StatusBroken
	} else {
		data, err := AddGlobalFields(inst)
		if err != nil {
			inst.Errors = append(inst.Errors, fmt.Errorf("cannot add global fields to instance data: %w", err))
			inst.Status = limatype.StatusBroken
		} else {
			var message strings.Builder
			err = tmpl.Execute(&message, data)
			if err != nil {
				inst.Errors = append(inst.Errors, fmt.Errorf("cannot execute template %q: %w", y.Message, err))
				inst.Status = limatype.StatusBroken
			} else {
				inst.Message = message.String()
			}
		}
	}

	limaVersionFile := filepath.Join(instDir, filenames.LimaVersion)
	if version, err := os.ReadFile(limaVersionFile); err == nil {
		inst.LimaVersion = strings.TrimSpace(string(version))
		if _, err = versionutil.Parse(inst.LimaVersion); err != nil {
			logrus.Warnf("treating lima version %q from %q as very latest release", inst.LimaVersion, limaVersionFile)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		inst.Errors = append(inst.Errors, err)
	}
	inst.Param = y.Param
	return inst, nil
}

func inspectStatus(ctx context.Context, instDir string, inst *limatype.Instance, y *limatype.LimaYAML) {
	status, err := driverutil.InspectStatus(ctx, inst)
	if err != nil {
		inst.Status = limatype.StatusBroken
		inst.Errors = append(inst.Errors, fmt.Errorf("failed to inspect status: %w", err))
		return
	}

	if status == "" {
		inspectStatusWithPIDFiles(instDir, inst, y)
		return
	}

	inst.Status = status
}

func inspectStatusWithPIDFiles(instDir string, inst *limatype.Instance, y *limatype.LimaYAML) {
	var err error
	inst.DriverPID, err = ReadPIDFile(filepath.Join(instDir, filenames.PIDFile(*y.VMType)))
	if err != nil {
		inst.Status = limatype.StatusBroken
		inst.Errors = append(inst.Errors, err)
	}

	if inst.Status == limatype.StatusUnknown {
		switch {
		case inst.HostAgentPID > 0 && inst.DriverPID > 0:
			inst.Status = limatype.StatusRunning
		case inst.HostAgentPID == 0 && inst.DriverPID == 0:
			inst.Status = limatype.StatusStopped
		case inst.HostAgentPID > 0 && inst.DriverPID == 0:
			inst.Errors = append(inst.Errors, errors.New("host agent is running but driver is not"))
			inst.Status = limatype.StatusBroken
		default:
			inst.Errors = append(inst.Errors, fmt.Errorf("%s driver is running but host agent is not", inst.VMType))
			inst.Status = limatype.StatusBroken
		}
	}
}

// ReadPIDFile returns 0 if the PID file does not exist or the process has already terminated
// (in which case the PID file will be removed).
func ReadPIDFile(path string) (int, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		return 0, err
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return 0, err
	}
	// os.FindProcess will only return running processes on Windows, exit early
	if runtime.GOOS == "windows" {
		return pid, nil
	}
	err = proc.Signal(syscall.Signal(0))
	if err != nil {
		if errors.Is(err, os.ErrProcessDone) {
			_ = os.Remove(path)
			return 0, nil
		}
		// We may not have permission to send the signal (e.g. to network daemon running as root).
		// But if we get a permissions error, it means the process is still running.
		if !errors.Is(err, os.ErrPermission) {
			return 0, err
		}
	}
	return pid, nil
}

type FormatData struct {
	limatype.Instance `yaml:",inline"`

	// Using these host attributes is deprecated; they will be removed in Lima 3.0
	// The values are available from `limactl info` as hostOS, hostArch, limaHome, and identifyFile.
	HostOS       string `json:"HostOS" yaml:"HostOS" lima:"deprecated"`
	HostArch     string `json:"HostArch" yaml:"HostArch" lima:"deprecated"`
	LimaHome     string `json:"LimaHome" yaml:"LimaHome" lima:"deprecated"`
	IdentityFile string `json:"IdentityFile" yaml:"IdentityFile" lima:"deprecated"`
}

var FormatHelp = "\n" +
	"These functions are available to go templates:\n\n" +
	textutil.IndentString(2,
		strings.Join(textutil.FuncHelp, "\n")+"\n")

func AddGlobalFields(inst *limatype.Instance) (FormatData, error) {
	var data FormatData
	data.Instance = *inst
	// Add HostOS
	data.HostOS = runtime.GOOS
	// Add HostArch
	data.HostArch = limatype.NewArch(runtime.GOARCH)
	// Add IdentityFile
	configDir, err := dirnames.LimaConfigDir()
	if err != nil {
		return FormatData{}, err
	}
	data.IdentityFile = filepath.Join(configDir, filenames.UserPrivateKey)
	// Add LimaHome
	data.LimaHome, err = dirnames.LimaDir()
	if err != nil {
		return FormatData{}, err
	}
	return data, nil
}

type PrintOptions struct {
	AllFields     bool
	TerminalWidth int
}

// PrintInstances prints instances in a requested format to a given io.Writer.
// Supported formats are "json", "yaml", "table", or a go template.
func PrintInstances(w io.Writer, instances []*limatype.Instance, format string, options *PrintOptions) error {
	switch format {
	case "json":
		format = "{{json .}}"
	case "yaml":
		format = "{{yaml .}}"
	case "table":
		types := map[string]int{}
		archs := map[string]int{}
		for _, instance := range instances {
			types[instance.VMType]++
			archs[instance.Arch]++
		}
		all := options != nil && options.AllFields
		width := 0
		if options != nil {
			width = options.TerminalWidth
		}
		columnWidth := 8
		hideType := false
		hideArch := false
		hideDir := false

		columns := 1 // NAME
		columns += 2 // STATUS
		columns += 2 // SSH
		// can we still fit the remaining columns (7)
		if width == 0 || (columns+7)*columnWidth > width && !all {
			hideType = len(types) == 1
		}
		if !hideType {
			columns++ // VMTYPE
		}
		// only hide arch if it is the same as the host arch
		goarch := limatype.NewArch(runtime.GOARCH)
		// can we still fit the remaining columns (6)
		if width == 0 || (columns+6)*columnWidth > width && !all {
			hideArch = len(archs) == 1 && instances[0].Arch == goarch
		}
		if !hideArch {
			columns++ // ARCH
		}
		columns++ // CPUS
		columns++ // MEMORY
		columns++ // DISK
		// can we still fit the remaining columns (2)
		if width != 0 && (columns+2)*columnWidth > width && !all {
			hideDir = true
		}
		if !hideDir {
			columns += 2 // DIR
		}
		_ = columns

		w := tabwriter.NewWriter(w, 4, 8, 4, ' ', 0)
		fmt.Fprint(w, "NAME\tSTATUS\tSSH")
		if !hideType {
			fmt.Fprint(w, "\tVMTYPE")
		}
		if !hideArch {
			fmt.Fprint(w, "\tARCH")
		}
		fmt.Fprint(w, "\tCPUS\tMEMORY\tDISK")
		if !hideDir {
			fmt.Fprint(w, "\tDIR")
		}
		fmt.Fprintln(w)

		homeDir, err := os.UserHomeDir()
		if err != nil {
			return err
		}

		for _, instance := range instances {
			dir := instance.Dir
			if strings.HasPrefix(dir, homeDir) {
				dir = strings.Replace(dir, homeDir, "~", 1)
			}
			fmt.Fprintf(w, "%s\t%s\t%s",
				instance.Name,
				instance.Status,
				fmt.Sprintf("%s:%d", instance.SSHAddress, instance.SSHLocalPort),
			)
			if !hideType {
				fmt.Fprintf(w, "\t%s",
					instance.VMType,
				)
			}
			if !hideArch {
				fmt.Fprintf(w, "\t%s",
					instance.Arch,
				)
			}
			fmt.Fprintf(w, "\t%d\t%s\t%s",
				instance.CPUs,
				units.BytesSize(float64(instance.Memory)),
				units.BytesSize(float64(instance.Disk)),
			)
			if !hideDir {
				fmt.Fprintf(w, "\t%s",
					dir,
				)
			}
			fmt.Fprint(w, "\n")
		}
		return w.Flush()
	default:
		// NOP
	}
	tmpl, err := template.New("format").Funcs(textutil.TemplateFuncMap).Parse(format)
	if err != nil {
		return fmt.Errorf("invalid go template: %w", err)
	}
	for _, instance := range instances {
		data, err := AddGlobalFields(instance)
		if err != nil {
			return err
		}
		data.Message = strings.TrimSuffix(instance.Message, "\n")
		err = tmpl.Execute(w, data)
		if err != nil {
			return err
		}
		fmt.Fprintln(w)
	}
	return nil
}
