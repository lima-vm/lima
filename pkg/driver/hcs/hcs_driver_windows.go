// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package hcs

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/Microsoft/go-winio"
	"github.com/Microsoft/hcsshim/hcn"
	"github.com/docker/go-units"
	"github.com/lima-vm/go-qcow2reader/image/vhdx"
	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/driver"
	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
	"github.com/lima-vm/lima/v2/pkg/reflectutil"
)

var knownYamlProperties = []string{
	"AdditionalDisks",
	"Arch",
	"Audio",
	"CACertificates",
	"Containerd",
	"CopyToHost",
	"CPUs",
	"CPUType",
	"Disk",
	"DNS",
	"Env",
	"Firmware",
	"GuestInstallPrefix",
	"HostResolver",
	"Images",
	"Memory",
	"Message",
	"MinimumLimaVersion",
	"Mounts",
	"MountType",
	"MountTypesUnsupported",
	"MountInotify",
	"NestedVirtualization",
	"Networks",
	"OS",
	"Param",
	"Plain",
	"PortForwards",
	"Probes",
	"PropagateProxyEnv",
	"Provision",
	"SSH",
	"TimeZone",
	"TPM",
	"UpgradePackages",
	"User",
	"Video",
	"VMType",
	"VMOpts",
}

const Enabled = true

type LimaHcsDriver struct {
	Instance *limatype.Instance

	SSHLocalPort int
	vSockPort    int
	virtioPort   string

	system         hcsSystem
	serialListener net.Listener
	sshAddress     string
}

var _ driver.Driver = (*LimaHcsDriver)(nil)

func New() *LimaHcsDriver {
	return &LimaHcsDriver{}
}

func (l *LimaHcsDriver) Configure(ctx context.Context, inst *limatype.Instance) (*driver.ConfiguredDriver, error) {
	cfg := inst.Config
	if cfg.VMType == nil {
		cfg.VMType = new(limatype.HCS)
	}
	if cfg.MountType == nil {
		cfg.MountType = new(limatype.REVSSHFS)
	}
	if err := validateConfig(ctx, cfg); err != nil {
		return nil, err
	}

	l.Instance = inst
	l.SSHLocalPort = inst.SSHLocalPort

	return &driver.ConfiguredDriver{
		Driver: l,
	}, nil
}

func (l *LimaHcsDriver) Validate(ctx context.Context) error {
	return validateConfig(ctx, l.Instance.Config)
}

func validateConfig(_ context.Context, cfg *limatype.LimaYAML) error {
	if cfg == nil {
		return errors.New("configuration is nil")
	}
	// TODO: revise this list for HCS
	if cfg.VMType != nil {
		if unknown := reflectutil.UnknownNonEmptyFields(cfg, knownYamlProperties...); len(unknown) > 0 {
			logrus.Warnf("Ignoring: vmType %s: %+v", *cfg.VMType, unknown)
		}
	}

	if cfg.OS != nil && *cfg.OS == limatype.WINDOWS {
		return errors.New("currently Windows guest OS is only supported on QEMU")
	}

	if !limatype.IsNativeArch(*cfg.Arch) {
		return fmt.Errorf("unsupported arch: %#q", *cfg.Arch)
	}

	if cfg.TPM != nil && *cfg.TPM {
		return errors.New("field `tpm` is not supported on HCS driver")
	}

	if cfg.VMType != nil {
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

		if cfg.HostResolver.Enabled == nil {
			cfg.HostResolver.Enabled = new(false)
		}
		if *cfg.HostResolver.Enabled {
			return errors.New("the HCS driver does not support HostResolver.Enabled")
		}
	}

	return nil
}

func (l *LimaHcsDriver) BootScripts(_ context.Context) (map[string][]byte, error) {
	return nil, nil
}

