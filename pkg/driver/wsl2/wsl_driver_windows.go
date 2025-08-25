// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package wsl2

import (
	"context"
	"embed"
	"fmt"
	"net"
	"regexp"

	"github.com/Microsoft/go-winio"
	"github.com/Microsoft/go-winio/pkg/guid"
	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/driver"
	"github.com/lima-vm/lima/v2/pkg/freeport"
	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limayaml"
	"github.com/lima-vm/lima/v2/pkg/ptr"
	"github.com/lima-vm/lima/v2/pkg/reflectutil"
	"github.com/lima-vm/lima/v2/pkg/windows"
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
	Instance *limatype.Instance

	SSHLocalPort int
	vSockPort    int
	virtioPort   string
}

var _ driver.Driver = (*LimaWslDriver)(nil)

func New() *LimaWslDriver {
	port, err := freeport.VSock()
	if err != nil {
		logrus.WithError(err).Error("failed to get free VSock port")
	}

	return &LimaWslDriver{
		vSockPort:  port,
		virtioPort: "",
	}
}

func (l *LimaWslDriver) Configure(inst *limatype.Instance) *driver.ConfiguredDriver {
	l.Instance = inst
	l.SSHLocalPort = inst.SSHLocalPort

	return &driver.ConfiguredDriver{
		Driver: l,
	}
}

func (l *LimaWslDriver) AcceptConfig(cfg *limatype.LimaYAML, _ string) error {
	if l.Instance == nil {
		l.Instance = &limatype.Instance{}
	}
	l.Instance.Config = cfg

	if err := l.Validate(context.Background()); err != nil {
		return fmt.Errorf("config not supported by the WSL2 driver: %w", err)
	}

	return nil
}

func (l *LimaWslDriver) FillConfig(cfg *limatype.LimaYAML, _ string) error {
	if cfg.VMType == nil {
		cfg.VMType = ptr.Of(limatype.WSL2)
	}
	if cfg.MountType == nil {
		cfg.MountType = ptr.Of(limatype.WSLMount)
	}

	return nil
}

func (l *LimaWslDriver) Validate(_ context.Context) error {
	if l.Instance.Config.MountType != nil && *l.Instance.Config.MountType != limatype.WSLMount {
		return fmt.Errorf("field `mountType` must be %q for WSL2 driver, got %q", limatype.WSLMount, *l.Instance.Config.MountType)
	}
	// TODO: revise this list for WSL2
	if l.Instance.Config.VMType != nil {
		if unknown := reflectutil.UnknownNonEmptyFields(l.Instance.Config, knownYamlProperties...); len(unknown) > 0 {
			logrus.Warnf("Ignoring: vmType %s: %+v", *l.Instance.Config.VMType, unknown)
		}
	}

	if !limayaml.IsNativeArch(*l.Instance.Config.Arch) {
		return fmt.Errorf("unsupported arch: %q", *l.Instance.Config.Arch)
	}

	if l.Instance.Config.VMType != nil {
		if l.Instance.Config.Images != nil && l.Instance.Config.Arch != nil {
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
		}

		if l.Instance.Config.Mounts != nil {
			for i, mount := range l.Instance.Config.Mounts {
				if unknown := reflectutil.UnknownNonEmptyFields(mount); len(unknown) > 0 {
					logrus.Warnf("Ignoring: vmType %s: mounts[%d]: %+v", *l.Instance.Config.VMType, i, unknown)
				}
			}
		}

		if l.Instance.Config.Networks != nil {
			for i, network := range l.Instance.Config.Networks {
				if unknown := reflectutil.UnknownNonEmptyFields(network); len(unknown) > 0 {
					logrus.Warnf("Ignoring: vmType %s: networks[%d]: %+v", *l.Instance.Config.VMType, i, unknown)
				}
			}
		}

		if l.Instance.Config.Audio.Device != nil {
			audioDevice := *l.Instance.Config.Audio.Device
			if audioDevice != "" {
				logrus.Warnf("Ignoring: vmType %s: `audio.device`: %+v", *l.Instance.Config.VMType, audioDevice)
			}
		}
	}

	return nil
}

//go:embed boot/*.sh
var bootFS embed.FS

func (l *LimaWslDriver) BootScripts() (map[string][]byte, error) {
	scripts := make(map[string][]byte)

	entries, err := bootFS.ReadDir("boot")
	if err != nil {
		return scripts, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		content, err := bootFS.ReadFile("boot/" + entry.Name())
		if err != nil {
			return nil, err
		}

		scripts[entry.Name()] = content
	}

	return scripts, nil
}

