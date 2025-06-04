// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package limainfo

import (
	"errors"
	"io/fs"

	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/pkg/limayaml"
	"github.com/lima-vm/lima/pkg/registry"
	"github.com/lima-vm/lima/pkg/store/dirnames"
	"github.com/lima-vm/lima/pkg/templatestore"
	"github.com/lima-vm/lima/pkg/usrlocalsharelima"
	"github.com/lima-vm/lima/pkg/version"
)

type LimaInfo struct {
	Version         string                       `json:"version"`
	Templates       []templatestore.Template     `json:"templates"`
	DefaultTemplate *limayaml.LimaYAML           `json:"defaultTemplate"`
	LimaHome        string                       `json:"limaHome"`
	VMTypes         []string                     `json:"vmTypes"`     // since Lima v0.14.2
	GuestAgents     map[limayaml.Arch]GuestAgent `json:"guestAgents"` // since Lima v1.1.0
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
	info := &LimaInfo{
		Version:         version.Version,
		DefaultTemplate: y,
		VMTypes:         registry.DefaultRegistry.List(),
		GuestAgents:     make(map[limayaml.Arch]GuestAgent),
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