func (l *LimaHcsDriver) InspectStatus(ctx context.Context, inst *limatype.Instance) string {
	inst.SSHLocalPort = 22
	state, err := getComputeSystemState(ctx, "lima-"+inst.Name)
	if err != nil {
		if errors.Is(err, errNotFound) {
			inst.Status = limatype.StatusUnknown
			return inst.Status
		}
		inst.Status = limatype.StatusBroken
		inst.Errors = append(inst.Errors, fmt.Errorf("failed to inspect HCS sompute system: %w", err))
		return inst.Status
	}
	if state == "Running" {
		inst.Status = limatype.StatusRunning
		e, err := hcn.GetEndpointByName(endpointName(inst.Name))
		if err != nil {
			logrus.Errorf("failed to get HCN endpoint: %s", err.Error())
			inst.Status = limatype.StatusBroken
			return inst.Status
		}
		ips := e.IpConfigurations
		if len(ips) == 0 {
			logrus.Errorf("the HCN endpoint for instance %#q does not have an IP address yet", inst.Name)
			inst.Status = limatype.StatusBroken
			return inst.Status
		}
		ip := ips[0]
		l.sshAddress = ip.IpAddress
		inst.SSHAddress = ip.IpAddress
	} else {
		inst.Status = limatype.StatusStopped
	}
	return inst.Status
}

func (l *LimaHcsDriver) Delete(_ context.Context) error {
	if endpoint, err := hcn.GetEndpointByName(endpointName(l.Instance.Name)); err == nil {
		if err := endpoint.Delete(); err != nil {
			return fmt.Errorf("failed to delete the HCN endpoint: %w", err)
		}
	}
	if network, err := hcn.GetNetworkByName(networkName(l.Instance.Name)); err == nil {
		if err := network.Delete(); err != nil {
			return fmt.Errorf("failed to delete the HCN network: %w", err)
		}
	}
	return nil
}

func (l *LimaHcsDriver) Start(ctx context.Context) (chan error, error) {
	inst := l.Instance
	diskPath := filepath.Join(inst.Dir, filenames.Disk) + ".vhdx"
	isoPath := filepath.Join(inst.Dir, filenames.CIDataISO)

	network, err := ensureNetwork(networkName(inst.Name))
	if err != nil {
		return nil, err
	}
	endpoint, err := ensureEndpoint(network, endpointName(inst.Name), inst.Dir)
	if err != nil {
		return nil, err
	}

	pipeName := `\\.\pipe\lima-` + inst.Name + `-com1`
	ln, err := winio.ListenPipe(pipeName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on %#q: %w", pipeName, err)
	}
	l.serialListener = ln
	go serveSerial(ln, filepath.Join(inst.Dir, filenames.SerialLog))

	vmID := "lima-" + inst.Name
	for _, p := range []string{diskPath, isoPath} {
		if err := hcsGrantVMAccess(vmID, p); err != nil {
			return nil, fmt.Errorf("failed to grant VM access to %#q: %w", p, err)
		}
	}

	memBytes, err := units.RAMInBytes(*inst.Config.Memory)
	if err != nil {
		return nil, err
	}
	config, err := buildVMConfig(diskPath, isoPath, endpoint.Id, endpoint.MacAddress, pipeName, *inst.Config.CPUs, memBytes>>20)
	if err != nil {
		return nil, err
	}
	logrus.Debugf("HCS compute system config: %s", config)

	system, err := createComputeSystem(ctx, "lima-"+inst.Name, config)
	if err != nil {
		return nil, err
	}
	l.system = system
	if err := startComputeSystem(ctx, system); err != nil {
		return nil, err
	}

	for i := range 50 {
		endpoint, err = hcn.GetEndpointByName(endpointName(inst.Name))
		if err != nil {
			logrus.Warnf("failed in GetEndpointByName: %s", err.Error())
			time.Sleep(2 * time.Second)
			continue
		}
		if len(endpoint.IpConfigurations) == 0 {
			if i == 49 {
				return nil, fmt.Errorf("HNS did not assign an IP address to the endpoint %#q", endpoint.Name)
			}
			time.Sleep(2 * time.Second)
			continue
		}
		l.sshAddress = endpoint.IpConfigurations[0].IpAddress
		logrus.Infof("HCS: the guest is expected to get %s (MAC %s)", l.sshAddress, endpoint.MacAddress)
		break
	}

	errCh := make(chan error, 1)
	go func() {
		result, waitErr := hcsWaitForComputeSystemExit(ctx, system, hcsTimeoutInfinite)
		if waitErr != nil {
			errCh <- fmt.Errorf("HCS compute system exited: %w (result=%s)", waitErr, result)
			return
		}
		errCh <- nil
	}()
	return errCh, nil
}

