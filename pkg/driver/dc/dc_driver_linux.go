// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package dc

import (
	"context"
	"fmt"
	"net"
	"regexp"

	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/driver"
	"github.com/lima-vm/lima/v2/pkg/limayaml"
	"github.com/lima-vm/lima/v2/pkg/reflectutil"
	"github.com/lima-vm/lima/v2/pkg/store"
)

var knownYamlProperties = []string{
	"Arch",
	"Containerd",
	"CopyToHost",
	"CPUType",
	"Disk",
	"DNS",
	"Env",
	"HostResolver",
	"Images",
	"Message",
	"Mounts",
	"MountType",
	"Param",
	"Plain",
	"PortForwards",
	"Probes",
	"PropagateProxyEnv",
	"Provision",
	"SSH",
	"VMType",
}

const Enabled = true

type LimaDcDriver struct {
	Instance *store.Instance

	SSHLocalPort int
	vSockPort    int
	virtioPort   string
}

var _ driver.Driver = (*LimaDcDriver)(nil)

func New() *LimaDcDriver {
	return &LimaDcDriver{
		vSockPort:  0,
		virtioPort: "",
	}
}

func (l *LimaDcDriver) Configure(inst *store.Instance) *driver.ConfiguredDriver {
	l.Instance = inst
	l.SSHLocalPort = inst.SSHLocalPort

	return &driver.ConfiguredDriver{
		Driver: l,
	}
}

func (l *LimaDcDriver) Validate() error {
	if *l.Instance.Config.MountType != limayaml.REVSSHFS {
		return fmt.Errorf("field `mountType` must be %q for DC driver, got %q", limayaml.REVSSHFS, *l.Instance.Config.MountType)
	}
	// TODO: revise this list for DC
	if unknown := reflectutil.UnknownNonEmptyFields(l.Instance.Config, knownYamlProperties...); len(unknown) > 0 {
		logrus.Warnf("Ignoring: vmType %s: %+v", *l.Instance.Config.VMType, unknown)
	}

	if !limayaml.IsNativeArch(*l.Instance.Config.Arch) {
		return fmt.Errorf("unsupported arch: %q", *l.Instance.Config.Arch)
	}

	// TODO: real filetype checks
	tarFileRegex := regexp.MustCompile(`.*tar\.*`)
	for i, image := range l.Instance.Config.Images {
		if unknown := reflectutil.UnknownNonEmptyFields(image, "File"); len(unknown) > 0 {
			logrus.Warnf("Ignoring: vmType %s: images[%d]: %+v", *l.Instance.Config.VMType, i, unknown)
		}
		match := tarFileRegex.MatchString(image.Location)
		if image.Arch == *l.Instance.Config.Arch && !match {
			return fmt.Errorf("unsupported image type for vmType: %s, tarball root file system required: %q", *l.Instance.Config.VMType, image.Location)
		}
	}

	for i, mount := range l.Instance.Config.Mounts {
		if unknown := reflectutil.UnknownNonEmptyFields(mount); len(unknown) > 0 {
			logrus.Warnf("Ignoring: vmType %s: mounts[%d]: %+v", *l.Instance.Config.VMType, i, unknown)
		}
	}

	for i, network := range l.Instance.Config.Networks {
		if unknown := reflectutil.UnknownNonEmptyFields(network); len(unknown) > 0 {
			logrus.Warnf("Ignoring: vmType %s: networks[%d]: %+v", *l.Instance.Config.VMType, i, unknown)
		}
	}

	audioDevice := *l.Instance.Config.Audio.Device
	if audioDevice != "" {
		logrus.Warnf("Ignoring: vmType %s: `audio.device`: %+v", *l.Instance.Config.VMType, audioDevice)
	}

	return nil
}

func (l *LimaDcDriver) Start(ctx context.Context) (chan error, error) {
	logrus.Infof("Starting DC VM")
	status, err := store.GetDcStatus(l.Instance.Name)
	if err != nil {
		return nil, err
	}

	distroName := "lima-" + l.Instance.Name

	if status == store.StatusUninitialized {
		if err := EnsureFs(ctx, l.Instance); err != nil {
			return nil, err
		}
		if err := initVM(ctx, l.Instance.Dir, distroName); err != nil {
			return nil, err
		}
		cpus := l.Instance.CPUs
		memory := int(l.Instance.Memory >> 20) // MiB
		if err := registerVM(ctx, distroName, cpus, memory); err != nil {
			return nil, err
		}
	}

	errCh := make(chan error)

	if err := startVM(ctx, distroName); err != nil {
		return nil, err
	}

	if err := provisionVM(
		ctx,
		l.Instance.Dir,
		l.Instance.Name,
		distroName,
		errCh,
	); err != nil {
		return nil, err
	}

	return errCh, err
}

func (l *LimaDcDriver) canRunGUI() bool {
	return false
}

func (l *LimaDcDriver) RunGUI() error {
	return fmt.Errorf("RunGUI is not supported for the given driver '%s' and display '%s'", "dc", *l.Instance.Config.Video.Display)
}

func (l *LimaDcDriver) Stop(ctx context.Context) error {
	logrus.Info("Shutting down DC VM")
	distroName := "lima-" + l.Instance.Name
	return stopVM(ctx, distroName)
}

func (l *LimaDcDriver) Unregister(ctx context.Context) error {
	distroName := "lima-" + l.Instance.Name
	status, err := store.GetDcStatus(l.Instance.Name)
	if err != nil {
		return err
	}
	switch status {
	case store.StatusRunning, store.StatusStopped, store.StatusBroken, store.StatusInstalling:
		return unregisterVM(ctx, distroName)
	}

	logrus.Info("VM not registered, skipping unregistration")
	return nil
}

// GuestAgentConn returns the guest agent connection, or nil (if forwarded by ssh).
func (l *LimaDcDriver) GuestAgentConn(_ context.Context) (net.Conn, string, error) {
	return nil, "", nil
}

func (l *LimaDcDriver) Info() driver.Info {
	var info driver.Info
	if l.Instance != nil {
		info.InstanceDir = l.Instance.Dir
	}
	info.DriverName = "dc"
	info.CanRunGUI = l.canRunGUI()
	info.VirtioPort = l.virtioPort
	info.VsockPort = l.vSockPort
	return info
}

func (l *LimaDcDriver) Initialize(_ context.Context) error {
	return nil
}

func (l *LimaDcDriver) CreateDisk(_ context.Context) error {
	return nil
}

func (l *LimaDcDriver) Register(_ context.Context) error {
	return nil
}

func (l *LimaDcDriver) ChangeDisplayPassword(_ context.Context, _ string) error {
	return nil
}

func (l *LimaDcDriver) DisplayConnection(_ context.Context) (string, error) {
	return "", nil
}

func (l *LimaDcDriver) CreateSnapshot(_ context.Context, _ string) error {
	return errUnimplemented
}

func (l *LimaDcDriver) ApplySnapshot(_ context.Context, _ string) error {
	return errUnimplemented
}

func (l *LimaDcDriver) DeleteSnapshot(_ context.Context, _ string) error {
	return errUnimplemented
}

func (l *LimaDcDriver) ListSnapshots(_ context.Context) (string, error) {
	return "", errUnimplemented
}

func (l *LimaDcDriver) ForwardGuestAgent() bool {
	// If driver is not providing, use host agent
	return l.vSockPort == 0 && l.virtioPort == ""
}
