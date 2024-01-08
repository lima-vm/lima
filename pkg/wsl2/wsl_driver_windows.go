package wsl2

import (
	"context"
	"fmt"
	"net"
	"regexp"

	"github.com/lima-vm/lima/pkg/driver"
	"github.com/lima-vm/lima/pkg/limayaml"
	"github.com/lima-vm/lima/pkg/reflectutil"
	"github.com/lima-vm/lima/pkg/store"
	"github.com/mdlayher/vsock"
	"github.com/sirupsen/logrus"
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
	return &LimaWslDriver{
		BaseDriver: driver,
	}
}

func (l *LimaWslDriver) Validate() error {
	if *l.Yaml.MountType != limayaml.WSLMount {
		return fmt.Errorf("field `mountType` must be %q for WSL2 driver, got %q", limayaml.WSLMount, *l.Yaml.MountType)
	}
	// TODO: revise this list for WSL2
	if unknown := reflectutil.UnknownNonEmptyFields(l.Yaml, knownYamlProperties...); len(unknown) > 0 {
		logrus.Warnf("Ignoring: vmType %s: %+v", *l.Yaml.VMType, unknown)
	}

	if !limayaml.IsNativeArch(*l.Yaml.Arch) {
		return fmt.Errorf("unsupported arch: %q", *l.Yaml.Arch)
	}

	for k, v := range l.Yaml.CPUType {
		if v != "" {
			logrus.Warnf("Ignoring: vmType %s: cpuType[%q]: %q", *l.Yaml.VMType, k, v)
		}
	}

	re, err := regexp.Compile(`.*tar\.*`)
	if err != nil {
		return fmt.Errorf("failed to compile file check regex: %w", err)
	}
	for i, image := range l.Yaml.Images {
		if unknown := reflectutil.UnknownNonEmptyFields(image, "File"); len(unknown) > 0 {
			logrus.Warnf("Ignoring: vmType %s: images[%d]: %+v", *l.Yaml.VMType, i, unknown)
		}
		// TODO: real filetype checks
		match := re.MatchString(image.Location)
		if image.Arch == *l.Yaml.Arch && !match {
			return fmt.Errorf("unsupported image type for vmType: %s, tarball root file system required: %q", *l.Yaml.VMType, image.Location)
		}
	}

	for i, mount := range l.Yaml.Mounts {
		if unknown := reflectutil.UnknownNonEmptyFields(mount); len(unknown) > 0 {
			logrus.Warnf("Ignoring: vmType %s: mounts[%d]: %+v", *l.Yaml.VMType, i, unknown)
		}
	}

	for i, network := range l.Yaml.Networks {
		if unknown := reflectutil.UnknownNonEmptyFields(network); len(unknown) > 0 {
			logrus.Warnf("Ignoring: vmType %s: networks[%d]: %+v", *l.Yaml.VMType, i, unknown)
		}
	}

	audioDevice := *l.Yaml.Audio.Device
	if audioDevice != "" {
		logrus.Warnf("Ignoring: vmType %s: `audio.device`: %+v", *l.Yaml.VMType, audioDevice)
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
		if err := EnsureFs(l.BaseDriver); err != nil {
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
		&errCh,
	); err != nil {
		return nil, err
	}

	keepAlive(ctx, distroName, &errCh)

	return errCh, err
}

// Requires WSLg, which requires specific version of WSL2 to be installed.
// TODO: Add check and add support for WSLg (instead of VNC) to hostagent.
func (l *LimaWslDriver) CanRunGUI() bool {
	// return *l.Yaml.Video.Display == "wsl"
	return false
}

func (l *LimaWslDriver) RunGUI() error {
	return fmt.Errorf("RunGUI is not supported for the given driver '%s' and display '%s'", "wsl", *l.Yaml.Video.Display)
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

func (l *LimaWslDriver) GuestAgentConn(_ context.Context) (net.Conn, error) {
	return vsock.Dial(2, uint32(l.VSockPort), nil)
}
