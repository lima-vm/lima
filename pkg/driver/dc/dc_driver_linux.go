// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package dc

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"net"
	"regexp"

	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/driver"
	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limayaml"
	"github.com/lima-vm/lima/v2/pkg/ptr"
	"github.com/lima-vm/lima/v2/pkg/reflectutil"
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
	Instance *limatype.Instance

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

func (l *LimaDcDriver) Configure(inst *limatype.Instance) *driver.ConfiguredDriver {
	l.Instance = inst
	l.SSHLocalPort = inst.SSHLocalPort

	return &driver.ConfiguredDriver{
		Driver: l,
	}
}

func (l *LimaDcDriver) FillConfig(ctx context.Context, cfg *limatype.LimaYAML, _ string) error {
	if cfg.VMType == nil {
		cfg.VMType = ptr.Of(limatype.DC)
	}
	if cfg.MountType == nil {
		cfg.MountType = ptr.Of(limatype.WSLMount)
	}
	return validateConfig(ctx, cfg)
}

func (l *LimaDcDriver) Validate(ctx context.Context) error {
	return validateConfig(ctx, l.Instance.Config)
}

func validateConfig(_ context.Context, cfg *limatype.LimaYAML) error {
	if cfg == nil {
		return errors.New("configuration is nil")
	}
	if *cfg.MountType != limatype.REVSSHFS {
		return fmt.Errorf("field `mountType` must be %q for DC driver, got %q", limatype.REVSSHFS, *cfg.MountType)
	}
	// TODO: revise this list for DC
	if cfg.VMType != nil {
		if unknown := reflectutil.UnknownNonEmptyFields(cfg, knownYamlProperties...); len(unknown) > 0 {
			logrus.Warnf("Ignoring: vmType %s: %+v", *cfg.VMType, unknown)
		}
	}

	if !limayaml.IsNativeArch(*cfg.Arch) {
		return fmt.Errorf("unsupported arch: %q", *cfg.Arch)
	}

	if cfg.VMType != nil {
		if cfg.Images != nil && cfg.Arch != nil {
			// TODO: real filetype checks
			tarFileRegex := regexp.MustCompile(`.*tar\.*`)
			for i, image := range cfg.Images {
				if unknown := reflectutil.UnknownNonEmptyFields(image, "File"); len(unknown) > 0 {
					logrus.Warnf("Ignoring: vmType %s: images[%d]: %+v", *cfg.VMType, i, unknown)
				}
				match := tarFileRegex.MatchString(image.Location)
				if image.Arch == *cfg.Arch && !match {
					return fmt.Errorf("unsupported image type for vmType: %s, tarball root file system required: %q", *cfg.VMType, image.Location)
				}
			}
		}

		if cfg.Mounts != nil {
			for i, mount := range cfg.Mounts {
				if unknown := reflectutil.UnknownNonEmptyFields(mount); len(unknown) > 0 {
					logrus.Warnf("Ignoring: vmType %s: mounts[%d]: %+v", *cfg.VMType, i, unknown)
				}
			}
		}

		if cfg.Networks != nil {
			for i, network := range cfg.Networks {
				if unknown := reflectutil.UnknownNonEmptyFields(network); len(unknown) > 0 {
					logrus.Warnf("Ignoring: vmType %s: networks[%d]: %+v", *cfg.VMType, i, unknown)
				}
			}
		}

		if cfg.Audio.Device != nil {
			audioDevice := *cfg.Audio.Device
			if audioDevice != "" {
				logrus.Warnf("Ignoring: vmType %s: `audio.device`: %+v", *cfg.VMType, audioDevice)
			}
		}
	}

	return nil
}

//go:embed boot/*.sh
var bootFS embed.FS

func (l *LimaDcDriver) BootScripts() (map[string][]byte, error) {
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

func (l *LimaDcDriver) InspectStatus(ctx context.Context, inst *limatype.Instance) string {
	status, err := getDcStatus(ctx, inst.Name)
	if err != nil {
		inst.Status = limatype.StatusBroken
		inst.Errors = append(inst.Errors, err)
	} else {
		inst.Status = status
	}

	inst.SSHLocalPort = 22

	if inst.Status == limatype.StatusRunning {
		sshAddr, err := getSSHAddress(ctx, inst.Name)
		if err == nil {
			inst.SSHAddress = sshAddr
		} else {
			inst.Errors = append(inst.Errors, err)
		}
	}

	return inst.Status
}

func (l *LimaDcDriver) Delete(ctx context.Context) error {
	distroName := "lima-" + l.Instance.Name
	status, err := getDcStatus(ctx, l.Instance.Name)
	if err != nil {
		return err
	}
	switch status {
	case limatype.StatusRunning, limatype.StatusStopped, limatype.StatusBroken, limatype.StatusInstalling:
		return deleteVM(ctx, distroName)
	}

	logrus.Info("AC VM is not running or does not exist, skipping deletion")
	return nil
}

func (l *LimaDcDriver) Start(ctx context.Context) (chan error, error) {
	logrus.Infof("Starting DC VM")
	status, err := getDcStatus(ctx, l.Instance.Name)
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
		cpus := l.Instance.CPUs
		memory := int(l.Instance.Memory >> 20) // MiB
		if err := createVM(ctx, distroName, cpus, memory); err != nil {
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

// GuestAgentConn returns the guest agent connection, or nil (if forwarded by ssh).
func (l *LimaDcDriver) GuestAgentConn(_ context.Context) (net.Conn, string, error) {
	return nil, "", nil
}

func (l *LimaDcDriver) Info() driver.Info {
	var info driver.Info
	info.Name = "dc"
	if l.Instance != nil {
		info.InstanceDir = l.Instance.Dir
	}
	info.VirtioPort = l.virtioPort
	info.VsockPort = l.vSockPort

	info.Features = driver.DriverFeatures{
		DynamicSSHAddress:    true,
		StaticSSHPort:        true,
		SkipSocketForwarding: true,
		NoCloudInit:          true,
		CanRunGUI:            l.canRunGUI(),
	}
	return info
}

func (l *LimaDcDriver) SSHAddress(_ context.Context) (string, error) {
	return "127.0.0.1", nil
}

func (l *LimaDcDriver) Create(_ context.Context) error {
	return nil
}

func (l *LimaDcDriver) CreateDisk(_ context.Context) error {
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

func (l *LimaDcDriver) AdditionalSetupForSSH(_ context.Context) error {
	return nil
}