func (l *LimaHcsDriver) RunGUI(_ context.Context) error {
	return fmt.Errorf("RunGUI is not supported for the given driver '%s' and display '%s'", "hcs", *l.Instance.Config.Video.Display)
}

func (l *LimaHcsDriver) Stop(ctx context.Context) error {
	if l.system == 0 {
		return nil
	}
	if err := terminateComputeSystem(ctx, l.system); err != nil {
		return err
	}
	if l.serialListener != nil {
		_ = l.serialListener.Close()
	}

	if endpoint, err := hcn.GetEndpointByName(endpointName(l.Instance.Name)); err == nil {
		if err = endpoint.Delete(); err != nil {
			return fmt.Errorf("failed to delete the HCN endpoint: %w", err)
		}
	}

	if network, err := hcn.GetNetworkByName(networkName(l.Instance.Name)); err == nil {
		if err := network.Delete(); err != nil {
			return fmt.Errorf("failed to delete the HCN network %w", err)
		}
	}

	return nil
}

// GuestAgentConn returns the guest agent connection, or nil (if forwarded by ssh).
// As of 08-01-2024, github.com/mdlayher/vsock does not natively support vsock on
// Windows, so use the winio library to create the connection.
func (l *LimaHcsDriver) GuestAgentConn(_ context.Context) (net.Conn, string, error) {
	return nil, "", nil
}

func (l *LimaHcsDriver) Info(_ context.Context) driver.Info {
	var info driver.Info
	info.Name = "hcs"
	if l.Instance != nil {
		info.InstanceDir = l.Instance.Dir
	}
	info.VirtioPort = l.virtioPort
	info.VsockPort = l.vSockPort

	info.Features = driver.DriverFeatures{
		DynamicSSHAddress:     true,
		StaticSSHPort:         true,
		SkipSocketForwarding:  true,
		NoCloudInit:           false,
		CanRunGUI:             false,
		SupportedImageFormats: []string{string(vhdx.Type)},
	}
	return info
}

func (l *LimaHcsDriver) SSHAddress(_ context.Context) (string, error) {
	if l.sshAddress == "" {
		return "", errors.New("the SSH address is not known yet")
	}
	return l.sshAddress, nil
}

func (l *LimaHcsDriver) Create(_ context.Context) error {
	return nil
}

func (l *LimaHcsDriver) CreateDisk(ctx context.Context) error {
	if err := mayConvertQcow2ToVHDX(ctx, l.Instance.Dir); err != nil {
		return err
	}
	disk := filepath.Join(l.Instance.Dir, filenames.Disk)
	link := disk + ".vhdx"
	if _, err := os.Stat(link); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.Link(disk, link)
}

func (l *LimaHcsDriver) ChangeDisplayPassword(_ context.Context, _ string) error {
	return nil
}

func (l *LimaHcsDriver) DisplayConnection(_ context.Context) (string, error) {
	return "", nil
}

func (l *LimaHcsDriver) CreateSnapshot(_ context.Context, _ string) error {
	return errUnimplemented
}

func (l *LimaHcsDriver) ApplySnapshot(_ context.Context, _ string) error {
	return errUnimplemented
}

func (l *LimaHcsDriver) DeleteSnapshot(_ context.Context, _ string) error {
	return errUnimplemented
}

func (l *LimaHcsDriver) ListSnapshots(_ context.Context) (string, error) {
	return "", errUnimplemented
}

func (l *LimaHcsDriver) ForwardGuestAgent(_ context.Context) bool {
	// If driver is not providing, use host agent
	return l.vSockPort == 0 && l.virtioPort == ""
}

func (l *LimaHcsDriver) AdditionalSetupForSSH(_ context.Context) error {
	return nil
}
