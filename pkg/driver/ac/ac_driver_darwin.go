// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package ac

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

type LimaAcDriver struct {
	Instance *store.Instance

	SSHLocalPort int
	vSockPort    int
	virtioPort   string
}

var _ driver.Driver = (*LimaAcDriver)(nil)

func New() *LimaAcDriver {
	return &LimaAcDriver{
		vSockPort:  0,
		virtioPort: "",
	}
}

func (l *LimaAcDriver) Configure(inst *store.Instance) *driver.ConfiguredDriver {
	l.Instance = inst
	l.SSHLocalPort = inst.SSHLocalPort

	return &driver.ConfiguredDriver{
		Driver: l,
	}
}

func (l *LimaAcDriver) Validate() error {
	if *l.Instance.Config.MountType != limayaml.VIRTIOFS && *l.Instance.Config.MountType != limayaml.REVSSHFS {
		return fmt.Errorf("field `mountType` must be %q or %q for AC driver, got %q", limayaml.VIRTIOFS, limayaml.REVSSHFS, *l.Instance.Config.MountType)
	}
	// TODO: revise this list for AC
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

func (l *LimaAcDriver) Start(ctx context.Context) (chan error, error) {
	logrus.Infof("Starting AC VM")
	status, err := store.GetAcStatus(l.Instance.Name)
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

func (l *LimaAcDriver) canRunGUI() bool {
	return false
}

func (l *LimaAcDriver) RunGUI() error {
	return fmt.Errorf("RunGUI is not supported for the given driver '%s' and display '%s'", "ac", *l.Instance.Config.Video.Display)
}

func (l *LimaAcDriver) Stop(ctx context.Context) error {
	logrus.Info("Shutting down AC VM")
	distroName := "lima-" + l.Instance.Name
	return stopVM(ctx, distroName)
}

func (l *LimaAcDriver) Unregister(ctx context.Context) error {
	distroName := "lima-" + l.Instance.Name
	status, err := store.GetAcStatus(l.Instance.Name)
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
func (l *LimaAcDriver) GuestAgentConn(_ context.Context) (net.Conn, string, error) {
	return nil, "", nil
}

func (l *LimaAcDriver) Info() driver.Info {
	var info driver.Info
	if l.Instance != nil {
		info.InstanceDir = l.Instance.Dir
	}
	info.DriverName = "ac"
	info.CanRunGUI = l.canRunGUI()
	info.VirtioPort = l.virtioPort
	info.VsockPort = l.vSockPort
	return info
}

func (l *LimaAcDriver) Initialize(_ context.Context) error {
	return nil
}

func (l *LimaAcDriver) CreateDisk(_ context.Context) error {
	return nil
}

func (l *LimaAcDriver) Register(_ context.Context) error {
	return nil
}

func (l *LimaAcDriver) ChangeDisplayPassword(_ context.Context, _ string) error {
	return nil
}

func (l *LimaAcDriver) DisplayConnection(_ context.Context) (string, error) {
	return "", nil
}

func (l *LimaAcDriver) CreateSnapshot(_ context.Context, _ string) error {
	return errUnimplemented
}

func (l *LimaAcDriver) ApplySnapshot(_ context.Context, _ string) error {
	return errUnimplemented
}

func (l *LimaAcDriver) DeleteSnapshot(_ context.Context, _ string) error {
	return errUnimplemented
}

func (l *LimaAcDriver) ListSnapshots(_ context.Context) (string, error) {
	return "", errUnimplemented
}

func (l *LimaAcDriver) ForwardGuestAgent() bool {
	// If driver is not providing, use host agent
	return l.vSockPort == 0 && l.virtioPort == ""
}
