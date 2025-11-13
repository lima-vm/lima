// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package krunkit

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/driver"
	"github.com/lima-vm/lima/v2/pkg/executil"
	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
	"github.com/lima-vm/lima/v2/pkg/limayaml"
	"github.com/lima-vm/lima/v2/pkg/networks/usernet"
	"github.com/lima-vm/lima/v2/pkg/osutil"
	"github.com/lima-vm/lima/v2/pkg/ptr"
)

type LimaKrunkitDriver struct {
	Instance     *limatype.Instance
	SSHLocalPort int

	usernetClient *usernet.Client
	stopUsernet   context.CancelFunc
	krunkitCmd    *exec.Cmd
	krunkitWaitCh chan error
}

var (
	_      driver.Driver   = (*LimaKrunkitDriver)(nil)
	vmType limatype.VMType = "krunkit"
)

func New() *LimaKrunkitDriver {
	return &LimaKrunkitDriver{}
}

func (l *LimaKrunkitDriver) Configure(inst *limatype.Instance) *driver.ConfiguredDriver {
	l.Instance = inst
	l.SSHLocalPort = inst.SSHLocalPort

	return &driver.ConfiguredDriver{
		Driver: l,
	}
}

func (l *LimaKrunkitDriver) CreateDisk(ctx context.Context) error {
	return EnsureDisk(ctx, l.Instance)
}

func (l *LimaKrunkitDriver) Start(ctx context.Context) (chan error, error) {
	var err error
	l.usernetClient, l.stopUsernet, err = startUsernet(ctx, l.Instance)
	if err != nil {
		return nil, fmt.Errorf("failed to start usernet: %w", err)
	}

	krunkitCmd, err := Cmdline(l.Instance)
	if err != nil {
		return nil, fmt.Errorf("failed to construct krunkit command line: %w", err)
	}
	// Detach krunkit process from parent Lima process
	krunkitCmd.SysProcAttr = executil.BackgroundSysProcAttr

	logPath := filepath.Join(l.Instance.Dir, "krunkit.log")
	logfile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("failed to open krunkit logfile: %w", err)
	}
	krunkitCmd.Stderr = logfile

	logrus.Infof("Starting krun VM (hint: to watch the progress, see %q)", logPath)
	logrus.Infof("krunkitCmd.Args: %v", krunkitCmd.Args)

	if err := krunkitCmd.Start(); err != nil {
		logfile.Close()
		return nil, errors.New("failed to start krunkitCmd")
	}

	pidPath := filepath.Join(l.Instance.Dir, filenames.PIDFile(*l.Instance.Config.VMType))
	if err := os.WriteFile(pidPath, fmt.Appendf(nil, "%d\n", krunkitCmd.Process.Pid), 0o644); err != nil {
		logrus.WithError(err).Warn("Failed to write PID file")
	}

	l.krunkitCmd = krunkitCmd
	l.krunkitWaitCh = make(chan error, 1)
	go func() {
		defer func() {
			logfile.Close()
			os.RemoveAll(pidPath)
			close(l.krunkitWaitCh)
		}()
		l.krunkitWaitCh <- krunkitCmd.Wait()
	}()

	err = l.usernetClient.ConfigureDriver(ctx, l.Instance, l.SSHLocalPort)
	if err != nil {
		l.krunkitWaitCh <- fmt.Errorf("failed to configure usernet: %w", err)
	}

	return l.krunkitWaitCh, nil
}

func (l *LimaKrunkitDriver) Stop(_ context.Context) error {
	if l.krunkitCmd == nil {
		return nil
	}

	if err := l.krunkitCmd.Process.Signal(syscall.SIGTERM); err != nil {
		logrus.WithError(err).Warn("Failed to send interrupt signal")
	}

	go func() {
		if l.usernetClient != nil {
			_ = l.usernetClient.UnExposeSSH(l.Instance.SSHLocalPort)
		}
		if l.stopUsernet != nil {
			l.stopUsernet()
		}
	}()

	timeout := time.After(30 * time.Second)
	select {
	case <-l.krunkitWaitCh:
		return nil
	case <-timeout:
		if err := l.krunkitCmd.Process.Kill(); err != nil {
			return err
		}

		<-l.krunkitWaitCh
		return nil
	}
}

func (l *LimaKrunkitDriver) Validate(_ context.Context) error {
	return validateConfig(l.Instance.Config)
}

