// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package wsl2

import (
	"context"
	"fmt"
	"net"
	"regexp"

	"github.com/Microsoft/go-winio"
	"github.com/Microsoft/go-winio/pkg/guid"
	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/pkg/driver"
	"github.com/lima-vm/lima/pkg/freeport"
	"github.com/lima-vm/lima/pkg/limayaml"
	"github.com/lima-vm/lima/pkg/reflectutil"
	"github.com/lima-vm/lima/pkg/store"
	"github.com/lima-vm/lima/pkg/windows"
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

type LimaWslDriver struct {
	*driver.BaseDriver
}

func New(driver *driver.BaseDriver) *LimaWslDriver {
	port, err := freeport.VSock()
	if err != nil {
		logrus.WithError(err).Error("failed to get free VSock port")
	}
	driver.VSockPort = port
	driver.VirtioPort = ""
	return &LimaWslDriver{
		BaseDriver: driver,
	}
}

func (l *LimaWslDriver) Validate() error {
	if *l.Instance.Config.MountType != limayaml.WSLMount {
		return fmt.Errorf("field `mountType` must be %q for WSL2 driver, got %q", limayaml.WSLMount, *l.Instance.Config.MountType)
	}
	// TODO: revise this list for WSL2
	if unknown := reflectutil.UnknownNonEmptyFields(l.Instance.Config, knownYamlProperties...); len(unknown) > 0 {
		logrus.Warnf("Ignoring: vmType %s: %+v", *l.Instance.Config.VMType, unknown)
	}

	if !limayaml.IsNativeArch(*l.Instance.Config.Arch) {
		return fmt.Errorf("unsupported arch: %q", *l.Instance.Config.Arch)
	}

	for k, v := range l.Instance.Config.CPUType {
		if v != "" {
			logrus.Warnf("Ignoring: vmType %s: cpuType[%q]: %q", *l.Instance.Config.VMType, k, v)
		}
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

func (l *LimaWslDriver) Start(ctx context.Context) (chan error, error) {
	logrus.Infof("Starting WSL VM")
	status, err := store.GetWslStatus(l.Instance.Name)
	if err != nil {
		return nil, err
	}

	distroName := "lima-" + l.Instance.Name

	if status == store.StatusUninitialized {
		if err := EnsureFs(ctx, l.BaseDriver); err != nil {
			return nil, err
		}
		if err := initVM(ctx, l.BaseDriver.Instance.Dir, distroName); err != nil {
			return nil, err
		}
	}

	errCh := make(chan error)

	if err := startVM(ctx, distroName); err != nil {
		return nil, err
	}

	if err := provisionVM(
		ctx,
		l.BaseDriver.Instance.Dir,
		l.BaseDriver.Instance.Name,
		distroName,
		errCh,
	); err != nil {
		return nil, err
	}

	keepAlive(ctx, distroName, errCh)

	return errCh, err
}

// CanRunGUI requires WSLg, which requires specific version of WSL2 to be installed.
// TODO: Add check and add support for WSLg (instead of VNC) to hostagent.
func (l *LimaWslDriver) CanRunGUI() bool {
	return false
}

func (l *LimaWslDriver) RunGUI() error {
	return fmt.Errorf("RunGUI is not supported for the given driver '%s' and display '%s'", "wsl", *l.Instance.Config.Video.Display)
}

func (l *LimaWslDriver) Stop(ctx context.Context) error {
	logrus.Info("Shutting down WSL2 VM")
	distroName := "lima-" + l.Instance.Name
	return stopVM(ctx, distroName)
}

func (l *LimaWslDriver) Unregister(ctx context.Context) error {
	distroName := "lima-" + l.Instance.Name
	status, err := store.GetWslStatus(l.Instance.Name)
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
// As of 08-01-2024, github.com/mdlayher/vsock does not natively support vsock on
// Windows, so use the winio library to create the connection.
func (l *LimaWslDriver) GuestAgentConn(ctx context.Context) (net.Conn, error) {
	VMIDStr, err := windows.GetInstanceVMID(fmt.Sprintf("lima-%s", l.Instance.Name))
	if err != nil {
		return nil, err
	}
	VMIDGUID, err := guid.FromString(VMIDStr)
	if err != nil {
		return nil, err
	}
	sockAddr := &winio.HvsockAddr{
		VMID:      VMIDGUID,
		ServiceID: winio.VsockServiceID(uint32(l.VSockPort)),
	}
	return winio.Dial(ctx, sockAddr)
}
