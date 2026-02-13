// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package limainfo

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"runtime"

	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/envutil"
	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limatype/dirnames"
	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
	"github.com/lima-vm/lima/v2/pkg/limayaml"
	"github.com/lima-vm/lima/v2/pkg/plugins"
	"github.com/lima-vm/lima/v2/pkg/registry"
	"github.com/lima-vm/lima/v2/pkg/templatestore"
	"github.com/lima-vm/lima/v2/pkg/usrlocal"
	"github.com/lima-vm/lima/v2/pkg/version"
)

type LimaInfo struct {
	Version         string                       `json:"version"`
	Templates       []templatestore.Template     `json:"templates"`
	DefaultTemplate *limatype.LimaYAML           `json:"defaultTemplate"`
	LimaHome        string                       `json:"limaHome"`
	VMTypes         []string                     `json:"vmTypes"`       // since Lima v0.14.2
	VMTypesEx       map[string]DriverExt         `json:"vmTypesEx"`     // since Lima v2.0.0
	GuestAgents     map[limatype.Arch]GuestAgent `json:"guestAgents"`   // since Lima v1.1.0
	ShellEnvBlock   []string                     `json:"shellEnvBlock"` // since Lima v2.0.0
	HostOS          string                       `json:"hostOS"`        // since Lima v2.0.0
	HostArch        string                       `json:"hostArch"`      // since Lima v2.0.0
	IdentityFile    string                       `json:"identityFile"`  // since Lima v2.0.0
	Plugins         []plugins.Plugin             `json:"plugins"`       // since Lima v2.0.0
	LibexecPaths    []string                     `json:"libexecPaths"`  // since Lima v2.0.0
	SharePaths      []string                     `json:"sharePaths"`    // since Lima v2.0.0
}

type DriverExt struct {
	Location string `json:"location,omitempty"` // since Lima v2.0.0
}

type GuestAgent struct {
	Location string `json:"location"` // since Lima v1.1.0
}

// New returns a LimaInfo object with the Lima version, a list of all Templates and their location,
// the DefaultTemplate corresponding to template:default with all defaults filled in, the
// LimaHome location, a list of all supported VMTypes, and a map of GuestAgents for each architecture.
func New(ctx context.Context) (*LimaInfo, error) {
	b, err := templatestore.Read(templatestore.Default)
	if err != nil {
		return nil, err
	}
	y, err := limayaml.Load(ctx, b, "")
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
		GuestAgents:     make(map[limatype.Arch]GuestAgent),
		ShellEnvBlock:   envutil.GetDefaultBlockList(),
		HostOS:          runtime.GOOS,
		HostArch:        limatype.NewArch(runtime.GOARCH),
	}
	info.Templates, err = templatestore.Templates()
	if err != nil {
		return nil, err
	}
	info.LimaHome, err = dirnames.LimaDir()
	if err != nil {
		return nil, err
	}
	configDir, err := dirnames.LimaConfigDir()
	if err != nil {
		return nil, err
	}
	info.LibexecPaths, err = usrlocal.LibexecLima()
	if err != nil {
		return nil, err
	}
	info.SharePaths, err = usrlocal.ShareLima()
	if err != nil {
		return nil, err
	}
	info.IdentityFile = filepath.Join(configDir, filenames.UserPrivateKey)
	for _, arch := range limatype.ArchTypes {
		for _, os := range limatype.OSTypes {
			bin, err := usrlocal.GuestAgentBinary(os, arch)
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					logrus.WithError(err).Debugf("Failed to resolve the guest agent binary for %q-%q", os, arch)
				} else {
					logrus.WithError(err).Warnf("Failed to resolve the guest agent binary for %q-%q", os, arch)
				}
				continue
			}
			key := arch
			// For the historical reason, the key does not have "Linux-" prefix
			if os != limatype.LINUX {
				key = os + "-" + arch
			}
			info.GuestAgents[key] = GuestAgent{
				Location: bin,
			}
		}
	}

	info.Plugins, err = plugins.Discover()
	if err != nil {
		// Don't fail the entire info command if plugin discovery fails.
		logrus.WithError(err).Warn("Failed to discover plugins")
	}

	return info, nil
}