func validateConfig(cfg *limatype.LimaYAML) error {
	if cfg == nil {
		return errors.New("configuration is nil")
	}
	macOSProductVersion, err := osutil.ProductVersion()
	if err != nil {
		return err
	}
	if macOSProductVersion.LessThan(*semver.New("13.0.0")) {
		return errors.New("krunkit driver requires macOS 13 or higher to run")
	}
	if cfg.Arch != nil && !limayaml.IsNativeArch(*cfg.Arch) {
		return fmt.Errorf("unsupported arch: %q (krunkit requires native arch)", *cfg.Arch)
	}
	if _, err := exec.LookPath(vmType); err != nil {
		return errors.New("krunkit CLI not found in PATH. Install it via:\nbrew tap slp/krunkit\nbrew install krunkit")
	}

	if cfg.MountType != nil && (*cfg.MountType != limatype.VIRTIOFS && *cfg.MountType != limatype.REVSSHFS) {
		return fmt.Errorf("field `mountType` must be %q or %q for krunkit driver, got %q", limatype.VIRTIOFS, limatype.REVSSHFS, *cfg.MountType)
	}

	return nil
}

func isFedoraConfigured(cfg *limatype.LimaYAML) bool {
	for _, b := range cfg.Base {
		if strings.Contains(strings.ToLower(b.URL), "fedora") {
			return true
		}
	}
	for _, img := range cfg.Images {
		if strings.Contains(strings.ToLower(img.Location), "fedora") {
			return true
		}
	}
	return false
}

func (l *LimaKrunkitDriver) FillConfig(_ context.Context, cfg *limatype.LimaYAML, _ string) error {
	if cfg.MountType == nil {
		cfg.MountType = ptr.Of(limatype.VIRTIOFS)
	} else {
		*cfg.MountType = limatype.VIRTIOFS
	}

	if cfg.Arch == nil {
		cfg.Arch = ptr.Of(limatype.AARCH64)
	} else {
		*cfg.Arch = limatype.AARCH64
	}

	cfg.VMType = ptr.Of(vmType)

	return validateConfig(cfg)
}

//go:embed boot/*.sh
var bootFS embed.FS

func (l *LimaKrunkitDriver) BootScripts() (map[string][]byte, error) {
	scripts := make(map[string][]byte)

	entries, err := bootFS.ReadDir("boot")
	if err == nil && !isFedoraConfigured(l.Instance.Config) {
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
	}

	// Disabled by krunkit driver for Fedora to make boot time faster
	if isFedoraConfigured(l.Instance.Config) {
		scripts["00-reboot-if-required.sh"] = []byte(`#!/bin/sh
set -eu
exit 0
`)
	}

	return scripts, nil
}

func (l *LimaKrunkitDriver) Create(_ context.Context) error {
	return nil
}

func (l *LimaKrunkitDriver) Info() driver.Info {
	var info driver.Info
	info.Name = vmType
	if l.Instance != nil && l.Instance.Dir != "" {
		info.InstanceDir = l.Instance.Dir
	}

	info.Features = driver.DriverFeatures{
		DynamicSSHAddress:    false,
		SkipSocketForwarding: false,
		CanRunGUI:            false,
	}
	return info
}

func (l *LimaKrunkitDriver) SSHAddress(_ context.Context) (string, error) {
	return "127.0.0.1", nil
}

func (l *LimaKrunkitDriver) ForwardGuestAgent() bool {
	return true
}

func (l *LimaKrunkitDriver) Delete(_ context.Context) error {
	return nil
}

func (l *LimaKrunkitDriver) InspectStatus(_ context.Context, _ *limatype.Instance) string {
	return ""
}

func (l *LimaKrunkitDriver) RunGUI() error {
	return nil
}

func (l *LimaKrunkitDriver) ChangeDisplayPassword(_ context.Context, _ string) error {
	return errUnimplemented
}

func (l *LimaKrunkitDriver) DisplayConnection(_ context.Context) (string, error) {
	return "", errUnimplemented
}

func (l *LimaKrunkitDriver) CreateSnapshot(_ context.Context, _ string) error {
	return errUnimplemented
}

func (l *LimaKrunkitDriver) ApplySnapshot(_ context.Context, _ string) error {
	return errUnimplemented
}

func (l *LimaKrunkitDriver) DeleteSnapshot(_ context.Context, _ string) error {
	return errUnimplemented
}

func (l *LimaKrunkitDriver) ListSnapshots(_ context.Context) (string, error) {
	return "", errUnimplemented
}

func (l *LimaKrunkitDriver) Register(_ context.Context) error {
	return nil
}

func (l *LimaKrunkitDriver) Unregister(_ context.Context) error {
	return nil
}

func (l *LimaKrunkitDriver) GuestAgentConn(_ context.Context) (net.Conn, string, error) {
	return nil, "unix", nil
}

func (l *LimaKrunkitDriver) AdditionalSetupForSSH(_ context.Context) error {
	return nil
}