func (l *LimaWslDriver) InspectStatus(ctx context.Context, inst *limatype.Instance) string {
	status, err := getWslStatus(inst.Name)
	if err != nil {
		inst.Status = limatype.StatusBroken
		inst.Errors = append(inst.Errors, err)
	} else {
		inst.Status = status
	}

	inst.SSHLocalPort = 22

	if inst.Status == limatype.StatusRunning {
		sshAddr, err := l.SSHAddress(ctx)
		if err == nil {
			inst.SSHAddress = sshAddr
		} else {
			inst.Errors = append(inst.Errors, err)
		}
	}

	return inst.Status
}

func (l *LimaWslDriver) Delete(ctx context.Context) error {
	distroName := "lima-" + l.Instance.Name
	status, err := getWslStatus(l.Instance.Name)
	if err != nil {
		return err
	}
	switch status {
	case limatype.StatusRunning, limatype.StatusStopped, limatype.StatusBroken, limatype.StatusInstalling:
		return unregisterVM(ctx, distroName)
	}

	logrus.Info("WSL VM is not running or does not exist, skipping deletion")
	return nil
}

func (l *LimaWslDriver) Start(ctx context.Context) (chan error, error) {
	logrus.Infof("Starting WSL VM")
	status, err := getWslStatus(l.Instance.Name)
	if err != nil {
		return nil, err
	}

	distroName := "lima-" + l.Instance.Name

	if status == limatype.StatusUninitialized {
		if err := EnsureFs(ctx, l.Instance); err != nil {
			return nil, err
		}
		if err := initVM(ctx, l.Instance.Dir, distroName); err != nil {
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

	keepAlive(ctx, distroName, errCh)

	return errCh, err
}

// CanRunGUI requires WSLg, which requires specific version of WSL2 to be installed.
// TODO: Add check and add support for WSLg (instead of VNC) to hostagent.
func (l *LimaWslDriver) canRunGUI() bool {
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

// GuestAgentConn returns the guest agent connection, or nil (if forwarded by ssh).
// As of 08-01-2024, github.com/mdlayher/vsock does not natively support vsock on
// Windows, so use the winio library to create the connection.
func (l *LimaWslDriver) GuestAgentConn(ctx context.Context) (net.Conn, string, error) {
	VMIDStr, err := windows.GetInstanceVMID(ctx, fmt.Sprintf("lima-%s", l.Instance.Name))
	if err != nil {
		return nil, "", err
	}
	VMIDGUID, err := guid.FromString(VMIDStr)
	if err != nil {
		return nil, "", err
	}
	sockAddr := &winio.HvsockAddr{
		VMID:      VMIDGUID,
		ServiceID: winio.VsockServiceID(uint32(l.vSockPort)),
	}
	conn, err := winio.Dial(ctx, sockAddr)
	if err != nil {
		return nil, "", err
	}

	return conn, "vsock", nil
}

func (l *LimaWslDriver) Info() driver.Info {
	var info driver.Info
	if l.Instance != nil {
		info.InstanceDir = l.Instance.Dir
	}
	info.VirtioPort = l.virtioPort
	info.VsockPort = l.vSockPort

	info.Features = driver.DriverFeatures{
		DynamicSSHAddress:    true,
		SkipSocketForwarding: true,
		DriverName:           "wsl2",
		CanRunGUI:            l.canRunGUI(),
	}
	return info
}

func (l *LimaWslDriver) SSHAddress(_ context.Context) (string, error) {
	return "127.0.0.1", nil
}

func (l *LimaWslDriver) Create(_ context.Context) error {
	return nil
}

func (l *LimaWslDriver) CreateDisk(_ context.Context) error {
	return nil
}

func (l *LimaWslDriver) ChangeDisplayPassword(_ context.Context, _ string) error {
	return nil
}

func (l *LimaWslDriver) DisplayConnection(_ context.Context) (string, error) {
	return "", nil
}

func (l *LimaWslDriver) CreateSnapshot(_ context.Context, _ string) error {
	return errUnimplemented
}

func (l *LimaWslDriver) ApplySnapshot(_ context.Context, _ string) error {
	return errUnimplemented
}

func (l *LimaWslDriver) DeleteSnapshot(_ context.Context, _ string) error {
	return errUnimplemented
}

func (l *LimaWslDriver) ListSnapshots(_ context.Context) (string, error) {
	return "", errUnimplemented
}

func (l *LimaWslDriver) ForwardGuestAgent() bool {
	// If driver is not providing, use host agent
	return l.vSockPort == 0 && l.virtioPort == ""
}
