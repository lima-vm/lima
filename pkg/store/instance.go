package store

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/AkihiroSuda/lima/pkg/limayaml"
)

type Status = string

const (
	StatusUnknown Status = ""
	StatusBroken  Status = "Broken"
	StatusStopped Status = "Stopped"
	StatusRunning Status = "Running"
)

type Instance struct {
	Name         string        `json:"name"`
	Status       Status        `json:"status"`
	Dir          string        `json:"dir"`
	Arch         limayaml.Arch `json:"arch"`
	SSHLocalPort int           `json:"sshLocalPort,omitempty"`
	HostAgentPID int           `json:"hostAgentPID,omitempty"`
	QemuPID      int           `json:"qemuPID,omitempty"`
	Errors       []error       `json:"errors,omitempty"`
}

// Inspect returns err only when the instance does not exist.
// Other errors are returned as *Instance.Errors
func Inspect(instName string) (*Instance, error) {
	inst := &Instance{
		Name:   instName,
		Status: StatusUnknown,
	}
	y, instDir, err := LoadYAMLByInstanceName(instName)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		inst.Errors = append(inst.Errors, err)
		return inst, nil
	}
	inst.Dir = instDir
	inst.Arch = y.Arch
	inst.SSHLocalPort = y.SSH.LocalPort

	inst.HostAgentPID, err = readPIDFile(filepath.Join(instDir, "ha.pid"))
	if err != nil {
		inst.Status = StatusBroken
		inst.Errors = append(inst.Errors, err)
	}

	inst.QemuPID, err = readPIDFile(filepath.Join(instDir, "qemu.pid"))
	if err != nil {
		inst.Status = StatusBroken
		inst.Errors = append(inst.Errors, err)
	}

	if inst.Status == StatusUnknown {
		if inst.HostAgentPID > 0 && inst.QemuPID > 0 {
			inst.Status = StatusRunning
		} else if inst.HostAgentPID == 0 && inst.QemuPID == 0 {
			inst.Status = StatusStopped
		} else if inst.HostAgentPID > 0 && inst.QemuPID == 0 {
			inst.Errors = append(inst.Errors, errors.New("host agent is running but qemu is not"))
			inst.Status = StatusBroken
		} else {
			inst.Errors = append(inst.Errors, errors.New("qemu is running but host agent is not"))
			inst.Status = StatusBroken
		}
	}

	return inst, nil
}

// readPIDFile returns 0 if the PID file does not exist
func readPIDFile(path string) (int, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(b)))
}
