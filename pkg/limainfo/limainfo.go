// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package limainfo

import (
	"errors"
	"io/fs"

	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/envutil"
	"github.com/lima-vm/lima/v2/pkg/limayaml"
	"github.com/lima-vm/lima/v2/pkg/registry"
	"github.com/lima-vm/lima/v2/pkg/store/dirnames"
	"github.com/lima-vm/lima/v2/pkg/templatestore"
	"github.com/lima-vm/lima/v2/pkg/usrlocalsharelima"
	"github.com/lima-vm/lima/v2/pkg/version"
)

type LimaInfo struct {
	Version         string                       `json:"version"`
	Templates       []templatestore.Template     `json:"templates"`
	DefaultTemplate *limayaml.LimaYAML           `json:"defaultTemplate"`
	LimaHome        string                       `json:"limaHome"`
	VMTypes         []string                     `json:"vmTypes"`     // since Lima v0.14.2
	VMTypesEx       map[string]DriverExt         `json:"vmTypesEx"`   // since Lima v2.0.0
	GuestAgents     map[limayaml.Arch]GuestAgent `json:"guestAgents"` // since Lima v1.1.0
	ShellEnvBlock   []string                     `json:"shellEnvBlock"`
}

type DriverExt struct {
	Location string `json:"location,omitempty"` // since Lima v2.0.0
}

type GuestAgent struct {
	Location string `json:"location"` // since Lima v1.1.0
}

// New returns a LimaInfo object with the Lima version, a list of all Templates and their location,
// the DefaultTemplate corresponding to template://default with all defaults filled in, the
// LimaHome location, a list of all supported VMTypes, and a map of GuestAgents for each architecture.
func New() (*LimaInfo, error) {
	b, err := templatestore.Read(templatestore.Default)
	if err != nil {
		return nil, err
	}
	y, err := limayaml.Load(b, "")
	if err != nil {
		return nil, err
	}

	reg := registry.List()
	if len(reg) == 0 {
		return nil, errors.New("no VM types found; ensure that the drivers are properly registered")
	}
	vmTypesEx := make(map[string]DriverExt)
	var vmTypes []string
	for name, path := range reg {
		vmTypesEx[name] = DriverExt{
			Location: path,
		}
		vmTypes = append(vmTypes, name)
	}

	info := &LimaInfo{
		Version:         version.Version,
		DefaultTemplate: y,
		VMTypes:         vmTypes,
		VMTypesEx:       vmTypesEx,
		GuestAgents:     make(map[limayaml.Arch]GuestAgent),
		ShellEnvBlock:   envutil.GetDefaultBlockList(),
	}
	info.Templates, err = templatestore.Templates()
	if err != nil {
		return nil, err
	}
	info.LimaHome, err = dirnames.LimaDir()
	if err != nil {
		return nil, err
	}
	for _, arch := range limayaml.ArchTypes {
		bin, err := usrlocalsharelima.GuestAgentBinary(limayaml.LINUX, arch)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				logrus.WithError(err).Debugf("Failed to resolve the guest agent binary for %q", arch)
			} else {
				logrus.WithError(err).Warnf("Failed to resolve the guest agent binary for %q", arch)
			}
			continue
		}
		info.GuestAgents[arch] = GuestAgent{
			Location: bin,
		}
	}
	return info, nil
}
