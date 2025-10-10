// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package limatype

import (
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"

	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
)

type Status = string

const (
	StatusUnknown       Status = ""
	StatusUninitialized Status = "Uninitialized"
	StatusInstalling    Status = "Installing"
	StatusBroken        Status = "Broken"
	StatusStopped       Status = "Stopped"
	StatusRunning       Status = "Running"
)

type Instance struct {
	Name string `json:"name"`
	// Hostname, not HostName (corresponds to SSH's naming convention)
	Hostname              string            `json:"hostname"`
	Status                Status            `json:"status"`
	Dir                   string            `json:"dir"`
	VMType                VMType            `json:"vmType"`
	Arch                  Arch              `json:"arch"`
	CPUs                  int               `json:"cpus,omitempty"`
	Memory                int64             `json:"memory,omitempty"` // bytes
	Disk                  int64             `json:"disk,omitempty"`   // bytes
	Message               string            `json:"message,omitempty"`
	AdditionalDisks       []Disk            `json:"additionalDisks,omitempty"`
	Networks              []Network         `json:"network,omitempty"`
	SSHLocalPort          int               `json:"sshLocalPort,omitempty"`
	SSHConfigFile         string            `json:"sshConfigFile,omitempty"`
	HostAgentPID          int               `json:"hostAgentPID,omitempty"`
	DriverPID             int               `json:"driverPID,omitempty"`
	Errors                []error           `json:"errors,omitempty"`
	Config                *LimaYAML         `json:"config,omitempty"`
	SSHAddress            string            `json:"sshAddress,omitempty"`
	Protected             bool              `json:"protected"`
	LimaVersion           string            `json:"limaVersion"`
	Param                 map[string]string `json:"param,omitempty"`
	AutoStartedIdentifier string            `json:"autoStartedIdentifier,omitempty"`
	// Guest IP address directly accessible from the host.
	GuestIP net.IP `json:"guestIP,omitempty"`
}

// Protect protects the instance to prohibit accidental removal.
// Protect does not return an error even when the instance is already protected.
func (inst *Instance) Protect() error {
	protected := filepath.Join(inst.Dir, filenames.Protected)
	// TODO: Do an equivalent of `chmod +a "everyone deny delete,delete_child,file_inherit,directory_inherit"`
	// https://github.com/lima-vm/lima/issues/1595
	if err := os.WriteFile(protected, nil, 0o400); err != nil {
		return err
	}
	inst.Protected = true
	return nil
}

// Unprotect unprotects the instance.
// Unprotect does not return an error even when the instance is already unprotected.
func (inst *Instance) Unprotect() error {
	protected := filepath.Join(inst.Dir, filenames.Protected)
	if err := os.RemoveAll(protected); err != nil {
		return err
	}
	inst.Protected = false
	return nil
}

func (inst *Instance) MarshalJSON() ([]byte, error) {
	type Alias Instance
	errorsAsStrings := make([]string, len(inst.Errors))
	for i, err := range inst.Errors {
		if err != nil {
			errorsAsStrings[i] = err.Error()
		}
	}
	return json.Marshal(&struct {
		*Alias
		Errors []string `json:"errors,omitempty"`
	}{
		Alias:  (*Alias)(inst),
		Errors: errorsAsStrings,
	})
}

func (inst *Instance) UnmarshalJSON(data []byte) error {
	type Alias Instance
	aux := &struct {
		*Alias
		Errors []string `json:"errors,omitempty"`
	}{
		Alias: (*Alias)(inst),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	inst.Errors = nil
	for _, msg := range aux.Errors {
		inst.Errors = append(inst.Errors, errors.New(msg))
	}
	return nil
}

func (inst *Instance) SSHAddressPort() (sshAddress string, sshPort int) {
	sshAddress = inst.SSHAddress
	sshPort = inst.SSHLocalPort
	if inst.GuestIP != nil {
		sshAddress = inst.GuestIP.String()
		sshPort = 22
	}
	return sshAddress, sshPort
}
